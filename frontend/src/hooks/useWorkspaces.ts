import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { workspaceApi } from '../api/workspaces'
import { queryClient } from '../lib/query-client'
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
    mutationFn: () => workspaceApi.leave(wsId),
    onSuccess: async () => {
      queryClient.removeQueries({ predicate: (q) => JSON.stringify(q.queryKey).includes(wsId) })
      await queryClient.invalidateQueries({ queryKey: keys.workspaces.all() })
    },
  })
}
