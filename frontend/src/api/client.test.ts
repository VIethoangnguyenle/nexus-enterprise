import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

const mockFetch = vi.fn()
vi.stubGlobal('fetch', mockFetch)

// A tiny stand-in for the auth store, so these tests exercise the client's
// refresh logic rather than Zustand.
const state = {
  accessToken: 'expired-token' as string | null,
  /** The tenant the held token speaks for: derived from the token in the real store. */
  tenantId: 'ws-1' as string | null,
  user: { id: 'u1', username: 'alice' } as { id: string; username: string } | null,
  setAccessToken: vi.fn((t: string | null) => { state.accessToken = t }),
  setBootstrapped: vi.fn(),
  logout: vi.fn(() => { state.accessToken = null; state.user = null }),
}

vi.mock('../stores/auth.store', () => ({
  useAuthStore: { getState: () => state },
}))

const { apiFetch, publicFetch, ApiError, refreshAccessToken, bootstrapSession, onTenantMoved } = await import('./client')

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
  state.tenantId = 'ws-1'
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

    expect(a).toEqual({ status: 'ok', token: 'fresh-token' })
    expect(b).toEqual({ status: 'ok', token: 'fresh-token' })
    expect(c).toEqual({ status: 'ok', token: 'fresh-token' })

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

  // Opening the installed app with no network: the session is not over, the
  // server simply cannot be asked. Keep it, show the shell, and finish the
  // refresh once the network is back.
  it('keeps the session when the network is down, and refreshes when it returns', async () => {
    state.accessToken = null
    mockFetch.mockRejectedValueOnce(new TypeError('Failed to fetch'))

    await bootstrapSession()

    expect(state.logout).not.toHaveBeenCalled()
    expect(state.user).not.toBeNull()
    expect(state.setBootstrapped).toHaveBeenCalled()

    mockFetch.mockResolvedValueOnce(ok({ access_token: 'back-online' }))
    window.dispatchEvent(new Event('online'))
    await vi.waitFor(() => expect(state.setAccessToken).toHaveBeenCalledWith('back-online'))
  })

  it('keeps the session when the server is failing, and ends it only when the server refuses it', async () => {
    mockFetch.mockResolvedValueOnce({ ok: false, status: 503, json: () => Promise.resolve({}) })
    expect(await refreshAccessToken()).toEqual({ status: 'unavailable' })
    expect(state.logout).not.toHaveBeenCalled()

    mockFetch.mockResolvedValueOnce(unauthorized())
    expect(await refreshAccessToken()).toEqual({ status: 'ended' })
    expect(state.logout).toHaveBeenCalled()
  })

  it('does not call refresh when nobody was signed in', async () => {
    state.user = null
    state.accessToken = null

    await bootstrapSession()

    expect(mockFetch).not.toHaveBeenCalled()
    expect(state.setBootstrapped).toHaveBeenCalled()
  })
})

// An outage is not an answer about the session. The refresh endpoint answers 503
// when the policy or the database cannot be asked (and leaves the cookie, so the
// next try can work); a request that only wanted a new token must not end the
// session on that.
describe('a refresh that cannot be answered', () => {
  const unavailable = () => ({ ok: false, status: 503, json: () => Promise.resolve({ code: 'unavailable' }) })

  it('surfaces a 503 and keeps the session, instead of logging out on the original 401', async () => {
    mockFetch
      .mockResolvedValueOnce(unauthorized()) // the request, with an expired token
      .mockResolvedValueOnce(unavailable())  // the refresh

    await expect(apiFetch('/documents')).rejects.toMatchObject({ status: 503 })
    expect(state.logout).not.toHaveBeenCalled()
    expect(state.user).not.toBeNull()
  })

  it('does the same when the network is down at that moment', async () => {
    mockFetch
      .mockResolvedValueOnce(unauthorized())
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))

    await expect(apiFetch('/documents')).rejects.toMatchObject({ status: 503 })
    expect(state.logout).not.toHaveBeenCalled()
  })

  it('still ends the session when the server refuses it', async () => {
    mockFetch.mockResolvedValueOnce(unauthorized()).mockResolvedValueOnce(unauthorized())

    await expect(apiFetch('/documents')).rejects.toMatchObject({ status: 401 })
    expect(state.logout).toHaveBeenCalled()
  })

  it('works again once the server answers', async () => {
    mockFetch
      .mockResolvedValueOnce(unauthorized())
      .mockResolvedValueOnce(unavailable())
      .mockResolvedValueOnce(unauthorized())
      .mockResolvedValueOnce(ok({ access_token: 'fresh-token', tenant_id: 'ws-1' }))
      .mockResolvedValueOnce(ok({ data: 'payload' }))

    await expect(apiFetch('/documents')).rejects.toMatchObject({ status: 503 })
    await expect(apiFetch<{ data: string }>('/documents')).resolves.toEqual({ data: 'payload' })
    expect(state.logout).not.toHaveBeenCalled()
  })
})

