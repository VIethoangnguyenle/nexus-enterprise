import { apiFetch } from './client'

export interface LoginPayload { username: string; password: string }
export interface RegisterPayload { username: string; password: string }
export interface AuthResponse { token: string; user: { id: string; username: string; ngac_node_id?: string } }

export interface OTPRequestPayload { identifier: string; type: 'phone' | 'email' }
export interface OTPRequestResponse { session_id: string; expires_in: number }
export interface OTPVerifyPayload { session_id: string; code: string }
export interface OTPVerifyResponse {
  token: string
  user: { id: string; username: string; ngac_node_id: string; email: string; phone: string; union_id: string }
  is_new_user: boolean
}

export const authApi = {
  login: (data: LoginPayload) => apiFetch<AuthResponse>('/auth/login', { method: 'POST', body: JSON.stringify(data) }),
  register: (data: RegisterPayload) => apiFetch<AuthResponse>('/auth/register', { method: 'POST', body: JSON.stringify(data) }),
  lookupUser: (username: string) => apiFetch<{ id: string; username: string; ngac_node_id: string }>(`/users/lookup?username=${encodeURIComponent(username)}`),

  // OTP flow
  requestOTP: (data: OTPRequestPayload) => apiFetch<OTPRequestResponse>('/auth/otp/request', { method: 'POST', body: JSON.stringify(data) }),
  verifyOTP: (data: OTPVerifyPayload) => apiFetch<OTPVerifyResponse>('/auth/otp/verify', { method: 'POST', body: JSON.stringify(data) }),
}
