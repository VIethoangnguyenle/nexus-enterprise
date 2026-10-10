import { keepPreviousData, queryOptions, useMutation, useQuery } from '@tanstack/react-query'
import {
  assetApi,
  type ApproveRequestInput,
  type CreateAssetInput,
  type CreateAssetRequestInput,
  type CreateAssetTypeInput,
  type ListAssetsParams,
  type ListRequestsParams,
} from '../api/assets'
import { statusOf } from '../lib/errors'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'

// --- Query options ---

export const assetTypesQueryOptions = (wsId: string) =>
  queryOptions({
    queryKey: keys.assets.types(wsId),
    queryFn: () => assetApi.listTypes(wsId),
    enabled: !!wsId,
    select: (d) => ({ types: d.types ?? [], canManage: d.can_manage === true }),
  })

export const assetsQueryOptions = (wsId: string, params?: ListAssetsParams) =>
  queryOptions({
    queryKey: keys.assets.list(wsId, params),
    queryFn: () => assetApi.list(wsId, params),
    enabled: !!wsId,
    select: (d) => ({ assets: d.assets ?? [], total: d.total ?? 0 }),
  })

export const assetQueryOptions = (id: string) =>
  queryOptions({
    queryKey: keys.assets.asset(id),
    queryFn: () => assetApi.get(id),
    enabled: !!id,
    // 403 and 404 are answers ("not yours", "gone"), not hiccups.
    retry: (count, err) => ![403, 404].includes(statusOf(err) ?? 0) && count < 1,
  })

export const assetSummaryQueryOptions = (wsId: string) =>
  queryOptions({
    queryKey: keys.assets.summary(wsId),
    queryFn: () => assetApi.getSummary(wsId),
    enabled: !!wsId,
    select: (d) => ({
      total: d.total ?? 0,
      byState: d.by_state ?? {},
      byType: (d.by_type ?? []).map((t) => ({ typeId: t.type_id, name: t.type_name, count: t.count ?? 0 })),
      holders: d.holders ?? 0,
      maintenanceOverdue: d.maintenance_overdue ?? 0,
    }),
  })

export const assetActivityQueryOptions = (wsId: string, limit = 10) =>
  queryOptions({
    queryKey: keys.assets.activity(wsId, limit),
    queryFn: () => assetApi.getActivity(wsId, limit),
    enabled: !!wsId,
    select: (d) => d.entries ?? [],
  })

export const assetRequestsQueryOptions = (wsId: string, params?: ListRequestsParams) =>
  queryOptions({
    queryKey: keys.assets.requestList(wsId, params),
    queryFn: () => assetApi.listRequests(wsId, params),
    enabled: !!wsId,
    select: (d) => ({ requests: d.requests ?? [], total: d.total ?? 0 }),
  })

export const assetRequestQueryOptions = (id: string) =>
  queryOptions({
    queryKey: keys.assets.request(id),
    queryFn: () => assetApi.getRequest(id),
    enabled: !!id,
    retry: (count, err) => ![403, 404].includes(statusOf(err) ?? 0) && count < 1,
  })

export const assetTransitionsQueryOptions = (id: string) =>
  queryOptions({
    queryKey: keys.assets.transitions(id),
    queryFn: () => assetApi.getTransitions(id),
    enabled: !!id,
    select: (d) => ({ transitions: d.transitions ?? [], canAssign: d.can_assign === true }),
  })

export const assetHistoryQueryOptions = (id: string) =>
  queryOptions({
    queryKey: keys.assets.history(id),
    queryFn: () => assetApi.getHistory(id),
    enabled: !!id,
    // Newest first, which is how a history is read.
    select: (d) => [...(d.records ?? [])].reverse(),
    retry: (count, err) => statusOf(err) !== 403 && count < 1,
  })

// --- Query hooks ---

export const useAssetTypes = (wsId: string) => useQuery(assetTypesQueryOptions(wsId))

/** A page of the list. The previous page stays on screen while the next loads. */
export const useAssets = (wsId: string, params?: ListAssetsParams, enabled = true) =>
  useQuery({ ...assetsQueryOptions(wsId, params), enabled: enabled && !!wsId, placeholderData: keepPreviousData })

