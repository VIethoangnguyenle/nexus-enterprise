import { describe, it, expect, vi, beforeEach } from 'vitest'

const mockFetch = vi.fn()
vi.stubGlobal('fetch', mockFetch)

// Minimal stand-in for the auth store: no persisted user, no token — the state
// the browser is in when Google redirects back to the SPA.
const state = {
  accessToken: null as string | null,
  user: null as { id: string; username: string } | null,
  setAccessToken: vi.fn((t: string | null) => { state.accessToken = t }),
  setBootstrapped: vi.fn(),
  login: vi.fn((t: string, u: { id: string; username: string }) => { state.accessToken = t; state.user = u }),
  logout: vi.fn(() => { state.accessToken = null; state.user = null }),
}

vi.mock('../stores/auth.store', () => ({
  useAuthStore: { getState: () => state },
}))

const { completeGoogleSignIn, GOOGLE_SIGN_IN_PATH } = await import('./auth')

function ok(body: unknown) {
  return { ok: true, status: 200, json: () => Promise.resolve(body) }
}
function unauthorized() {
  return { ok: false, status: 401, json: () => Promise.resolve({}) }
}

const me = {
  user: { id: 'u-1', username: 'alice.acme', ngac_node_id: 'n-1', email: 'alice@acme.com', union_id: 'x', display_name: 'Alice' },
  current_tenant: { id: 't-1', name: 'Acme', role: 'member', open_id: 'o-1' },
}

beforeEach(() => {
  vi.clearAllMocks()
  state.accessToken = null
  state.user = null
})

describe('completeGoogleSignIn', () => {
  it('trades the refresh cookie for an access token, loads /me, and signs the user in', async () => {
    mockFetch
      .mockResolvedValueOnce(ok({ access_token: 'fresh-token' })) // /auth/refresh
      .mockResolvedValueOnce(ok(me))                              // /me

    await expect(completeGoogleSignIn()).resolves.toBe(true)

    const [refreshUrl, refreshInit] = mockFetch.mock.calls[0]!
    expect(refreshUrl).toBe('/api/auth/refresh')
    expect((refreshInit as RequestInit).credentials).toBe('include')

    const [meUrl, meInit] = mockFetch.mock.calls[1]!
    expect(meUrl).toBe('/api/me')
    expect((meInit as RequestInit).headers).toMatchObject({ Authorization: 'Bearer fresh-token' })

    expect(state.login).toHaveBeenCalledWith('fresh-token', me.user)
  })

  it('reports failure and signs nobody in when there is no session cookie', async () => {
    mockFetch.mockResolvedValueOnce(unauthorized())

    await expect(completeGoogleSignIn()).resolves.toBe(false)
    expect(state.login).not.toHaveBeenCalled()
    expect(mockFetch).toHaveBeenCalledTimes(1)
  })

  it('reports failure when /me cannot be loaded', async () => {
    mockFetch
      .mockResolvedValueOnce(ok({ access_token: 'fresh-token' }))
      .mockResolvedValueOnce({ ok: false, status: 500, json: () => Promise.resolve({}) })

    await expect(completeGoogleSignIn()).resolves.toBe(false)
    expect(state.login).not.toHaveBeenCalled()
  })
})

describe('GOOGLE_SIGN_IN_PATH', () => {
  it('points at the server-side start endpoint (a full-page navigation, not a fetch)', () => {
    expect(GOOGLE_SIGN_IN_PATH).toBe('/api/auth/google/start')
  })
})
