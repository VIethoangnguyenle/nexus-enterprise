import { createFileRoute } from '@tanstack/react-router'
import { CreateWorkspaceScreen } from '../../components/auth/CreateWorkspaceScreen'
import { requireSignedIn } from '../../lib/auth-guards'

/** Create a workspace: a name, nothing else. */
export const Route = createFileRoute('/_auth/onboarding')({
  beforeLoad: requireSignedIn,
  component: CreateWorkspaceScreen,
})
