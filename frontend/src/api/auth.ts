import { apiFetch } from './client'

export interface LoginPayload { username: string; password: string }
export interface RegisterPayload { username: string; password: string }
export interface AuthResponse { token: string; user: { id: string; username: string; ngac_node_id?: string } }

export const authApi = {
  login: (data: LoginPayload) => apiFetch<AuthResponse>('/auth/login', { method: 'POST', body: JSON.stringify(data) }),
  register: (data: RegisterPayload) => apiFetch<AuthResponse>('/auth/register', { method: 'POST', body: JSON.stringify(data) }),
}
