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

const { apiFetch, publicFetch, ApiError, refreshAccessToken, bootstrapSession } = await import('./client')

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

// Signing in is not a request made on behalf of a session: a 401 from it is an
// answer ("wrong code", "code expired"), and treating it as an expired session
// would refresh, then log out whoever is on the page and drop the answer's body.
describe('publicFetch', () => {
  it('keeps the body of a 401 and neither refreshes nor logs anyone out', async () => {
    const body = { code: 'otp_invalid', attempts_left: 3, message: 'invalid otp code' }
    mockFetch.mockResolvedValueOnce({ ok: false, status: 401, json: () => Promise.resolve(body) })

    const err = await publicFetch('/auth/otp/verify', { method: 'POST', body: '{}' }).catch((e) => e)

    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 401, body })
    expect(mockFetch).toHaveBeenCalledTimes(1) // no refresh, no retry
    expect(state.logout).not.toHaveBeenCalled()
  })

  it('sends no Authorization header even when a token is held', async () => {
    mockFetch.mockResolvedValueOnce(ok({ session_id: 's' }))
    await publicFetch('/auth/otp/request', { method: 'POST', body: '{"identifier":"x"}' })
    const [url, init] = mockFetch.mock.calls[0]! as [string, RequestInit]
    expect(url).toBe('/api/auth/otp/request')
    expect(init.headers).toEqual({ 'Content-Type': 'application/json' })
  })

  it('returns the parsed body', async () => {
    mockFetch.mockResolvedValueOnce(ok({ google: true }))
    await expect(publicFetch('/auth/providers')).resolves.toEqual({ google: true })
  })

  it.each([
    [429, { code: 'otp_rate_limited', retry_after_seconds: 700 }],
    [503, { code: 'otp_unavailable' }],
    [500, { code: 'internal' }],
  ])('keeps the body of a %i', async (status, body) => {
    mockFetch.mockResolvedValueOnce({ ok: false, status, json: () => Promise.resolve(body) })
    await expect(publicFetch('/auth/otp/request', { method: 'POST' })).rejects.toMatchObject({ status, body })
  })

  it('survives an answer that is not JSON', async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 502, statusText: 'Bad Gateway', json: () => Promise.reject(new Error('html')) })
    await expect(publicFetch('/auth/providers')).rejects.toMatchObject({ status: 502, message: 'Bad Gateway' })
  })

  it('lets a network failure through as it is', async () => {
    mockFetch.mockRejectedValueOnce(new TypeError('Failed to fetch'))
    await expect(publicFetch('/auth/providers')).rejects.toBeInstanceOf(TypeError)
  })
})
