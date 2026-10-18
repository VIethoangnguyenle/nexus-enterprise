import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { usePwaStore } from './pwa'

vi.mock('./push', () => ({ getOneSignalAppId: () => 'app-id' }))

describe('registerPwa with OneSignal owning the worker', () => {
  let reload: ReturnType<typeof vi.fn>
  let swListeners: Record<string, () => void>
  let waiting: { postMessage: ReturnType<typeof vi.fn> }

  beforeEach(() => {
    vi.stubEnv('PROD', true)
    reload = vi.fn()
    vi.stubGlobal('location', { ...window.location, reload })
    swListeners = {}
    waiting = { postMessage: vi.fn() }
    const reg = { waiting, addEventListener: vi.fn() }
    Object.defineProperty(navigator, 'serviceWorker', {
      configurable: true,
      value: {
        ready: Promise.resolve(reg),
        addEventListener: (type: string, fn: () => void) => { swListeners[type] = fn },
      },
    })
  })

  afterEach(() => {
    vi.unstubAllEnvs()
    vi.unstubAllGlobals()
    Reflect.deleteProperty(navigator, 'serviceWorker')
  })

  it('reloads only once the new worker controls the page', async () => {
    const { registerPwa } = await import('./pwa-register')
    await registerPwa()
    expect(usePwaStore.getState().needRefresh).toBe(true)

    usePwaStore.getState().applyUpdate()
    expect(waiting.postMessage).toHaveBeenCalledWith({ type: 'SKIP_WAITING' })
    // Reloading now would be answered by the old worker's app shell, i.e. the old version.
    expect(reload).not.toHaveBeenCalled()

    swListeners.controllerchange?.()
    expect(reload).toHaveBeenCalledTimes(1)
  })
})

describe('registerPwa asks for updates', () => {
  let docListeners: Record<string, () => void>
  let update: ReturnType<typeof vi.fn>
  let reg: { waiting: unknown; update: ReturnType<typeof vi.fn>; addEventListener: ReturnType<typeof vi.fn> }
  let visibility: string
  let onRegisteredSW: ((url: string, r: unknown) => void) | undefined

  const setup = async (oneSignal: boolean) => {
    vi.resetModules()
    vi.doMock('./push', () => ({ getOneSignalAppId: () => (oneSignal ? 'app-id' : undefined) }))
    vi.doMock('virtual:pwa-register', () => ({
      registerSW: (opts: { onRegisteredSW: (url: string, r: unknown) => void }) => {
        onRegisteredSW = opts.onRegisteredSW
        return vi.fn()
      },
    }))
    const { usePwaStore: store } = await import('./pwa')
    store.setState({ needRefresh: false })
    const { registerPwa } = await import('./pwa-register')
    await registerPwa()
    if (!oneSignal) onRegisteredSW?.('/sw.js', reg)
    return store
  }
  const resume = () => docListeners.visibilitychange?.()

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-10-18T00:00:00Z'))
    vi.stubEnv('PROD', true)
    update = vi.fn().mockResolvedValue(undefined)
    reg = { waiting: null, update, addEventListener: vi.fn() }
    visibility = 'visible'
    docListeners = {}
    vi.spyOn(document, 'addEventListener').mockImplementation(((type: string, fn: () => void) => {
      docListeners[type] = fn
    }) as typeof document.addEventListener)
    Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => visibility })
    Object.defineProperty(navigator, 'serviceWorker', {
      configurable: true,
      value: { ready: Promise.resolve(reg), addEventListener: vi.fn(), getRegistration: () => Promise.resolve(reg) },
    })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    vi.unstubAllEnvs()
    Reflect.deleteProperty(navigator, 'serviceWorker')
    Reflect.deleteProperty(document, 'visibilityState')
  })

  it.each([true, false])('checks on resume, throttled (oneSignal=%s)', async (oneSignal) => {
    await setup(oneSignal)
    resume()
    expect(update).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(30_000)
    resume()
    expect(update).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(31_000)
    resume()
    expect(update).toHaveBeenCalledTimes(2)
  })

  it('does not check while hidden', async () => {
    await setup(true)
    visibility = 'hidden'
    resume()
    expect(update).not.toHaveBeenCalled()
  })

  it('checks on pageshow from the back/forward cache and on focus', async () => {
    const winAdd = vi.spyOn(window, 'addEventListener')
    await setup(true)
    const handler = (type: string) => winAdd.mock.calls.find((c) => c[0] === type)![1] as (e: unknown) => void
    handler('pageshow')({ persisted: false })
    expect(update).not.toHaveBeenCalled()
    handler('pageshow')({ persisted: true })
    expect(update).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(61_000)
    handler('focus')({})
    expect(update).toHaveBeenCalledTimes(2)
  })

  it('checks on an interval while visible', async () => {
    await setup(true)
    vi.advanceTimersByTime(30 * 60_000)
    expect(update).toHaveBeenCalledTimes(1)
    vi.advanceTimersByTime(30 * 60_000)
    expect(update).toHaveBeenCalledTimes(2)
  })

  it('shows the update bar when a worker is already waiting on resume', async () => {
    const store = await setup(false)
    expect(store.getState().needRefresh).toBe(false)
    reg.waiting = {}
    vi.advanceTimersByTime(61_000)
    resume()
    expect(store.getState().needRefresh).toBe(true)
  })

  it('swallows an update() rejection', async () => {
    update.mockRejectedValue(new TypeError('offline'))
    await setup(true)
    resume()
    await vi.advanceTimersByTimeAsync(0)
    expect(update).toHaveBeenCalledTimes(1)
  })
})
