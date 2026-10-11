import { createFileRoute, redirect, Navigate } from '@tanstack/react-router'
import { AssetsScreen } from '../../components/assets/AssetsScreen'
import { validateAssetsSearch } from '../../lib/assets-search'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { keys } from '../../hooks/keys'
import type { QueryClient } from '@tanstack/react-query'

// The tab, the filters, the page and the open asset, request or type are part of
// the URL, so reload, Back/Forward and links keep the place. `?ws=` is kept by
// the layout. Tài sản lives in the shared shell: one sign-in guard, one
// WebSocket, one workspace switcher, one logout.
export const Route = createFileRoute('/_workspace/assets')({
  validateSearch: validateAssetsSearch,
  beforeLoad: ({ search, context }) => {
    const qc = (context as { queryClient?: QueryClient } | undefined)?.queryClient
    const cached = qc?.getQueryData<{ workspaces: Array<{ id: string; type?: string }> }>(keys.workspaces.all())
    const active = cached?.workspaces.find((w) => w.id === search.ws) ?? cached?.workspaces[0]
    if (active?.type === 'personal') {
      throw redirect({ to: '/channels', search: { ...(search.ws ? { ws: search.ws } : {}) }, replace: true })
    }
  },
  component: AssetsRoute,
})

function AssetsRoute() {
  const { isPersonal, isLoading } = useWorkspaceType()
  const search = Route.useSearch()
  if (!isLoading && isPersonal) {
    return <Navigate to="/channels" search={{ ...(search.ws ? { ws: search.ws } : {}) }} replace />
  }
  return <AssetsScreen />
}
