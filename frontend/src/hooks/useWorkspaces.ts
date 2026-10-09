import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { workspaceApi } from '../api/workspaces'
import { authApi } from '../api/auth'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

export const workspacesQueryOptions = () =>
  queryOptions({ queryKey: keys.workspaces.all(), queryFn: () => workspaceApi.list() })

export function useWorkspaces() {
  return useQuery(workspacesQueryOptions())
}

export function useCreateWorkspace() {
  return useMutation({
    mutationFn: (name: string) => workspaceApi.create(name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.workspaces.all() }),
  })
}

/** Invite a user by username to a workspace. Resolves username → ngac_node_id, then calls invite API. */
export function useInviteMember(wsId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: async (username: string) => {
      const user = await authApi.lookupUser(username)
      return workspaceApi.inviteMember(wsId, user.ngac_node_id)
    },
  })
}
