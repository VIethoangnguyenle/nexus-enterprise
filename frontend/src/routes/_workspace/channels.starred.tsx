import { createFileRoute } from '@tanstack/react-router'
import { StarredRoute } from './-starred-route'
import { validateWorkspaceSearch } from '../../lib/workspace'

export const Route = createFileRoute('/_workspace/channels/starred')({
  validateSearch: validateWorkspaceSearch,
  component: StarredRoute,
})
