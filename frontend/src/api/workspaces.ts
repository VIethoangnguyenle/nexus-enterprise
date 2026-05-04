import { apiFetch } from './client'

export interface Workspace { id: string; name: string; created_at?: string }

export interface WorkspaceMember { ngac_node_id: string; role: string; username?: string }

export const workspaceApi = {
  list: () => apiFetch<{ workspaces: Workspace[] }>('/workspaces'),
  create: (name: string) => apiFetch<Workspace>('/workspaces', { method: 'POST', body: JSON.stringify({ name }) }),
  get: (id: string) => apiFetch<Workspace>(`/workspaces/${id}`),
  inviteMember: (wsId: string, ngacNodeId: string) =>
    apiFetch<{ status: string }>(`/workspaces/${wsId}/invite`, { method: 'POST', body: JSON.stringify({ ngac_node_id: ngacNodeId }) }),
  listMembers: (wsId: string) =>
    apiFetch<{ members: WorkspaceMember[] }>(`/workspaces/${wsId}/members`),
}
