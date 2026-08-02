import { useAuthStore } from '../stores/auth.store'

const API_BASE = '/api'

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public body?: unknown,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

/**
 * In-flight refresh, shared by every caller.
 *
 * A page typically fires several requests at once, so an expired access token
 * produces a burst of 401s rather than one. Without this, each would start its
 * own refresh — and since refresh tokens rotate, the second call would present
 * a token the first had already spent, which the server treats as replay and
 * punishes by killing the whole session. Coalescing is not an optimisation
 * here; it is what keeps rotation from logging the user out.
 */
let refreshInFlight: Promise<string | null> | null = null

/**
 * Exchanges the httpOnly refresh cookie for a new access token.
 * Resolves to the new token, or null if the session is over.
 */
export function refreshAccessToken(): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight

  refreshInFlight = (async () => {
    try {
      const res = await fetch(`${API_BASE}/auth/refresh`, {
        method: 'POST',
        // The refresh token is a cookie the script cannot read, so it has to
        // be attached by the browser.
        credentials: 'include',
      })
      if (!res.ok) {
        useAuthStore.getState().logout()
        return null
      }
      const { access_token } = (await res.json()) as { access_token: string }
      useAuthStore.getState().setAccessToken(access_token)
      return access_token
    } catch {
      useAuthStore.getState().logout()
      return null
    } finally {
      refreshInFlight = null
    }
  })()

  return refreshInFlight
}

/**
 * Restores the session on page load.
 *
 * The access token is memory-only, so after a reload there is a persisted user
 * but no token. This trades the refresh cookie for one, and marks bootstrapping
 * done either way so the router stops waiting.
 */
export async function bootstrapSession(): Promise<void> {
  const { user, setBootstrapped } = useAuthStore.getState()
  if (!user) {
    setBootstrapped()
    return
  }
  await refreshAccessToken()
  useAuthStore.getState().setBootstrapped()
}

/** Ends the session server-side, clearing the refresh cookie. */
export async function logoutSession(): Promise<void> {
  const token = useAuthStore.getState().accessToken
  try {
    await fetch(`${API_BASE}/auth/logout`, {
      method: 'POST',
      credentials: 'include',
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
  } catch {
    // The local session is cleared regardless — a network failure must not
    // leave the user apparently signed in.
  }
  useAuthStore.getState().logout()
}

/** Sends a request with the current access token attached. */
function send(path: string, options: RequestInit, jsonBody: boolean): Promise<Response> {
  const token = useAuthStore.getState().accessToken
  const headers: HeadersInit = {
    ...(jsonBody ? { 'Content-Type': 'application/json' } : {}),
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(options.headers || {}),
  }
  return fetch(`${API_BASE}${path}`, { ...options, headers })
}

/**
 * Runs a request, and on a 401 refreshes once and runs it again.
 *
 * The retry is deliberately capped at one: if the request still 401s with a
 * token minted moments ago, the problem is authorization rather than expiry,
 * and retrying further would just loop.
 */
async function withRefreshRetry(path: string, options: RequestInit, jsonBody: boolean): Promise<Response> {
  let res = await send(path, options, jsonBody)
  if (res.status !== 401) return res

  const token = await refreshAccessToken()
  if (!token) return res

  res = await send(path, options, jsonBody)
  return res
}

async function toResult<T>(res: Response): Promise<T> {
  if (res.status === 401) {
    useAuthStore.getState().logout()
    throw new ApiError('Unauthorized', 401)
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError((body as any).error || (body as any).message || res.statusText, res.status, body)
  }
  if (res.status === 204) return {} as T
  return res.json()
}

/** Authenticated fetch wrapper. Injects the access token and handles JSON. */
export async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  return toResult<T>(await withRefreshRetry(path, options, true))
}

/** Authenticated FormData upload. Does NOT set Content-Type — browser handles multipart boundary. */
export async function apiUpload<T>(path: string, body: FormData): Promise<T> {
  return toResult<T>(await withRefreshRetry(path, { method: 'POST', body }, false))
}
