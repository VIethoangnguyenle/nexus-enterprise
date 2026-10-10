import { createFileRoute, redirect } from '@tanstack/react-router'
import { validateWorkspaceSearch } from '../../lib/workspace'

// Văn bản is a group inside Tài liệu now, not a page of its own: the old
// address lands on the group and keeps the workspace.
export const Route = createFileRoute('/_workspace/documents/')({
  validateSearch: validateWorkspaceSearch,
  beforeLoad: ({ search }) => {
    throw redirect({ to: '/drive', search: { ...(search.ws ? { ws: search.ws } : {}), view: 'texts' }, replace: true })
  },
})
