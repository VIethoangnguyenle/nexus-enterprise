import { createFileRoute, redirect } from '@tanstack/react-router'
import { legacyAssetsRedirect } from '../../lib/assets-search'
import { validateWorkspaceSearch } from '../../lib/workspace'

// A shared link to one asset opens the list with its panel; on a phone the panel is a full screen.
export const Route = createFileRoute('/assets/$assetId')({
  validateSearch: validateWorkspaceSearch,
  beforeLoad: ({ params, search }) => {
    throw redirect({ to: '/assets', search: legacyAssetsRedirect({ asset: params.assetId }, search), replace: true })
  },
})
