import { apiFetch } from './client'

export interface Workspace { id: string; name: string; created_at?: string }

export interface WorkspaceMember { ngac_node_id: string; role: string; username?: string }

/** What Cài đặt → Workspace shows. `can_manage` says whether the caller may change it. */
export interface WorkspaceDetails { name: string; description: string; can_manage: boolean }

export const workspaceApi = {
  list: () => apiFetch<{ workspaces: Workspace[] }>('/workspaces'),
  get: (id: string) => apiFetch<Workspace>(`/workspaces/${id}`),
  /** The signed-in person leaves. 409 with `reason: 'last_owner'` when they are the only Owner. */
  leave: (id: string) => apiFetch<{ status: string }>(`/workspaces/${id}/leave`, { method: 'POST' }),
  details: (id: string) => apiFetch<WorkspaceDetails>(`/workspaces/${id}/details`),
  updateDetails: (id: string, body: { name?: string; description?: string }) =>
    apiFetch<WorkspaceDetails>(`/workspaces/${id}/details`, { method: 'PATCH', body: JSON.stringify(body) }),
  listMembers: (wsId: string) =>
    apiFetch<{ members: WorkspaceMember[] }>(`/workspaces/${wsId}/members`),
}
