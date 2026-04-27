import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { documentApi } from '../api/documents'
import { queryClient } from '../lib/query-client'

export const documentsQueryOptions = (wsId: string) =>
  queryOptions({ queryKey: ['documents', wsId], queryFn: () => documentApi.list(wsId), enabled: !!wsId })

export const documentQueryOptions = (id: string) =>
  queryOptions({ queryKey: ['document', id], queryFn: () => documentApi.get(id), enabled: !!id })

export function useDocuments(wsId: string) { return useQuery(documentsQueryOptions(wsId)) }
export function useDocument(id: string) { return useQuery(documentQueryOptions(id)) }

export function useDeleteDocument(wsId: string) {
  return useMutation({
    mutationFn: (id: string) => documentApi.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['documents', wsId] }),
  })
}

export function useApproveDocument(wsId: string) {
  return useMutation({
    mutationFn: (id: string) => documentApi.approve(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['documents', wsId] }),
  })
}
