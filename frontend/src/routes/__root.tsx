import { createRootRoute, Outlet } from '@tanstack/react-router'
import { useEffect } from 'react'
import { bootstrapSession } from '../api/client'
import { useAuthStore } from '../stores/auth.store'
import { Spinner } from '../components/primitives'

export const Route = createRootRoute({
  component: RootRoute,
})

/**
 * Re-establishes the session before any route renders.
 *
 * The access token is memory-only, so a page reload starts with a persisted
 * user and no token. Rendering the app in that state would fire every route's
 * queries with no Authorization header, and the resulting 401s would race the
 * refresh. Holding the tree back for one round-trip avoids that entirely.
 */
function RootRoute() {
  const bootstrapping = useAuthStore((s) => s.bootstrapping)

  useEffect(() => {
    void bootstrapSession()
  }, [])

  if (bootstrapping) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-background">
        <Spinner />
      </div>
    )
  }

  return <Outlet />
}
