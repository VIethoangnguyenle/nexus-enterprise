import { redirect } from '@tanstack/react-router'
import { useAuthStore } from '../stores/auth.store'

/**
 * Route guards for the sign-in group, run once when a navigation starts
 * (`beforeLoad`), not on every change of the session. That is deliberate: the
 * moment a sign-in succeeds the store gains a user, and a guard that reacted to
 * that would race the screen's own "where next" decision (profile, or
 * workspace selection).
 *
 * The root route holds rendering until the boot-time refresh has settled, so by
 * the time these run a persisted user really is a live session.
 */

/** Sign-in pages are for people who are not signed in; anyone else goes on to choose a workspace. */
export function redirectIfSignedIn(): void {
  if (useAuthStore.getState().isAuthenticated()) {
    throw redirect({ to: '/workspace-select', replace: true })
  }
}

/** Pages after sign-in (profile, workspace choice, new workspace) need a session. */
export function requireSignedIn(): void {
  if (!useAuthStore.getState().isAuthenticated()) {
    throw redirect({ to: '/login', replace: true })
  }
}

/**
 * The workspace needs a session too, and decides before its code loads: a
 * signed-out visitor sent through the shell would download it, ask for the
 * workspaces without a token, and the 401 that came back would end a session
 * that never was. A shared link's workspace rides along to the login page.
 */
export function requireSignedInToWorkspace({ search }: { search: Record<string, unknown> }): void {
  if (!useAuthStore.getState().isAuthenticated()) {
    const ws = typeof search.ws === 'string' && search.ws ? { ws: search.ws } : {}
    throw redirect({ to: '/login', search: ws, replace: true })
  }
}
