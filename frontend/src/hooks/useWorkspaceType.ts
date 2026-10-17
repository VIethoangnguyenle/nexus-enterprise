import { useMemo } from 'react'
import { useActiveWorkspace } from './useActiveWorkspace'
import { useWorkspaces } from './useWorkspaces'
import type { WorkspaceType } from '../api/workspaces'

interface WorkspaceKind { type: WorkspaceType | undefined; isPersonal: boolean; isLoading: boolean }

/** The one place a workspace's raw type becomes `personal`, `organization` or unknown. */
function kindOf(workspaces: { id: string; type?: string }[], workspaceId: string, isLoading: boolean): WorkspaceKind {
  const raw = workspaces.find((w) => w.id === workspaceId)?.type
  const type = raw === 'personal' || raw === 'organization' ? raw : undefined
  return { type, isPersonal: type === 'personal', isLoading }
}

/**
 * The active workspace's type. A personal workspace (made for one person when
 * they first signed in) offers fewer areas than an organization, and no
 * departments: those exist only in company workspaces.
 *
 * Until the workspace is known, or when the server sends a type this build does
 * not know, `type` is undefined and `isPersonal` is false: the client never
 * guesses. The server enforces the difference regardless; this only decides
 * what the screen offers.
 */
export function useWorkspaceType(): WorkspaceKind {
  const { workspaceId, workspaces, isLoading } = useActiveWorkspace()
  return useMemo(() => kindOf(workspaces, workspaceId, isLoading), [workspaceId, workspaces, isLoading])
}

/**
 * The type of one workspace by id, for hooks that are given the id and have no
 * router (`usePeople`, which many screens use outside any route).
 */
export function useWorkspaceTypeOf(workspaceId: string): WorkspaceKind {
  const { data, isLoading } = useWorkspaces()
  return useMemo(() => kindOf(data?.workspaces ?? [], workspaceId, isLoading), [workspaceId, data, isLoading])
}
