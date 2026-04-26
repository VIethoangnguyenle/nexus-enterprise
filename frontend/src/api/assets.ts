import { apiFetch } from './client'

export interface AssetType { id: string; name: string; category: string; fields_schema?: object; lifecycle_config?: object }
export interface Asset { id: string; name: string; type_id: string; type_name?: string; state: string; assigned_to?: string; custom_fields?: object; created_at?: string }
export interface AssetRequest { id: string; type_id: string; type_name?: string; requester_id: string; requester_name?: string; status: string; justification?: string; urgency?: string; created_at?: string; reason?: string }
export interface Transition { action: string; to: string; ngac_permission?: string }

export const assetApi = {
  // Types
  listTypes: (wsId: string) => apiFetch<{ types: AssetType[] }>(`/workspaces/${wsId}/asset-types`),
  createType: (wsId: string, data: Partial<AssetType>) => apiFetch<AssetType>(`/workspaces/${wsId}/asset-types`, { method: 'POST', body: JSON.stringify(data) }),

  // Assets
  list: (wsId: string, params?: Record<string, string>) => {
    const qs = params ? '?' + new URLSearchParams(params).toString() : ''
    return apiFetch<{ assets: Asset[]; total: number }>(`/workspaces/${wsId}/assets${qs}`)
  },
  get: (id: string) => apiFetch<Asset>(`/assets/${id}`),
  create: (wsId: string, data: Partial<Asset>) => apiFetch<Asset>(`/workspaces/${wsId}/assets`, { method: 'POST', body: JSON.stringify(data) }),
  transition: (id: string, action: string, comment?: string) => apiFetch(`/assets/${id}/transition`, { method: 'POST', body: JSON.stringify({ action, comment }) }),
  getTransitions: (id: string) => apiFetch<{ transitions: Transition[] }>(`/assets/${id}/transitions`),
  getHistory: (id: string) => apiFetch<{ history: any[] }>(`/assets/${id}/history`),
  getSummary: (wsId: string) => apiFetch<any>(`/workspaces/${wsId}/assets/summary`),

  // Requests
  listRequests: (wsId: string, params?: Record<string, string>) => {
    const qs = params ? '?' + new URLSearchParams(params).toString() : ''
    return apiFetch<{ requests: AssetRequest[]; total: number }>(`/workspaces/${wsId}/asset-requests${qs}`)
  },
  createRequest: (wsId: string, data: object) => apiFetch<AssetRequest>(`/workspaces/${wsId}/asset-requests`, { method: 'POST', body: JSON.stringify(data) }),
  approveRequest: (id: string) => apiFetch(`/asset-requests/${id}/approve`, { method: 'POST' }),
  rejectRequest: (id: string, reason: string) => apiFetch(`/asset-requests/${id}/reject`, { method: 'POST', body: JSON.stringify({ reason }) }),
}
