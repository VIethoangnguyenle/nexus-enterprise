import { createFileRoute, Outlet } from '@tanstack/react-router'
import { validateAdminSearch } from '../../lib/admin-search'

// The workspace and what is open (a department, a person, a role, the
// permission editor) are part of the URL, so reload, Back/Forward and a pasted
// link land in the same place. Each screen reads its own keys. The sub-navigation
// is the tab bar each screen draws (components/admin/AdminFrame), inside the
// workspace shell that `_workspace.tsx` provides.
export const Route = createFileRoute('/_workspace/admin')({
  validateSearch: validateAdminSearch,
  component: Outlet,
})
