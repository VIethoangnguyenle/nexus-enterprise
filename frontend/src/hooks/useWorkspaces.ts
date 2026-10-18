import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { workspaceApi, type WorkspaceLocks } from '../api/workspaces'
import { queryClient } from '../lib/query-client'
import { noteSelfLeave } from '../lib/self-leave'
import { keys } from './keys'

export const workspacesQueryOptions = () =>
  queryOptions({ queryKey: keys.workspaces.all(), queryFn: () => workspaceApi.list() })

export function useWorkspaces() {
  return useQuery(workspacesQueryOptions())
}

export function useWorkspaceDetails(wsId: string) {
  return useQuery({
    queryKey: keys.workspaces.details(wsId),
    queryFn: () => workspaceApi.details(wsId),
    enabled: !!wsId,
  })
}

/** Renames or re-describes a workspace; the switcher and the sidebar read the list, so it is refreshed too. */
export function useUpdateWorkspaceDetails(wsId: string) {
  return useMutation({
    meta: { action: 'lưu thông tin workspace' },
    mutationFn: (body: { name?: string; description?: string }) => workspaceApi.updateDetails(wsId, body),
    onSuccess: (details) => {
      queryClient.setQueryData(keys.workspaces.details(wsId), details)
      void queryClient.invalidateQueries({ queryKey: keys.workspaces.all() })
    },
  })
}

/**
 * Leaves a workspace. The caller words the failures (the last Owner is told what
 * to do instead; anything else gets the shared sentence), so no toast is added
 * here. What the person could see of this workspace is dropped from the cache.
 */
export function useLeaveWorkspace(wsId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: () => {
      // The session is moved and the sockets close as for a removal; this is not one.
      noteSelfLeave(wsId)
      return workspaceApi.leave(wsId)
    },
    onSuccess: async () => {
      queryClient.removeQueries({ predicate: (q) => JSON.stringify(q.queryKey).includes(wsId) })
      await queryClient.invalidateQueries({ queryKey: keys.workspaces.all() })
    },
  })
}

/** The workspace's admin-controlled rules (group creation lock). */
export function useWorkspaceLocks(wsId: string) {
  return useQuery({
    queryKey: [...keys.workspaces.details(wsId), 'locks'] as const,
    queryFn: () => workspaceApi.getLocks(wsId),
    enabled: !!wsId,
  })
}


/**
 * Sets "Ai được tạo nhóm" (organization only, administrators only). Locking takes the right away from Members and
 * custom roles on the server, so the roles screen is read again too. A refusal is put back by the server's answer
 * and told by the shared handler.
 */
export function useSetGroupCreation(wsId: string) {
  return useMutation({
    meta: { action: 'đổi quy định tạo nhóm' },
    mutationFn: (group_creation: WorkspaceLocks['group_creation']) => workspaceApi.putLocks(wsId, { group_creation }),
    onSuccess: (locks) => {
      queryClient.setQueryData([...keys.workspaces.details(wsId), 'locks'], locks)
      void queryClient.invalidateQueries({ queryKey: keys.admin.roles(wsId) })
    },
  })
}
