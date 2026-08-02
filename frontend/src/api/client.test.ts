import { describe, it, expect, vi, beforeEach } from 'vitest'

const mockFetch = vi.fn()
vi.stubGlobal('fetch', mockFetch)

// A tiny stand-in for the auth store, so these tests exercise the client's
// refresh logic rather than Zustand.
const state = {
  accessToken: 'expired-token' as string | null,
  user: { id: 'u1', username: 'alice' } as { id: string; username: string } | null,
  setAccessToken: vi.fn((t: string | null) => { state.accessToken = t }),
  setBootstrapped: vi.fn(),
  logout: vi.fn(() => { state.accessToken = null; state.user = null }),
}

vi.mock('../stores/auth.store', () => ({
  useAuthStore: { getState: () => state },
}))

const { apiFetch, refreshAccessToken, bootstrapSession } = await import('./client')

/** A 401 followed by whatever the caller queues next. */
function unauthorized() {
  return { ok: false, status: 401, json: () => Promise.resolve({}) }
}
function ok(body: unknown = {}) {
  return { ok: true, status: 200, json: () => Promise.resolve(body) }
}

beforeEach(() => {
  vi.clearAllMocks()
  state.accessToken = 'expired-token'
  state.user = { id: 'u1', username: 'alice' }
})

describe('apiFetch refresh handling', () => {
  it('refreshes once and retries the request after a 401', async () => {
    mockFetch
      .mockResolvedValueOnce(unauthorized())                          // original request
      .mockResolvedValueOnce(ok({ access_token: 'fresh-token' }))     // /auth/refresh
      .mockResolvedValueOnce(ok({ data: 'payload' }))                 // retry

    const result = await apiFetch<{ data: string }>('/documents')

    expect(result).toEqual({ data: 'payload' })
    expect(state.setAccessToken).toHaveBeenCalledWith('fresh-token')

    // The retry must carry the new token, not the stale one.
    const retry = mockFetch.mock.calls[2]!
    expect(retry[0]).toBe('/api/documents')
    expect((retry[1] as RequestInit).headers).toMatchObject({
      Authorization: 'Bearer fresh-token',
    })
  })

  it('sends the refresh request with credentials so the cookie is attached', async () => {
    mockFetch
      .mockResolvedValueOnce(unauthorized())
      .mockResolvedValueOnce(ok({ access_token: 'fresh-token' }))
      .mockResolvedValueOnce(ok({}))

    await apiFetch('/documents')

    const refreshCall = mockFetch.mock.calls[1]!
    expect(refreshCall[0]).toBe('/api/auth/refresh')
    expect(refreshCall[1]).toMatchObject({ method: 'POST', credentials: 'include' })
  })

  it('logs out and surfaces 401 when the refresh is rejected', async () => {
    mockFetch
      .mockResolvedValueOnce(unauthorized())
      .mockResolvedValueOnce(unauthorized()) // refresh rejected

    await expect(apiFetch('/documents')).rejects.toMatchObject({ status: 401 })
    expect(state.logout).toHaveBeenCalled()
  })

  it('does not retry more than once', async () => {
    mockFetch
      .mockResolvedValueOnce(unauthorized())
      .mockResolvedValueOnce(ok({ access_token: 'fresh-token' }))
      .mockResolvedValueOnce(unauthorized()) // still 401 with a brand-new token

    await expect(apiFetch('/documents')).rejects.toMatchObject({ status: 401 })
    // original + refresh + one retry, and nothing further
    expect(mockFetch).toHaveBeenCalledTimes(3)
  })
})

describe('refresh coalescing', () => {
  // Refresh tokens rotate, so a second concurrent refresh would present a token
  // the first had already spent — which the server treats as replay and answers
  // by killing the session. One in-flight refresh is what prevents that.
  it('issues a single refresh for concurrent callers', async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url === '/api/auth/refresh') {
        return new Promise((resolve) =>
          setTimeout(() => resolve(ok({ access_token: 'fresh-token' })), 10),
        )
      }
      return Promise.resolve(ok({}))
    })

    const [a, b, c] = await Promise.all([
      refreshAccessToken(),
      refreshAccessToken(),
      refreshAccessToken(),
    ])

    expect(a).toBe('fresh-token')
    expect(b).toBe('fresh-token')
    expect(c).toBe('fresh-token')

    const refreshCalls = mockFetch.mock.calls.filter((call) => call[0] === '/api/auth/refresh')
    expect(refreshCalls).toHaveLength(1)
  })

  it('allows a new refresh after the previous one settles', async () => {
    mockFetch.mockResolvedValue(ok({ access_token: 'fresh-token' }))

    await refreshAccessToken()
    await refreshAccessToken()

    const refreshCalls = mockFetch.mock.calls.filter((call) => call[0] === '/api/auth/refresh')
    expect(refreshCalls).toHaveLength(2)
  })
})

describe('bootstrapSession', () => {
  it('trades the cookie for an access token when a user is persisted', async () => {
    state.accessToken = null
    mockFetch.mockResolvedValue(ok({ access_token: 'restored-token' }))

    await bootstrapSession()

    expect(mockFetch).toHaveBeenCalledWith('/api/auth/refresh', expect.objectContaining({
      credentials: 'include',
    }))
    expect(state.setAccessToken).toHaveBeenCalledWith('restored-token')
    expect(state.setBootstrapped).toHaveBeenCalled()
  })

  it('does not call refresh when nobody was signed in', async () => {
    state.user = null
    state.accessToken = null

    await bootstrapSession()

    expect(mockFetch).not.toHaveBeenCalled()
    expect(state.setBootstrapped).toHaveBeenCalled()
  })
})