export const useAsset = (id: string) => useQuery(assetQueryOptions(id))
export const useAssetSummary = (wsId: string, enabled = true) =>
  useQuery({ ...assetSummaryQueryOptions(wsId), enabled: enabled && !!wsId })
export const useAssetActivity = (wsId: string, limit = 10, enabled = true) =>
  useQuery({ ...assetActivityQueryOptions(wsId, limit), enabled: enabled && !!wsId })
export const useAssetRequests = (wsId: string, params?: ListRequestsParams, enabled = true) =>
  useQuery({ ...assetRequestsQueryOptions(wsId, params), enabled: enabled && !!wsId, placeholderData: keepPreviousData })
export const useAssetRequest = (id: string) => useQuery(assetRequestQueryOptions(id))
export const useAssetTransitions = (id: string) => useQuery(assetTransitionsQueryOptions(id))
export const useAssetHistory = (id: string) => useQuery(assetHistoryQueryOptions(id))

// --- Mutations ---

/** What a change to one asset makes stale: the asset, its lists, counts, history and the feed. */
function invalidateAssetData(assetId?: string) {
  if (assetId) {
    queryClient.invalidateQueries({ queryKey: keys.assets.asset(assetId) })
    queryClient.invalidateQueries({ queryKey: keys.assets.history(assetId) })
    queryClient.invalidateQueries({ queryKey: keys.assets.transitions(assetId) })
  }
  queryClient.invalidateQueries({ queryKey: keys.assets.listsAll() })
  queryClient.invalidateQueries({ queryKey: keys.assets.summaries() })
  queryClient.invalidateQueries({ queryKey: keys.assets.activitiesAll() })
  queryClient.invalidateQueries({ queryKey: keys.assets.typesAll() }) // available counts
}

function invalidateRequestData(requestId?: string) {
  queryClient.invalidateQueries({ queryKey: keys.assets.requestsAll() })
  if (requestId) queryClient.invalidateQueries({ queryKey: keys.assets.request(requestId) })
}

/** The dialog that sends this explains a failure on the spot, so the shared toast stays quiet. */
export function useCreateAssetType(wsId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (data: CreateAssetTypeInput) => assetApi.createType(wsId, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.assets.types(wsId) }),
  })
}

export function useUpdateTypeSchema(wsId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: ({ typeId, schema }: { typeId: string; schema: object }) => assetApi.updateTypeSchema(typeId, schema),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.assets.types(wsId) }),
  })
}

export function useCreateAsset(wsId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (data: CreateAssetInput) => assetApi.create(wsId, data),
    onSuccess: () => invalidateAssetData(),
  })
}

export function useTransitionAsset() {
  return useMutation({
    meta: { silentError: true }, // the panel says why, in words that fit the refusal
    mutationFn: ({ id, action, comment }: { id: string; action: string; comment?: string }) =>
      assetApi.transition(id, action, comment),
    onSuccess: (_, vars) => invalidateAssetData(vars.id),
  })
}

export function useHandOverAsset() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: ({ id, assigneeId, comment }: { id: string; assigneeId: string; comment?: string }) =>
      assetApi.handOver(id, assigneeId, comment),
    onSuccess: (_, vars) => invalidateAssetData(vars.id),
  })
}

export function useCreateAssetRequest(wsId: string) {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (data: CreateAssetRequestInput) => assetApi.createRequest(wsId, data),
    onSuccess: () => invalidateRequestData(),
  })
}

/** Approve, optionally handing the chosen asset over in the same step. The dialog says why on failure. */
export function useApproveRequest() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: ({ id, ...data }: { id: string } & ApproveRequestInput) => assetApi.approveRequest(id, data),
    onSuccess: (_, vars) => {
      invalidateRequestData(vars.id)
      if (vars.asset_id) invalidateAssetData(vars.asset_id)
    },
  })
}

export function useRejectRequest() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: ({ id, reason }: { id: string; reason: string }) => assetApi.rejectRequest(id, reason),
    onSuccess: (_, vars) => invalidateRequestData(vars.id),
  })
}

/** Give an asset to a request that was approved without one. */
export function useAssignRequest() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: ({ id, assetId }: { id: string; assetId: string }) => assetApi.assignRequest(id, assetId),
    onSuccess: (_, vars) => {
      invalidateRequestData(vars.id)
      invalidateAssetData(vars.assetId)
    },
  })
}