// A person removed from the workspace of their token keeps their session: the
// server moves it to the workspace they still belong to (or to none) and says so
// with `moved: true`. Only that flag is a membership move; a different tenant on its
// own is just another tab having switched workspace on purpose.
describe('a refresh that moves the session to another workspace', () => {
  const moved = vi.fn()
  let stop: () => void
  beforeEach(() => { stop = onTenantMoved(moved) })
  afterEach(() => { stop(); moved.mockReset() })

  it('tells the app which workspace it left and which it is in now', async () => {
    mockFetch.mockResolvedValueOnce(ok({ access_token: 'fresh-token', tenant_id: 'ws-2', moved: true }))

    expect(await refreshAccessToken()).toEqual({ status: 'ok', token: 'fresh-token' })

    expect(state.setAccessToken).toHaveBeenCalledWith('fresh-token')
    expect(moved).toHaveBeenCalledExactlyOnceWith({ from: 'ws-1', to: 'ws-2' })
  })

  it('says "to: null" when no workspace is left', async () => {
    mockFetch.mockResolvedValueOnce(ok({ access_token: 'fresh-token', tenant_id: '', moved: true }))

    await refreshAccessToken()

    expect(moved).toHaveBeenCalledExactlyOnceWith({ from: 'ws-1', to: null })
  })

  // The paired allow: another tab switched workspace on purpose, the cookie followed,
  // and this tab's refresh lands in a different tenant. That is not a removal.
  it('says nothing when the tenant differs but the server did not say the person was moved', async () => {
    mockFetch.mockResolvedValueOnce(ok({ access_token: 'fresh-token', tenant_id: 'ws-2' }))

    await refreshAccessToken()

    expect(state.setAccessToken).toHaveBeenCalledWith('fresh-token')
    expect(moved).not.toHaveBeenCalled()
  })

  it('says nothing for an ordinary refresh, for a session that had no workspace to leave, or for an older server', async () => {
    mockFetch.mockResolvedValueOnce(ok({ access_token: 'fresh-token', tenant_id: 'ws-1' }))
    await refreshAccessToken()
    state.tenantId = null
    mockFetch.mockResolvedValueOnce(ok({ access_token: 'fresh-token', tenant_id: 'ws-2', moved: true }))
    await refreshAccessToken()
    state.tenantId = 'ws-1'
    mockFetch.mockResolvedValueOnce(ok({ access_token: 'fresh-token-2' }))
    await refreshAccessToken()

    expect(moved).not.toHaveBeenCalled()
  })
})

// Two tabs of one browser share the refresh cookie, and a removal tells every tab's
// socket to ask for a new token at the same moment. A refresh therefore runs under a
// browser-wide lock: the second tab waits, then presents the cookie the first one has
// already replaced, instead of the token the first has just spent.
describe('refresh across tabs', () => {
  /** One Web Locks manager shared by every "tab" (module instance) below. */
  function fakeLocks() {
    const queues = new Map<string, Promise<unknown>>()
    const names: string[] = []
    return {
      names,
      request: vi.fn(async (name: string, cb: () => Promise<unknown>) => {
        names.push(name)
        const previous = queues.get(name) ?? Promise.resolve()
        const run = previous.then(cb, cb)
        queues.set(name, run.catch(() => undefined))
        return run
      }),
    }
  }

  afterEach(() => { vi.unstubAllGlobals(); vi.stubGlobal('fetch', mockFetch) })

  it('serialises the refreshes of two tabs under one named lock', async () => {
    const locks = fakeLocks()
    vi.stubGlobal('navigator', { ...navigator, locks })
    let inFlight = 0
    let overlapped = false
    mockFetch.mockImplementation(async () => {
      inFlight++
      if (inFlight > 1) overlapped = true
      await new Promise((r) => setTimeout(r, 15))
      inFlight--
      return ok({ access_token: 'fresh-token', tenant_id: 'ws-1' })
    })
    vi.resetModules()
    const otherTab = await import('./client')

    const results = await Promise.all([refreshAccessToken(), otherTab.refreshAccessToken()])

    expect(results.map((r) => r.status)).toEqual(['ok', 'ok'])
    expect(overlapped, 'the two tabs refreshed at the same time').toBe(false)
    expect(locks.names).toEqual(['nexus-refresh', 'nexus-refresh'])
  })

  it('still refreshes where the browser has no Web Locks', async () => {
    vi.stubGlobal('navigator', { ...navigator, locks: undefined })
    mockFetch.mockResolvedValueOnce(ok({ access_token: 'fresh-token', tenant_id: 'ws-1' }))

    expect(await refreshAccessToken()).toEqual({ status: 'ok', token: 'fresh-token' })
  })

  it('a refresh that fails inside the lock releases it for the next', async () => {
    const locks = fakeLocks()
    vi.stubGlobal('navigator', { ...navigator, locks })
    mockFetch.mockRejectedValueOnce(new TypeError('Failed to fetch'))
    expect(await refreshAccessToken()).toEqual({ status: 'unavailable' })

    mockFetch.mockResolvedValueOnce(ok({ access_token: 'fresh-token', tenant_id: 'ws-1' }))
    expect(await refreshAccessToken()).toEqual({ status: 'ok', token: 'fresh-token' })
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
