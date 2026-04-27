import { useQuery, useMutation, queryOptions } from '@tanstack/react-query'
import { assetApi, type CreateAssetTypeInput, type CreateAssetInput, type CreateAssetRequestInput } from '../api/assets'
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

export function useApproveRequest(wsId: string) {
  return useMutation({
    mutationFn: (id: string) => assetApi.approveRequest(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['asset-requests', wsId] }),
  })
}

export function useRejectRequest(wsId: string) {
  return useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) => assetApi.rejectRequest(id, reason),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['asset-requests', wsId] }),
  })
}
