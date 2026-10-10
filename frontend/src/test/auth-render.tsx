import type { ComponentType } from 'react'
import { render } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import {
  Outlet, RouterProvider, type RouteComponent, createMemoryHistory, createRootRoute, createRoute, createRouter,
} from '@tanstack/react-router'
import { vi } from 'vitest'
import { apiFetch, publicFetch } from '../api/client'
import { queryClient } from '../lib/query-client'
import { validateWorkspaceSearch } from '../lib/workspace'
import { useAuthStore } from '../stores/auth.store'
import { authFixtureApi, ID, resetFixtures } from './auth-fixtures'

/**
 * Mounts a sign-in screen on a real in-memory router, with every other
 * destination reduced to a marker, so a test reads where the screen sent the
 * person from the router's own location.
 */
export async function renderAuthScreen(
  Screen: ComponentType,
  path: '/login' | '/register' | '/workspace-select' | '/onboarding',
  url: string = path,
) {
  const root = createRootRoute({ component: Outlet })
  const marker = (name: string) => () => <div data-testid="arrived">{name}</div>
  const screenRoute = createRoute({ getParentRoute: () => root, path, component: Screen as RouteComponent })
  const others = (['/login', '/register', '/workspace-select', '/onboarding'] as const)
    .filter((p) => p !== path)
    .map((p) => createRoute({ getParentRoute: () => root, path: p, component: marker(p) }))
  const channels = createRoute({
    getParentRoute: () => root, path: '/channels', validateSearch: validateWorkspaceSearch, component: marker('/channels'),
  })
  const router = createRouter({
    routeTree: root.addChildren([screenRoute, ...others, channels]),
    history: createMemoryHistory({ initialEntries: [url] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  )
  return router
}

/** Sets the store and the fixture API to a known start. Call from `beforeEach`. */
export function startAuthTest({ signedIn }: { signedIn: boolean }) {
  resetFixtures()
  queryClient.clear()
  queryClient.setDefaultOptions({ queries: { retry: false, staleTime: 0 } })
  vi.mocked(apiFetch).mockImplementation(authFixtureApi as typeof apiFetch)
  vi.mocked(publicFetch).mockImplementation(authFixtureApi as typeof publicFetch)
  useAuthStore.setState(
    signedIn
      ? { user: { id: ID.me, username: 'hoa.le', ngac_node_id: ID.node }, accessToken: 'h.e30.s', tenantId: null, bootstrapping: false }
      : { user: null, accessToken: null, tenantId: null, bootstrapping: false },
  )
}

export const where = (router: Awaited<ReturnType<typeof renderAuthScreen>>) => ({
  path: router.state.location.pathname,
  search: router.state.location.search as Record<string, unknown>,
})
