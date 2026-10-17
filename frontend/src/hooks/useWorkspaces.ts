import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { workspaceApi } from '../api/workspaces'
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

/** Converts a personal workspace to an organization workspace. */
export function useConvertToOrganization(wsId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: () => workspaceApi.convertToOrganization(wsId),
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: keys.workspaces.all() }),
        queryClient.invalidateQueries({ queryKey: keys.workspaces.details(wsId) }),
        queryClient.invalidateQueries({ queryKey: keys.auth.workspaces() }),
        queryClient.invalidateQueries({ queryKey: keys.auth.me() }),
        queryClient.invalidateQueries({ queryKey: keys.approval.all() }),
        queryClient.invalidateQueries({ queryKey: keys.assets.listsAll() }),
        queryClient.invalidateQueries({ queryKey: keys.assets.typesAll() }),
        queryClient.invalidateQueries({ queryKey: keys.assets.summaries() }),
        queryClient.invalidateQueries({ queryKey: keys.assets.requestsAll() }),
      ])
    },
  })
}
