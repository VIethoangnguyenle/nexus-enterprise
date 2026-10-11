import { createFileRoute, redirect, Navigate } from '@tanstack/react-router'
import { ApprovalScreen } from '../../components/approval/ApprovalScreen'
import { validateApprovalSearch } from '../../lib/approval-search'
import { useWorkspaceType } from '../../hooks/useWorkspaceType'
import { keys } from '../../hooks/keys'
import type { QueryClient } from '@tanstack/react-query'

// The tab, the open request or template and the template builder are part of
// the URL, so reload, Back/Forward and links keep the place. `?ws=` is kept by
// the layout.
export const Route = createFileRoute('/_workspace/approval')({
  validateSearch: validateApprovalSearch,
  beforeLoad: ({ search, context }) => {
    const qc = (context as { queryClient?: QueryClient } | undefined)?.queryClient
    const cached = qc?.getQueryData<{ workspaces: Array<{ id: string; type?: string }> }>(keys.workspaces.all())
    const active = cached?.workspaces.find((w) => w.id === search.ws) ?? cached?.workspaces[0]
    if (active?.type === 'personal') {
      throw redirect({ to: '/channels', search: { ...(search.ws ? { ws: search.ws } : {}) }, replace: true })
    }
  },
  component: ApprovalRoute,
})

function ApprovalRoute() {
  const { isPersonal, isLoading } = useWorkspaceType()
  const search = Route.useSearch()
  if (!isLoading && isPersonal) {
    return <Navigate to="/channels" search={{ ...(search.ws ? { ws: search.ws } : {}) }} replace />
  }
  return <ApprovalScreen />
}
