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
