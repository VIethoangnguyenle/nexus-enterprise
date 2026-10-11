import { useMemo } from 'react'
import { useActiveWorkspace } from './useActiveWorkspace'
import type { WorkspaceType } from '../api/workspaces'

/**
 * The active workspace's type. A personal workspace (made for one person when
 * they first signed in) offers fewer areas than an organization.
 *
 * Until the workspace is known, or when the server sends a type this build does
 * not know, `type` is undefined and `isPersonal` is false: the client never
 * guesses. The server enforces the difference regardless; this only decides
 * what the screen offers.
 */
export function useWorkspaceType(): { type: WorkspaceType | undefined; isPersonal: boolean; isLoading: boolean } {
  const { workspaceId, workspaces, isLoading } = useActiveWorkspace()
  return useMemo(() => {
    const raw = workspaces.find((w) => w.id === workspaceId)?.type
    const type = raw === 'personal' || raw === 'organization' ? raw : undefined
    return { type, isPersonal: type === 'personal', isLoading }
  }, [workspaceId, workspaces, isLoading])
}
