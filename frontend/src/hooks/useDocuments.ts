import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { documentApi, type Document } from '../api/documents'
import { queryClient } from '../lib/query-client'

export const documentsQueryOptions = (wsId: string) =>
  queryOptions({ queryKey: ['documents', wsId], queryFn: () => documentApi.list(wsId), enabled: !!wsId })

export const documentQueryOptions = (id: string) =>
  queryOptions({ queryKey: ['document', id], queryFn: () => documentApi.get(id), enabled: !!id })

export function useDocuments(wsId: string) { return useQuery(documentsQueryOptions(wsId)) }
export function useDocument(id: string) { return useQuery(documentQueryOptions(id)) }

/** Delete a document with optimistic removal from list cache. */
export function useDeleteDocument(wsId: string) {
  return useMutation({
    mutationFn: (id: string) => documentApi.delete(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: ['documents', wsId] })
      const prev = queryClient.getQueryData<{ documents: Document[] }>(['documents', wsId])
      if (prev) {
        queryClient.setQueryData(['documents', wsId], {
          ...prev,
          documents: prev.documents.filter((d) => d.id !== id),
        })
      }
      return { prev }
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) queryClient.setQueryData(['documents', wsId], context.prev)
    },
  })
}

/** Approve a document with optimistic status update. */
export function useApproveDocument(wsId: string) {
  return useMutation({
    mutationFn: (id: string) => documentApi.approve(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: ['documents', wsId] })
      const prev = queryClient.getQueryData<{ documents: Document[] }>(['documents', wsId])
      if (prev) {
        queryClient.setQueryData(['documents', wsId], {
          ...prev,
          documents: prev.documents.map((d) =>
            d.id === id ? { ...d, status: 'approved' } : d,
          ),
        })
      }
      return { prev }
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) queryClient.setQueryData(['documents', wsId], context.prev)
    },
  })
}

