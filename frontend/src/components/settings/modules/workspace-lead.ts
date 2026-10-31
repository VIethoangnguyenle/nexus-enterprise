import { useActiveWorkspace } from '../../../hooks/useActiveWorkspace'
import { useWorkspaceType } from '../../../hooks/useWorkspaceType'
import { workspaceDisplayName } from '../../../lib/workspace'

/**
 * The line under a module page's title on a wide screen, the heading its item sits under in the section list:
 * "Trong <workspace>", or "Trong workspace này" for a personal workspace, whose name is the person's own.
 */
export function useWorkspaceLead(): string {
  const { workspaceName } = useActiveWorkspace()
  const { isPersonal } = useWorkspaceType()
  return isPersonal ? 'Trong workspace này' : `Trong ${workspaceDisplayName(workspaceName)}`
}
