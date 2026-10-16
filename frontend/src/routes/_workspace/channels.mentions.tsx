import { createFileRoute } from '@tanstack/react-router'
import { MentionsRoute } from './-mentions-route'
import { validateWorkspaceSearch } from '../../lib/workspace'

export const Route = createFileRoute('/_workspace/channels/mentions')({
  validateSearch: validateWorkspaceSearch,
  component: MentionsRoute,
})
