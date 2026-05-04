import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { assetApi, type CreateAssetTypeInput, type CreateAssetInput, type CreateAssetRequestInput, type AssetRequest } from '../api/assets'
import { queryClient } from '../lib/query-client'

export const assetTypesQueryOptions = (wsId: string) =>
  queryOptions({ queryKey: ['asset-types', wsId], queryFn: () => assetApi.listTypes(wsId), enabled: !!wsId })

export const assetsQueryOptions = (wsId: string, params?: Record<string, string>) =>
  queryOptions({ queryKey: ['assets', wsId, params], queryFn: () => assetApi.list(wsId, params), enabled: !!wsId })

export const assetQueryOptions = (id: string) =>
  queryOptions({ queryKey: ['asset', id], queryFn: () => assetApi.get(id), enabled: !!id })

export const assetSummaryQueryOptions = (wsId: string) =>
  queryOptions({ queryKey: ['asset-summary', wsId], queryFn: () => assetApi.getSummary(wsId), enabled: !!wsId })

export const assetRequestsQueryOptions = (wsId: string, params?: Record<string, string>) =>
  queryOptions({ queryKey: ['asset-requests', wsId, params], queryFn: () => assetApi.listRequests(wsId, params), enabled: !!wsId })

export const assetTransitionsQueryOptions = (id: string) =>
  queryOptions({ queryKey: ['asset-transitions', id], queryFn: () => assetApi.getTransitions(id), enabled: !!id })

export const assetHistoryQueryOptions = (id: string) =>
  queryOptions({ queryKey: ['asset-history', id], queryFn: () => assetApi.getHistory(id), enabled: !!id })

export function useAssetTypes(wsId: string) { return useQuery(assetTypesQueryOptions(wsId)) }
export function useAssets(wsId: string, params?: Record<string, string>) { return useQuery(assetsQueryOptions(wsId, params)) }
export function useAsset(id: string) { return useQuery(assetQueryOptions(id)) }
export function useAssetSummary(wsId: string) { return useQuery(assetSummaryQueryOptions(wsId)) }
export function useAssetRequests(wsId: string, params?: Record<string, string>) { return useQuery(assetRequestsQueryOptions(wsId, params)) }
export function useAssetTransitions(id: string) { return useQuery(assetTransitionsQueryOptions(id)) }
export function useAssetHistory(id: string) { return useQuery(assetHistoryQueryOptions(id)) }

export function useCreateAssetType(wsId: string) {
  return useMutation({
    mutationFn: (data: CreateAssetTypeInput) => assetApi.createType(wsId, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['asset-types', wsId] }),
  })
}

export function useCreateAsset(wsId: string) {
  return useMutation({
    mutationFn: (data: CreateAssetInput) => assetApi.create(wsId, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['assets', wsId] })
      queryClient.invalidateQueries({ queryKey: ['asset-summary', wsId] })
    },
  })
}

export function useTransitionAsset() {
  return useMutation({
    mutationFn: ({ id, action, comment }: { id: string; action: string; comment?: string }) =>
      assetApi.transition(id, action, comment),
    onSuccess: (_, vars) => {
      queryClient.invalidateQueries({ queryKey: ['asset', vars.id] })
      queryClient.invalidateQueries({ queryKey: ['assets'] })
      queryClient.invalidateQueries({ queryKey: ['asset-history', vars.id] })
      queryClient.invalidateQueries({ queryKey: ['asset-transitions', vars.id] })
      queryClient.invalidateQueries({ queryKey: ['asset-summary'] })
    },
  })
}

export function useCreateAssetRequest(wsId: string) {
  return useMutation({
    mutationFn: (data: CreateAssetRequestInput) => assetApi.createRequest(wsId, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['asset-requests', wsId] }),
  })
}

/** Approve an asset request with optimistic status update. */
export function useApproveRequest(wsId: string) {
  return useMutation({
    mutationFn: (id: string) => assetApi.approveRequest(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: ['asset-requests', wsId] })
      const cache = queryClient.getQueryCache()
      const queries = cache.findAll({ queryKey: ['asset-requests', wsId] })
      const snapshots: { key: unknown[]; data: unknown }[] = []
      for (const q of queries) {
        const data = q.state.data as { requests: AssetRequest[]; total: number } | undefined
        if (!data?.requests) continue
        snapshots.push({ key: q.queryKey, data })
        queryClient.setQueryData(q.queryKey, {
          ...data,
          requests: data.requests.map((r) =>
            r.id === id ? { ...r, status: 'approved' } : r,
          ),
        })
      }
      return { snapshots }
    },
    onError: (_err, _vars, context) => {
      if (context?.snapshots) {
        for (const s of context.snapshots) queryClient.setQueryData(s.key, s.data)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['asset-summary'] })
    },
  })
}

/** Reject an asset request with optimistic status update. */
export function useRejectRequest(wsId: string) {
  return useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) => assetApi.rejectRequest(id, reason),
    onMutate: async ({ id }) => {
      await queryClient.cancelQueries({ queryKey: ['asset-requests', wsId] })
      const cache = queryClient.getQueryCache()
      const queries = cache.findAll({ queryKey: ['asset-requests', wsId] })
      const snapshots: { key: unknown[]; data: unknown }[] = []
      for (const q of queries) {
        const data = q.state.data as { requests: AssetRequest[]; total: number } | undefined
        if (!data?.requests) continue
        snapshots.push({ key: q.queryKey, data })
        queryClient.setQueryData(q.queryKey, {
          ...data,
          requests: data.requests.map((r) =>
            r.id === id ? { ...r, status: 'rejected' } : r,
          ),
        })
      }
      return { snapshots }
    },
    onError: (_err, _vars, context) => {
      if (context?.snapshots) {
        for (const s of context.snapshots) queryClient.setQueryData(s.key, s.data)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['asset-summary'] })
    },
  })
}
