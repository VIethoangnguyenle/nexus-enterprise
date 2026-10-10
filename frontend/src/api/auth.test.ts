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

const { authApi, completeGoogleSignIn, googleSignInUrl, leaveFor, GOOGLE_SIGN_IN_PATH } = await import('./auth')

function ok(body: unknown) {
  return { ok: true, status: 200, json: () => Promise.resolve(body) }
}
function unauthorized() {
  return { ok: false, status: 401, json: () => Promise.resolve({}) }
}

const me = {
  user: {
    id: 'u-1', username: 'alice.acme', ngac_node_id: 'n-1', email: 'alice@acme.com', union_id: 'x', display_name: 'Alice',
    email_verified: true, needs_profile: false,
  },
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

    await expect(completeGoogleSignIn()).resolves.toEqual({ needsProfile: false })

    const [refreshUrl, refreshInit] = mockFetch.mock.calls[0]!
    expect(refreshUrl).toBe('/api/auth/refresh')
    expect((refreshInit as RequestInit).credentials).toBe('include')

    const [meUrl, meInit] = mockFetch.mock.calls[1]!
    expect(meUrl).toBe('/api/me')
    expect((meInit as RequestInit).headers).toMatchObject({ Authorization: 'Bearer fresh-token' })

    // Only who they are is kept in the browser, not the address.
    expect(state.login).toHaveBeenCalledWith('fresh-token', { id: 'u-1', username: 'alice.acme', ngac_node_id: 'n-1' })
  })

  it('reports that a profile is still owed, so the caller can ask for it', async () => {
    mockFetch
      .mockResolvedValueOnce(ok({ access_token: 'fresh-token' }))
      .mockResolvedValueOnce(ok({ ...me, user: { ...me.user, needs_profile: true } }))

    await expect(completeGoogleSignIn()).resolves.toEqual({ needsProfile: true })
  })

  it('reports failure and signs nobody in when there is no session cookie', async () => {
    mockFetch.mockResolvedValueOnce(unauthorized())

    await expect(completeGoogleSignIn()).resolves.toBeNull()
    expect(state.login).not.toHaveBeenCalled()
    expect(mockFetch).toHaveBeenCalledTimes(1)
  })

  it('reports failure when /me cannot be loaded', async () => {
    mockFetch
      .mockResolvedValueOnce(ok({ access_token: 'fresh-token' }))
      .mockResolvedValueOnce({ ok: false, status: 500, json: () => Promise.resolve({}) })

    await expect(completeGoogleSignIn()).resolves.toBeNull()
    expect(state.login).not.toHaveBeenCalled()
  })
})

describe('GOOGLE_SIGN_IN_PATH', () => {
  it('points at the server-side start endpoint (a full-page navigation, not a fetch)', () => {
    expect(GOOGLE_SIGN_IN_PATH).toBe('/api/auth/google/start')
  })
})

describe('googleSignInUrl', () => {
  it('has no hint by default', () => {
    expect(googleSignInUrl()).toBe('/api/auth/google/start')
    expect(googleSignInUrl('   ')).toBe('/api/auth/google/start')
  })

  it('passes the address as login_hint, encoded', () => {
    expect(googleSignInUrl(' hoa.le+x@novapay.vn ')).toBe('/api/auth/google/start?login_hint=hoa.le%2Bx%40novapay.vn')
  })
})

// Each body is compared with the struct the Go handler binds; a field the
// handler does not read is silently dropped by the server, so the names matter.
describe('request bodies match the auth handlers', () => {
  const sent = () => {
    const [url, init] = mockFetch.mock.calls[0]! as [string, RequestInit]
    return { url, method: init.method, body: JSON.parse(String(init.body)) as unknown }
  }
  beforeEach(() => mockFetch.mockResolvedValue(ok({})))

  it('POST /api/auth/otp/request {identifier, type}', async () => {
    await authApi.requestOTP({ identifier: 'hoa.le@novapay.vn', type: 'email' })
    expect(sent()).toEqual({ url: '/api/auth/otp/request', method: 'POST', body: { identifier: 'hoa.le@novapay.vn', type: 'email' } })
  })

  it('POST /api/auth/otp/verify {session_id, code}', async () => {
    await authApi.verifyOTP({ session_id: 's-1', code: '481209' })
    expect(sent()).toEqual({ url: '/api/auth/otp/verify', method: 'POST', body: { session_id: 's-1', code: '481209' } })
  })

  it('a wrong code reaches the screen with its body, and does not end anyone’s session', async () => {
    const body = { code: 'otp_invalid', attempts_left: 4 }
    mockFetch.mockResolvedValueOnce({ ok: false, status: 401, json: () => Promise.resolve(body) })
    await expect(authApi.verifyOTP({ session_id: 's-1', code: '000000' })).rejects.toMatchObject({ status: 401, body })
    expect(state.logout).not.toHaveBeenCalled()
    expect(mockFetch).toHaveBeenCalledTimes(1)
  })

  it('PATCH /api/me/profile sends only the fields given', async () => {
    await authApi.updateProfile({ display_name: 'Phạm Thuý An' })
    expect(sent()).toEqual({ url: '/api/me/profile', method: 'PATCH', body: { display_name: 'Phạm Thuý An' } })
  })

  it('GET /api/me?workspace=<id> asks for the person in one workspace', async () => {
    await authApi.me('w 1')
    expect(mockFetch.mock.calls[0]![0]).toBe('/api/me?workspace=w%201')
    mockFetch.mockClear()
    await authApi.me()
    expect(mockFetch.mock.calls[0]![0]).toBe('/api/me')
  })

  it('POST /api/me/email/verify: {} asks for a code, {code} checks it', async () => {
    await authApi.requestEmailVerification()
    expect(sent()).toEqual({ url: '/api/me/email/verify', method: 'POST', body: {} })
    mockFetch.mockClear()
    await authApi.confirmEmailVerification('246810')
    expect(sent()).toEqual({ url: '/api/me/email/verify', method: 'POST', body: { code: '246810' } })
  })

  it('POST /api/me/email/verify/google has no body', async () => {
    await authApi.startEmailVerificationWithGoogle()
    const [url, init] = mockFetch.mock.calls[0]! as [string, RequestInit]
    expect(url).toBe('/api/me/email/verify/google')
    expect(init.method).toBe('POST')
    expect(init.body).toBeUndefined()
  })

  it('POST /api/me/workspaces {name} and nothing else', async () => {
    await authApi.createWorkspace('Tổ Đối soát')
    expect(sent()).toEqual({ url: '/api/me/workspaces', method: 'POST', body: { name: 'Tổ Đối soát' } })
  })

  it('POST /api/auth/switch-tenant {tenant_id}', async () => {
    await authApi.switchTenant('w-1')
    expect(sent()).toEqual({ url: '/api/auth/switch-tenant', method: 'POST', body: { tenant_id: 'w-1' } })
  })
})

describe('leaveFor', () => {
  it('refuses anything that is not a web address', () => {
    for (const bad of ['javascript:alert(1)', 'data:text/html,x', '//evil.test', '/relative', '', 'file:///etc/passwd']) {
      expect(() => leaveFor(bad), bad).toThrow()
    }
  })
})
