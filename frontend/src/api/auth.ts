import { apiFetch, refreshAccessToken } from './client'
import { useAuthStore } from '../stores/auth.store'

export interface LoginPayload { username: string; password: string }
export interface RegisterPayload { username: string; password: string }
export interface AuthResponse { access_token: string; user: { id: string; username: string; ngac_node_id?: string } }

export interface OTPRequestPayload { identifier: string; type: 'phone' | 'email' }
export interface OTPRequestResponse { session_id: string; expires_in: number }
export interface OTPVerifyPayload { session_id: string; code: string }
export interface OTPVerifyResponse {
  access_token: string
  user: { id: string; username: string; ngac_node_id: string; email: string; phone: string; union_id: string }
  is_new_user: boolean
}

/** Sign-in methods the server has configured. */
export interface AuthProviders { google: boolean }

export interface MeResponse {
  user: { id: string; username: string; ngac_node_id: string; email: string; union_id: string; display_name: string }
  current_tenant?: { id: string; name: string; role: string; open_id: string }
}

export const authApi = {
  login: (data: LoginPayload) => apiFetch<AuthResponse>('/auth/login', { method: 'POST', body: JSON.stringify(data) }),
  register: (data: RegisterPayload) => apiFetch<AuthResponse>('/auth/register', { method: 'POST', body: JSON.stringify(data) }),
  lookupUser: (username: string) => apiFetch<{ id: string; username: string; ngac_node_id: string }>(`/users/lookup?username=${encodeURIComponent(username)}`),

  // OTP flow
  requestOTP: (data: OTPRequestPayload) => apiFetch<OTPRequestResponse>('/auth/otp/request', { method: 'POST', body: JSON.stringify(data) }),
  verifyOTP: (data: OTPVerifyPayload) => apiFetch<OTPVerifyResponse>('/auth/otp/verify', { method: 'POST', body: JSON.stringify(data) }),

  providers: () => apiFetch<AuthProviders>('/auth/providers'),
  me: () => apiFetch<MeResponse>('/me'),
}

/**
 * Server endpoint that starts "Sign in with Google". The whole OAuth flow runs
 * on the server, so this is a full-page navigation rather than a fetch.
 */
export const GOOGLE_SIGN_IN_PATH = '/api/auth/google/start'

/** Leaves the SPA for Google's sign-in page. */
export function startGoogleSignIn(): void {
  window.location.assign(GOOGLE_SIGN_IN_PATH)
}

/**
 * Finishes a sign-in that happened outside the SPA (Google redirects back to
 * /auth/google/done).
 *
 * The server set the httpOnly refresh cookie and deliberately put no token in
 * the URL, so the access token comes from the same refresh call that restores
 * a session on page load. Resolves to false when no session could be
 * established.
 */
export async function completeGoogleSignIn(): Promise<boolean> {
  const token = await refreshAccessToken()
  if (!token) return false
  try {
    const me = await authApi.me()
    // apiFetch may itself have refreshed; use whatever token is current now.
    useAuthStore.getState().login(useAuthStore.getState().accessToken ?? token, me.user)
    return true
  } catch {
    useAuthStore.getState().logout()
    return false
  }
}
