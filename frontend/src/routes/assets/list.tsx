import { createFileRoute, redirect } from '@tanstack/react-router'
import { legacyAssetsRedirect } from '../../lib/assets-search'
import { validateWorkspaceSearch } from '../../lib/workspace'

// The old /assets pages became tabs of one screen in the shared shell; a saved link keeps working.
export const Route = createFileRoute('/assets/list')({
  validateSearch: validateWorkspaceSearch,
  beforeLoad: ({ search }) => {
    throw redirect({ to: '/assets', search: legacyAssetsRedirect('list', search), replace: true })
  },
})
