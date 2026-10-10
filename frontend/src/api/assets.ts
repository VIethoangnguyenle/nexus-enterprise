import { apiFetch } from './client'

// --- Types matching backend REST responses (proto JSON; empty fields are omitted) ---

/** The lifecycle the backend runs (`domain/lifecycle.go`). The screens use no other set. */
export type AssetState = 'requested' | 'available' | 'assigned' | 'maintenance' | 'retired' | 'disposed'

export type Urgency = 'low' | 'normal' | 'high' | 'urgent'

export type RequestStatus = 'pending' | 'approved' | 'rejected' | 'fulfilled'

export interface AssetType {
  id: string
  name: string
  description?: string
  /** A key (`hardware`…); shown through `categoryLabel`, never raw. */
  category: string
  /** JSON Schema of the custom fields, as a string. `{}` when the type has none. */
  fields_schema?: string
  asset_count?: number
  available_count?: number
  /** Operations (`read`, `write`, `approve`, `manage`) the caller holds on this type. */
  permissions?: string[]
  lifecycle?: {
    states?: string[]
    initial_state?: string
    transitions?: { from_state: string; to_state: string; operation: string; ngac_permission: string }[]
  }
}

export interface AssetTypeList {
  types?: AssetType[]
  /** The caller may define types and edit their fields. */
  can_manage?: boolean
}

export interface Asset {
  id: string
  name: string
  type_id: string
  type_name?: string
  state: AssetState | string
  custom_fields?: Record<string, unknown>
  /** The holder; resolved to a name through `assigned_to_name` or the people directory. */
  assigned_to_user_id?: string
  assigned_to_name?: string
  created_at?: unknown
  updated_at?: unknown
}

export interface AssetList {
  assets?: Asset[]
  total?: number
}

export interface AssetRequest {
  id: string
  type_id: string
  type_name?: string
  requester_id: string
  requester_name?: string
  status: RequestStatus | string
  justification?: string
  quantity?: number
  urgency?: Urgency | string
  assigned_asset_id?: string
  assigned_asset_name?: string
  approver_id?: string
  approver_name?: string
  approver_comment?: string
  /** The caller may approve or reject it now. */
  can_decide?: boolean
  /** The caller may give it an asset. */
  can_assign?: boolean
  created_at?: unknown
  updated_at?: unknown
}

export interface AssetRequestList {
  requests?: AssetRequest[]
  total?: number
}

export interface Transition {
  action: string
  to_state: string
  ngac_permission?: string
}

export interface TransitionList {
  transitions?: Transition[]
  current_state?: string
  /** The caller may hand the asset to someone (it is available or assigned, and they may manage it). */
  can_assign?: boolean
}

export interface HistoryRecord {
  id: string
  action: string
  from_state: string
  to_state: string
  actor_id?: string
  actor_name?: string
  /** The person the step concerned: the new holder, or the previous one on a return. */
  subject_user_id?: string
  subject_name?: string
  comment?: string
  created_at?: unknown
}

export interface HistoryList {
  records?: HistoryRecord[]
}

export interface TypeCount {
  type_id: string
  type_name: string
  count?: number
}

export interface AssetSummary {
  total?: number
  by_state?: Record<string, number>
  by_type?: TypeCount[]
  holders?: number
  maintenance_overdue?: number
}

export interface ActivityEntry extends HistoryRecord {
  asset_id: string
  /** For a decision on a request, the type asked for. */
  asset_name: string
  type_name?: string
  /** `approved` or `rejected` when this is a decision on a request, not a step on an asset. */
  request_status?: string
}

export interface ActivityList {
  entries?: ActivityEntry[]
}

// --- Inputs ---

export interface ListAssetsParams {
  type_id?: string
  state?: string
  search?: string
  assigned_to?: string
  limit?: number
  offset?: number
}

export interface ListRequestsParams {
  /** One status, or several separated by commas. */
  status?: string
  mine?: boolean
  limit?: number
  offset?: number
}

