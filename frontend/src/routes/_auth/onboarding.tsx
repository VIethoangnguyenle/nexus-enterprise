import { createFileRoute } from '@tanstack/react-router'
import { CreateWorkspaceScreen } from '../../components/auth/CreateWorkspaceScreen'
import { requireSignedIn } from '../../lib/auth-guards'
import { returnPath } from '../../lib/auth-flow'

/** Create a workspace: a type and a name. `?from=` is where Back returns to. */
export const Route = createFileRoute('/_auth/onboarding')({
  validateSearch: (search: Record<string, unknown>): { from?: string } => {
    const from = returnPath(search.from)
    return from ? { from } : {}
  },
  beforeLoad: requireSignedIn,
  component: CreateWorkspaceScreen,
})
