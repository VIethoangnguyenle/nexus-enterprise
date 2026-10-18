import { createFileRoute } from '@tanstack/react-router'
import { SearchRoute } from './-search-route'
import { parseChatSearch } from '../../lib/chat-search'
import { validateWorkspaceSearch } from '../../lib/workspace'

// The words and the filters of Tìm trong chat live in the address, so back, forward and reload keep them.
export const Route = createFileRoute('/_workspace/channels/search')({
  validateSearch: (search: Record<string, unknown>) => ({ ...validateWorkspaceSearch(search), ...parseChatSearch(search) }),
  component: SearchRoute,
})
