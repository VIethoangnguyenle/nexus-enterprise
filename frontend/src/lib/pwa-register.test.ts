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
