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
let refreshInFlight: Promise<RefreshResult> | null = null

/**
 * What a refresh came to:
 *  - `ok`: a new access token, already stored;
 *  - `ended`: the server refused the session (it is over, and has been cleared);
 *  - `unavailable`: the server could not be asked (down, or the policy/database
 *    behind it is). That says nothing about the session, which is kept.
 */
export type RefreshResult =
  | { status: 'ok'; token: string }
  | { status: 'ended' }
  | { status: 'unavailable' }

/** A refresh moved the session from one workspace to another (or to none). */
export interface TenantMove {
  from: string
  /** The workspace the session is in now; null when the person has none left. */
  to: string | null
}

const tenantMovedListeners = new Set<(move: TenantMove) => void>()

/**
 * Registers what to do when a refresh puts the session in another workspace
 * than the one it was in: the person was removed from that workspace, and the
 * server moved them to the one they still belong to. Returns the unregister.
 */
export function onTenantMoved(listener: (move: TenantMove) => void): () => void {
  tenantMovedListeners.add(listener)
  return () => { tenantMovedListeners.delete(listener) }
}

/**
 * Exchanges the httpOnly refresh cookie for a new access token.
 * See RefreshResult for the three ways it can end.
 */
export function refreshAccessToken(): Promise<RefreshResult> {
  if (refreshInFlight) return refreshInFlight

  refreshInFlight = underRefreshLock(async (): Promise<RefreshResult> => {
    try {
      const res = await fetch(`${API_BASE}/auth/refresh`, {
        method: 'POST',
        // The refresh token is a cookie the script cannot read, so it has to
        // be attached by the browser.
        credentials: 'include',
      })
      if (!res.ok) {
        // Only the server refusing the session ends it. A failing server
        // (5xx, 503 included) is not an answer about the session: keep it and
        // try later.
        if (res.status === 401 || res.status === 403) {
          useAuthStore.getState().logout()
          return { status: 'ended' }
        }
        return { status: 'unavailable' }
      }
      const { access_token, tenant_id, moved } = (await res.json()) as { access_token: string; tenant_id?: string; moved?: boolean }
      const before = useAuthStore.getState().tenantId ?? null
      useAuthStore.getState().setAccessToken(access_token)
      // Only the server's word that the person was moved because they no longer belong
      // counts. A different tenant on its own is another tab having switched workspace
      // on purpose (the cookie is shared), and says nothing about removal.
      if (moved === true && tenant_id !== undefined && before && tenant_id !== before) {
        const move: TenantMove = { from: before, to: tenant_id || null }
        for (const listener of tenantMovedListeners) {
          try { listener(move) } catch (err) { console.error('[session] tenant move handler failed', err) }
        }
      }
      return { status: 'ok', token: access_token }
    } catch {
      // No network (the installed app opened offline): the session is not
      // over, the server simply cannot be asked. Keep it — the shell shows
      // with the connection bar — and refresh once the network is back.
      retryWhenOnline()
      return { status: 'unavailable' }
    } finally {
      refreshInFlight = null
    }
  })

  return refreshInFlight
}

/** The name every tab of the browser takes before it refreshes. */
const REFRESH_LOCK = 'nexus-refresh'

/**
 * Runs a refresh so that no other tab of this browser is refreshing at the same time.
 *
 * Tabs share the refresh cookie, so they all rotate the same token chain. The coalescing
 * above keeps one tab from asking twice; this keeps two tabs from asking together (a
 * removal closes every tab's socket at once and each asks for a new token). Under the
 * lock the second tab presents the cookie the first has already replaced. Where the
 * browser has no Web Locks, a small random delay spreads the tabs out, and the server
 * answers a token spent a few seconds ago with the successor it already made.
 */
function underRefreshLock<T>(run: () => Promise<T>): Promise<T> {
  const locks = typeof navigator !== 'undefined' ? navigator.locks : undefined
  if (locks && typeof locks.request === 'function') {
    return locks.request(REFRESH_LOCK, run) as Promise<T>
  }
  return new Promise((resolve, reject) => {
    setTimeout(() => { run().then(resolve, reject) }, Math.random() * 250)
  })
}

let waitingForNetwork = false

/** Refreshes once, the next time the browser reports the network is back. */
function retryWhenOnline() {
  if (waitingForNetwork || typeof window === 'undefined') return
  waitingForNetwork = true
  window.addEventListener(
    'online',
    () => {
      waitingForNetwork = false
      if (useAuthStore.getState().user) void refreshAccessToken()
    },
    { once: true },
  )
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

/**
 * Settles once the session restore started at page load is over; at once when
 * none is running or a token is already there. While it runs the screen may already show the copy kept on
 * the device, and requests wait here so none leaves without a token and races
 * the boot refresh with a 401.
 */
function sessionRestored(): Promise<void> {
  const store = useAuthStore
  const waiting = (s: { bootstrapping?: boolean; accessToken: string | null }) => !!s.bootstrapping && !s.accessToken
  if (!waiting(store.getState()) || typeof store.subscribe !== 'function') return Promise.resolve()
  return new Promise((resolve) => {
    const stop = store.subscribe((s) => {
      if (!waiting(s)) {
        stop()
        resolve()
      }
    })
  })
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
  await sessionRestored()
  let res = await send(path, options, jsonBody)
  if (res.status !== 401) return res

  const refreshed = await refreshAccessToken()
  // The session was kept but could not be renewed right now: say so, rather than
  // hand back the 401 that `toResult` reads as "signed out".
  if (refreshed.status === 'unavailable') throw new ApiError('Service unavailable', 503)
  if (refreshed.status === 'ended') return res

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

/** The answer to a request that asked "has this changed since the copy I hold?". */
export type Revalidated<T> =
  | { changed: false }
  | { changed: true; data: T; /** The version to ask with next time; null when the server issued none. */ etag: string | null }

/**
 * A GET that sends `If-None-Match` when the caller holds a copy. A 304 means the
 * copy is current: nothing is parsed and the caller keeps what it has. Any other
 * answer is `apiFetch`'s, including the sign-in handling of a 401.
 */
export async function apiFetchIfChanged<T>(path: string, etag: string | null): Promise<Revalidated<T>> {
  const res = await withRefreshRetry(path, etag ? { headers: { 'If-None-Match': etag } } : {}, true)
  if (res.status === 304) return { changed: false }
  const data = await toResult<T>(res)
  return { changed: true, data, etag: res.headers.get('ETag') }
}

/**
 * A request that is not made on behalf of a session: signing in itself
 * (providers, ask for a code, check a code).
 *
 * `apiFetch` reads a 401 as "the session ended": it refreshes, then logs out
 * whoever is on the page, and drops the body. Here a 401 is an answer, "wrong
 * code" or "code expired", and the body (`code`, `attempts_left`) is what the
 * screen needs. So: no token, no refresh, no logout, and the body is kept.
 */
export async function publicFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers: { ...(options.body ? { 'Content-Type': 'application/json' } : {}), ...(options.headers || {}) },
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError((body as any).error || (body as any).message || res.statusText, res.status, body)
  }
  return res.json()
}

/** Authenticated FormData upload. Does NOT set Content-Type — browser handles multipart boundary. */
export async function apiUpload<T>(path: string, body: FormData, method: 'POST' | 'PUT' = 'POST'): Promise<T> {
  return toResult<T>(await withRefreshRetry(path, { method, body }, false))
}
