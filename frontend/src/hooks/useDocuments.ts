import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { documentApi, type Document } from '../api/documents'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

export const documentsQueryOptions = (wsId: string) =>
  queryOptions({ queryKey: keys.documents.list(wsId), queryFn: () => documentApi.list(wsId), enabled: !!wsId })

export const documentQueryOptions = (id: string) =>
  queryOptions({ queryKey: keys.documents.detail(id), queryFn: () => documentApi.get(id), enabled: !!id })

export function useDocuments(wsId: string) { return useQuery(documentsQueryOptions(wsId)) }
export function useDocument(id: string) { return useQuery(documentQueryOptions(id)) }

/** Delete a document with optimistic removal from list cache. */
export function useDeleteDocument(wsId: string) {
  return useMutation({
    mutationFn: (id: string) => documentApi.delete(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: keys.documents.list(wsId) })
      const prev = queryClient.getQueryData<{ documents: Document[] }>(keys.documents.list(wsId))
      if (prev) {
        queryClient.setQueryData(keys.documents.list(wsId), {
          ...prev,
          documents: prev.documents.filter((d) => d.id !== id),
        })
      }
      return { prev }
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) queryClient.setQueryData(keys.documents.list(wsId), context.prev)
    },
  })
}

/** Approve a document with optimistic status update. */
export function useApproveDocument(wsId: string) {
  return useMutation({
    mutationFn: (id: string) => documentApi.approve(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: keys.documents.list(wsId) })
      const prev = queryClient.getQueryData<{ documents: Document[] }>(keys.documents.list(wsId))
      if (prev) {
        queryClient.setQueryData(keys.documents.list(wsId), {
          ...prev,
          documents: prev.documents.map((d) =>
            d.id === id ? { ...d, status: 'approved' } : d,
          ),
        })
      }
      return { prev }
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) queryClient.setQueryData(keys.documents.list(wsId), context.prev)
    },
  })
}

