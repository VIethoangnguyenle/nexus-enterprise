import { createFileRoute, redirect } from '@tanstack/react-router'
import { legacyAssetsRedirect } from '../../../lib/assets-search'
import { validateWorkspaceSearch } from '../../../lib/workspace'

// The old "new request" page is the form beside the request table now.
export const Route = createFileRoute('/assets/request/new')({
  validateSearch: validateWorkspaceSearch,
  beforeLoad: ({ search }) => {
    throw redirect({ to: '/assets', search: legacyAssetsRedirect('new-request', search), replace: true })
  },
})
