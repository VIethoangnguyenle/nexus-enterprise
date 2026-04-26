import { apiFetch } from './client'
import { useAuthStore } from '../stores/auth.store'

export interface Document { id: string; name: string; state: string; owner_id?: string; created_at?: string }

export const documentApi = {
  list: (wsId: string) => apiFetch<{ documents: Document[] }>(`/workspaces/${wsId}/documents`),
  get: (id: string) => apiFetch<Document>(`/documents/${id}`),
  create: async (wsId: string, data: FormData) => {
    const token = useAuthStore.getState().token
    const res = await fetch(`/api/workspaces/${wsId}/documents`, {
      method: 'POST', body: data,
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    })
    return res.json()
  },
  delete: (id: string) => apiFetch(`/documents/${id}`, { method: 'DELETE' }),
  approve: (id: string) => apiFetch(`/documents/${id}/approve`, { method: 'POST' }),
  share: (id: string, data: object) => apiFetch(`/documents/${id}/share`, { method: 'POST', body: JSON.stringify(data) }),
}
