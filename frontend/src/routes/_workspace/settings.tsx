import { createFileRoute, Outlet } from '@tanstack/react-router'
import { validateSettingsSearch } from '../../lib/settings-search'

// The workspace and the open tab are part of the URL: /settings is Hồ sơ on a
// wide screen and Tôi on a phone; ?tab=workspace and ?tab=giao-dien are the
// other tabs. /settings/<section> is one settings page (Tài khoản, a module…).
export const Route = createFileRoute('/_workspace/settings')({
  validateSearch: validateSettingsSearch,
  component: Outlet,
})
