import { useMemo } from 'react'
import { useInfiniteQuery, useMutation, useQuery, type InfiniteData } from '@tanstack/react-query'
import { documentApi, type TextPage, type TextSave, type TextScope } from '../api/documents'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

/**
 * The workspace's text documents the caller may read, a page at a time. `docs`
 * is every page loaded so far, in order; `fetchNextPage` adds the next.
 */
export function useTextDocuments(wsId: string, scope: TextScope = 'all') {
  const q = useInfiniteQuery({
    queryKey: keys.documents.list(wsId, scope),
    queryFn: ({ pageParam }) => documentApi.list(wsId, scope, pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next,
    enabled: !!wsId,
  })
  const docs = useMemo(() => q.data?.pages.flatMap((p) => p.documents), [q.data])
  return { ...q, docs }
}

/** How many documents of a group the caller may read: for a badge, nothing is loaded. */
export function useTextCount(wsId: string, scope: TextScope) {
  return useQuery({
    queryKey: keys.documents.count(wsId, scope),
    queryFn: () => documentApi.count(wsId, scope),
    enabled: !!wsId,
  })
}

/**
 * One document, read fresh every time it is opened and kept nowhere after: the
 * editor takes the content once, and an older copy from a cache would be
 * mistaken for the latest.
 */
export function useTextDocument(id: string) {
  return useQuery({
    queryKey: keys.documents.detail(id),
    queryFn: () => documentApi.get(id),
    enabled: !!id,
    gcTime: 0,
    staleTime: 0,
    retry: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  })
}

export const invalidateTextLists = (wsId: string) =>
  queryClient.invalidateQueries({ queryKey: keys.documents.all(wsId) })

export function useCreateTextDocument(wsId: string) {
  return useMutation({
    meta: { action: 'tạo văn bản' },
    mutationFn: (body: { title?: string; folder_id?: string }) => documentApi.create(wsId, body),
    onSuccess: () => invalidateTextLists(wsId),
  })
}

export function useDeleteTextDocument(wsId: string) {
  return useMutation({
    meta: { action: 'xoá văn bản' },
    mutationFn: (id: string) => documentApi.remove(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: keys.documents.all(wsId) })
      const before = queryClient.getQueriesData<InfiniteData<TextPage>>({ queryKey: keys.documents.all(wsId) })
      queryClient.setQueriesData<InfiniteData<TextPage>>({ queryKey: keys.documents.all(wsId) }, (old) =>
        old && Array.isArray(old.pages)
          ? { ...old, pages: old.pages.map((p) => ({ ...p, documents: p.documents.filter((d) => d.id !== id) })) }
          : old,
      )
      return { before }
    },
    onError: (_err, _id, ctx) => {
      for (const [key, data] of ctx?.before ?? []) queryClient.setQueryData(key, data)
    },
    onSettled: () => invalidateTextLists(wsId),
  })
}

/**
 * Saves a document. The caller reports its failures itself (the editor shows
 * them next to the title), so no toast is added here.
 */
export function useSaveTextDocument(id: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (body: TextSave) => documentApi.save(id, body),
  })
}
