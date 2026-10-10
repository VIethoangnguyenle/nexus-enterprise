import { createFileRoute } from '@tanstack/react-router'
import { AssetsScreen } from '../../components/assets/AssetsScreen'
import { validateAssetsSearch } from '../../lib/assets-search'

// The tab, the filters, the page and the open asset, request or type are part of
// the URL, so reload, Back/Forward and links keep the place. `?ws=` is kept by
// the layout. Tài sản lives in the shared shell: one sign-in guard, one
// WebSocket, one workspace switcher, one logout.
export const Route = createFileRoute('/_workspace/assets')({
  validateSearch: validateAssetsSearch,
  component: AssetsScreen,
})
