import { apiFetch } from './client'

export interface AssetType { id: string; name: string; category: string; fields_schema?: object; lifecycle_config?: object }
export interface Asset { id: string; name: string; type_id: string; type_name?: string; state: string; assigned_to?: string; custom_fields?: object; created_at?: string }
export interface AssetRequest { id: string; type_id: string; type_name?: string; requester_id: string; requester_name?: string; status: string; justification?: string; urgency?: string; created_at?: string; reason?: string }
export interface Transition { action: string; to: string; ngac_permission?: string }

export interface AssetHistory {
  action: string
  from_state: string
  to_state: string
  actor_id?: string
  comment?: string
  created_at?: string
}

export interface AssetSummary {
  total: number
  in_use: number
  pending: number
  by_state?: Record<string, number>
}

export interface CreateAssetTypeInput { name: string; category: string; fields_schema?: object; lifecycle_config?: object }
export interface CreateAssetInput { name: string; type_id: string; custom_fields?: object }
export interface CreateAssetRequestInput { type_id: string; justification?: string; urgency?: string }

export const assetApi = {
  // Types
  listTypes: (wsId: string) => apiFetch<{ types: AssetType[] }>(`/workspaces/${wsId}/asset-types`),
  createType: (wsId: string, data: CreateAssetTypeInput) => apiFetch<AssetType>(`/workspaces/${wsId}/asset-types`, { method: 'POST', body: JSON.stringify(data) }),

  // Assets
  list: (wsId: string, params?: Record<string, string>) => {
    const qs = params ? '?' + new URLSearchParams(params).toString() : ''
    return apiFetch<{ assets: Asset[]; total: number }>(`/workspaces/${wsId}/assets${qs}`)
  },
  get: (id: string) => apiFetch<Asset>(`/assets/${id}`),
  create: (wsId: string, data: CreateAssetInput) => apiFetch<Asset>(`/workspaces/${wsId}/assets`, { method: 'POST', body: JSON.stringify(data) }),
  transition: (id: string, action: string, comment?: string) => apiFetch(`/assets/${id}/transition`, { method: 'POST', body: JSON.stringify({ action, comment }) }),
  getTransitions: (id: string) => apiFetch<{ transitions: Transition[] }>(`/assets/${id}/transitions`),
  getHistory: (id: string) => apiFetch<{ history: AssetHistory[] }>(`/assets/${id}/history`),
  getSummary: (wsId: string) => apiFetch<AssetSummary>(`/workspaces/${wsId}/assets/summary`),

  // Requests
  listRequests: (wsId: string, params?: Record<string, string>) => {
    const qs = params ? '?' + new URLSearchParams(params).toString() : ''
    return apiFetch<{ requests: AssetRequest[]; total: number }>(`/workspaces/${wsId}/asset-requests${qs}`)
  },
  createRequest: (wsId: string, data: CreateAssetRequestInput) => apiFetch<AssetRequest>(`/workspaces/${wsId}/asset-requests`, { method: 'POST', body: JSON.stringify(data) }),
  approveRequest: (id: string) => apiFetch(`/asset-requests/${id}/approve`, { method: 'POST' }),
  rejectRequest: (id: string, reason: string) => apiFetch(`/asset-requests/${id}/reject`, { method: 'POST', body: JSON.stringify({ reason }) }),
}
