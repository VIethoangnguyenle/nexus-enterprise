import { createFileRoute } from '@tanstack/react-router'
import { ProfileScreen } from '../../components/auth/ProfileScreen'
import { requireSignedIn } from '../../lib/auth-guards'
import { validateWorkspaceSearch } from '../../lib/workspace'

/** The profile step of sign-in: reached after the code is verified, only by a person who owes a profile. */
export const Route = createFileRoute('/_auth/register')({
  validateSearch: validateWorkspaceSearch,
  beforeLoad: requireSignedIn,
  component: ProfileScreen,
})
