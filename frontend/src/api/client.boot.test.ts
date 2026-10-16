import { describe, it, expect, vi, beforeEach } from 'vitest'

const mockFetch = vi.fn()
vi.stubGlobal('fetch', mockFetch)

// A small store with subscribe, so a request made during the boot refresh can wait for it.
type State = { accessToken: string | null; bootstrapping: boolean }
const listeners = new Set<(s: State) => void>()
const state: State & { setAccessToken: (t: string | null) => void; setBootstrapped: () => void } = {
  accessToken: null,
  bootstrapping: true,
  setAccessToken: (t) => { state.accessToken = t; listeners.forEach((l) => l(state)) },
  setBootstrapped: () => { state.bootstrapping = false; listeners.forEach((l) => l(state)) },
}
vi.mock('../stores/auth.store', () => ({
  useAuthStore: {
    getState: () => state,
    subscribe: (l: (s: State) => void) => { listeners.add(l); return () => listeners.delete(l) },
  },
}))

const { apiFetch } = await import('./client')

const ok = (body: unknown = {}) => ({ ok: true, status: 200, json: () => Promise.resolve(body) })

beforeEach(() => {
  vi.clearAllMocks()
  listeners.clear()
  state.accessToken = null
  state.bootstrapping = true
})

describe('a request made while the session is being restored', () => {
  it('waits for the boot refresh and leaves with its token, so the screen can show the device copy meanwhile', async () => {
    mockFetch.mockResolvedValue(ok({ items: [] }))
    const pending = apiFetch('/channels')
    await Promise.resolve()
    expect(mockFetch).not.toHaveBeenCalled()

    state.setAccessToken('fresh-token')
    state.setBootstrapped()
    await pending

    expect(mockFetch).toHaveBeenCalledTimes(1)
    const [, init] = mockFetch.mock.calls[0] as [string, RequestInit]
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer fresh-token')
  })

  it('goes out at once when no restore is running', async () => {
    state.bootstrapping = false
    state.accessToken = 'live-token'
    mockFetch.mockResolvedValue(ok())
    await apiFetch('/me')
    expect(mockFetch).toHaveBeenCalledTimes(1)
  })

  it('goes out at once when a token is already there, even before the restore is marked done', async () => {
    state.accessToken = 'live-token'
    mockFetch.mockResolvedValue(ok())
    await apiFetch('/me')
    expect(mockFetch).toHaveBeenCalledTimes(1)
  })
})
