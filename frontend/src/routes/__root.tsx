import { createRootRoute, Outlet } from '@tanstack/react-router'
import { useEffect } from 'react'
import { bootstrapSession } from '../api/client'
import { useAuthStore } from '../stores/auth.store'
import { Spinner, Toaster } from '../components/primitives'
import { getRestoredCopy } from '../lib/query-persist'

let sessionRestore: Promise<void> | null = null

/**
 * Starts restoring the session, once per page. The router calls this before
 * any route's own loading, so the refresh request leaves while the routes are
 * still being matched and rendered instead of after the first paint.
 */
export function restoreSessionOnce(): Promise<void> {
  sessionRestore ??= bootstrapSession()
  return sessionRestore
}

/** Test seam: forget the restore of a previous test. */
export function resetSessionRestore() {
  sessionRestore = null
}

export const Route = createRootRoute({
  // Not awaited: waiting here would hold the routes back, which is what the
  // spinner below already does for the render alone.
  beforeLoad: () => {
    void restoreSessionOnce()
  },
  component: RootRoute,
})

/**
 * Re-establishes the session before any route renders.
 *
 * The access token is memory-only, so a page reload starts with a persisted
 * user and no token. Without a copy of the last session on the device the tree
 * is held back for that one round-trip. With one, the app renders from the copy
 * at once, as Telegram does, and requests wait in the API client until the
 * refresh is done, so none leaves without a token.
 */
function RootRoute() {
  const bootstrapping = useAuthStore((s) => s.bootstrapping)

  useEffect(() => {
    void restoreSessionOnce()
  }, [])

  if (bootstrapping && !getRestoredCopy()) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-background">
        <Spinner />
      </div>
    )
  }

  // One toaster for every route: the global mutation error handler toasts from
  // auth and onboarding pages too, outside the workspace shell.
  return (
    <>
      <Outlet />
      <Toaster />
    </>
  )
}
