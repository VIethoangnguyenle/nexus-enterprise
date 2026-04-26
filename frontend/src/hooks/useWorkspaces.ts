import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { workspaceApi } from '../api/workspaces'
import { queryClient } from '../lib/query-client'

export const workspacesQueryOptions = () =>
  queryOptions({ queryKey: ['workspaces'], queryFn: () => workspaceApi.list() })

export function useWorkspaces() {
  return useQuery(workspacesQueryOptions())
}

export function useCreateWorkspace() {
  return useMutation({
    mutationFn: (name: string) => workspaceApi.create(name),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['workspaces'] }),
  })
}
