import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface User {
  id: string
  username: string
  ngac_node_id?: string
}

/**
 * Reads the tenant a token is scoped to.
 *
 * The token is the authority on which tenant a request runs as, so anything
 * that needs to scope client state by tenant reads it from here rather than
 * tracking it separately — a second copy could disagree with the token after a
 * refresh or a tenant switch, and the disagreement would be silent.
 *
 * This is a cache/scoping hint only. Nothing here is trusted for authorization;
 * the server re-validates the signature on every request.
 */
export function tenantIdFromToken(token: string | null): string | null {
  if (!token) return null
  const payload = token.split('.')[1]
  if (!payload) return null
  try {
    const json = atob(payload.replace(/-/g, '+').replace(/_/g, '/'))
    return (JSON.parse(json) as { tenant_id?: string }).tenant_id ?? null
  } catch {
    return null
  }
}

interface AuthState {
  /** Short-lived access token. Deliberately NOT persisted — see the store note. */
  accessToken: string | null
  /** Tenant the current access token is scoped to. Derived, never set directly. */
  tenantId: string | null
  user: User | null
  /** True while the boot-time refresh is still deciding whether we have a session. */
  bootstrapping: boolean
  login: (accessToken: string, user: User) => void
  setAccessToken: (accessToken: string | null) => void
  setBootstrapped: () => void
  logout: () => void
  isAuthenticated: () => boolean
}

const storageKey = (() => {
  const user = new URLSearchParams(window.location.search).get('user')
  return `ngac-auth${user ? `-${user}` : ''}`
})()

/**
 * Auth state.
 *
 * The access token lives in memory only. Persisting it would put a bearer
 * credential back in localStorage, which is exactly what the refresh-token
 * cookie exists to avoid — a token readable by any script on the page is a
 * token an XSS payload can exfiltrate.
 *
 * Only `user` is persisted, and purely so the UI can render a signed-in shell
 * immediately on reload. The actual session is re-established from the
 * httpOnly refresh cookie by the boot refresh in `client.ts`; if that fails,
 * `logout()` clears this and the user lands back on the sign-in screen.
 */
export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      accessToken: null,
      tenantId: null,
      user: null,
      bootstrapping: true,

      login: (accessToken, user) =>
        set({ accessToken, tenantId: tenantIdFromToken(accessToken), user, bootstrapping: false }),

      setAccessToken: (accessToken) =>
        set({ accessToken, tenantId: tenantIdFromToken(accessToken) }),

      setBootstrapped: () => set({ bootstrapping: false }),

      logout: () => {
        // Clear permission cache on tenant switch / logout
        import('../stores/permission.store').then(m => m.usePermissionStore.getState().clear())
        set({ accessToken: null, tenantId: null, user: null, bootstrapping: false })
      },

      /**
       * Reflects the persisted user, not the access token: right after a reload
       * the token is legitimately absent while the refresh is in flight, and
       * treating that as signed-out would bounce the user to the login screen
       * on every refresh of the page.
       */
      isAuthenticated: () => !!get().user,
    }),
    {
      name: storageKey,
      partialize: (state) => ({ user: state.user }),
    },
  ),
)