export interface CreateAssetTypeInput { name: string; category: string }
export interface CreateAssetInput { type_id: string; name: string; custom_fields?: Record<string, unknown> }
export interface CreateAssetRequestInput { type_id: string; reason: string; urgency: Urgency }
export interface ApproveRequestInput { asset_id?: string; comment?: string }

/** Query string from the parameters that are set; none set gives none. */
function query(params: object | undefined): string {
  const qs = new URLSearchParams()
  for (const [k, v] of Object.entries(params ?? {})) {
    if (v === undefined || v === null || v === '' || v === false) continue
    qs.set(k, String(v))
  }
  const s = qs.toString()
  return s ? `?${s}` : ''
}

export const assetApi = {
  // Types
  listTypes: (wsId: string) => apiFetch<AssetTypeList>(`/workspaces/${wsId}/asset-types`),
  createType: (wsId: string, data: CreateAssetTypeInput) =>
    apiFetch<AssetType>(`/workspaces/${wsId}/asset-types`, { method: 'POST', body: JSON.stringify(data) }),
  /** The handler binds `fields_schema` as a string, so the schema object is stringified here. */
  updateTypeSchema: (typeId: string, schema: object) =>
    apiFetch<AssetType>(`/asset-types/${typeId}/schema`, { method: 'PUT', body: JSON.stringify({ fields_schema: JSON.stringify(schema) }) }),

  // Assets
  list: (wsId: string, params?: ListAssetsParams) =>
    apiFetch<AssetList>(`/workspaces/${wsId}/assets${query(params)}`),
  get: (id: string) => apiFetch<Asset>(`/assets/${id}`),
  create: (wsId: string, data: CreateAssetInput) =>
    apiFetch<Asset>(`/workspaces/${wsId}/assets`, { method: 'POST', body: JSON.stringify(data) }),
  getSummary: (wsId: string) => apiFetch<AssetSummary>(`/workspaces/${wsId}/assets/summary`),
  getActivity: (wsId: string, limit?: number) =>
    apiFetch<ActivityList>(`/workspaces/${wsId}/assets/activity${query({ limit })}`),

  // Lifecycle
  getTransitions: (id: string) => apiFetch<TransitionList>(`/assets/${id}/transitions`),
  getHistory: (id: string) => apiFetch<HistoryList>(`/assets/${id}/history`),
  transition: (id: string, action: string, comment?: string) =>
    apiFetch<Asset>(`/assets/${id}/transition`, { method: 'POST', body: JSON.stringify({ action, ...(comment ? { comment } : {}) }) }),
  handOver: (id: string, assigneeId: string, comment?: string) =>
    apiFetch<Asset>(`/assets/${id}/assign`, { method: 'POST', body: JSON.stringify({ assignee_id: assigneeId, ...(comment ? { comment } : {}) }) }),

  // Requests
  listRequests: (wsId: string, params?: ListRequestsParams) =>
    apiFetch<AssetRequestList>(`/workspaces/${wsId}/asset-requests${query(params)}`),
  getRequest: (id: string) => apiFetch<AssetRequest>(`/asset-requests/${id}`),
  createRequest: (wsId: string, data: CreateAssetRequestInput) =>
    apiFetch<AssetRequest>(`/workspaces/${wsId}/asset-requests`, { method: 'POST', body: JSON.stringify(data) }),
  approveRequest: (id: string, data: ApproveRequestInput) =>
    apiFetch<AssetRequest>(`/asset-requests/${id}/approve`, { method: 'POST', body: JSON.stringify(data) }),
  rejectRequest: (id: string, reason: string) =>
    apiFetch<AssetRequest>(`/asset-requests/${id}/reject`, { method: 'POST', body: JSON.stringify({ reason }) }),
  assignRequest: (id: string, assetId: string) =>
    apiFetch<AssetRequest>(`/asset-requests/${id}/assign`, { method: 'POST', body: JSON.stringify({ asset_id: assetId }) }),
}
