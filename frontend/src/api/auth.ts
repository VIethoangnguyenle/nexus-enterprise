import { apiFetch, publicFetch, refreshAccessToken } from './client'
import { useAuthStore } from '../stores/auth.store'

/** What the sign-in endpoints say about the account that signed in. */
export interface SessionUser {
  id: string
  username: string
  ngac_node_id: string
  email: string
  phone?: string
  union_id: string
  /** The owner of `email` has proved it (Google, or a code delivered to it). */
  email_verified: boolean
}

// POST /api/auth/otp/request — body fields match the handler's struct exactly.
export interface OTPRequestPayload { identifier: string; type: 'phone' | 'email' }
export interface OTPRequestResponse { session_id: string; expires_in: number }

// POST /api/auth/otp/verify
export interface OTPVerifyPayload { session_id: string; code: string }
export interface OTPVerifyResponse {
  access_token: string
  user: SessionUser
  is_new_user: boolean
  /** The "what should we call you" step is still owed. */
  needs_profile: boolean
}

/** Sign-in methods the server has configured. */
export interface AuthProviders {
  google: boolean
  /** OTP sign-in can issue codes. */
  otp?: boolean
  /**
   * The test-only fixed OTP code is in force. For operators and the API: no
   * screen prints the code.
   */
  otp_fixed_code?: boolean
  /** A code delivered by this server reaches only the address's owner, so entering it verifies the address. */
  otp_proves_email?: boolean
}

export interface MeResponse {
  user: {
    id: string
    username: string
    ngac_node_id: string
    email: string
    union_id: string
    display_name: string
    title: string
    location: string
    avatar_url: string
    email_verified: boolean
    needs_profile: boolean
  }
  /**
   * The workspace asked about (the token's, or `?workspace=`), present only for
   * one the caller is an active member of. `department` is the one an
   * administrator assigned there; nobody can set their own.
   */
  current_tenant?: { id: string; name: string; role: string; open_id: string; department: string }
}

/**
 * PATCH /api/me/profile. A field left out is unchanged; "" clears it. There is
 * no department (an administrator assigns it) and no avatar (nowhere to keep
 * one yet): the server refuses both by name.
 */
export interface ProfilePatch {
  display_name?: string
  title?: string
  location?: string
}

/** A workspace the signed-in person belongs to, with their own role and the headcount. */
export interface MyWorkspace {
  id: string
  name: string
  role: string
  member_count: number
  /** The company domain the workspace claimed (novapay.vn); empty for a personal one. */
  domain: string
}

/**
 * What the browser keeps of the signed-in person. The store persists this to
 * localStorage so a reload can draw a signed-in shell, so it holds who they are
 * and nothing more: not the address, the phone number or the union id that the
 * sign-in answers also carry.
 */
export function sessionUser(u: { id: string; username: string; ngac_node_id?: string }) {
  return { id: u.id, username: u.username, ngac_node_id: u.ngac_node_id }
}

export const authApi = {
  // Signing in is not a request on behalf of a session: see publicFetch.
  requestOTP: (data: OTPRequestPayload) =>
    publicFetch<OTPRequestResponse>('/auth/otp/request', { method: 'POST', body: JSON.stringify(data) }),
  verifyOTP: (data: OTPVerifyPayload) =>
    publicFetch<OTPVerifyResponse>('/auth/otp/verify', { method: 'POST', body: JSON.stringify(data) }),

  providers: () => publicFetch<AuthProviders>('/auth/providers'),
  /** The signed-in person; with a workspace id, also their place in that workspace. */
  me: (workspaceId?: string) =>
    apiFetch<MeResponse>(workspaceId ? `/me?workspace=${encodeURIComponent(workspaceId)}` : '/me'),
  updateProfile: (patch: ProfilePatch) =>
    apiFetch<{ status: string }>('/me/profile', { method: 'PATCH', body: JSON.stringify(patch) }),

  /**
   * Proves the address on the account you are signed in to, without signing in
   * again. No `code` sends one to the address; with one it checks it. Neither
   * returns a token.
   */
  requestEmailVerification: () =>
    apiFetch<{ expires_in: number }>('/me/email/verify', { method: 'POST', body: JSON.stringify({}) }),
  confirmEmailVerification: (code: string) =>
    apiFetch<{ email_verified: boolean }>('/me/email/verify', { method: 'POST', body: JSON.stringify({ code }) }),
  /** Starts proving the address with Google; answers the Google URL to go to. */
  startEmailVerificationWithGoogle: () =>
    apiFetch<{ url: string }>('/me/email/verify/google', { method: 'POST' }),

  listMyWorkspaces: () => apiFetch<{ workspaces: MyWorkspace[] }>('/me/workspaces'),
  /** The name is the only input: a workspace is addressed by its ID, never by a slug. */
  createWorkspace: (name: string) =>
    apiFetch<MyWorkspace>('/me/workspaces', { method: 'POST', body: JSON.stringify({ name }) }),

  /** Re-scopes the session to a workspace the person actively belongs to. */
  switchTenant: (tenantId: string) =>
    apiFetch<{ access_token: string }>('/auth/switch-tenant', {
      method: 'POST',
      body: JSON.stringify({ tenant_id: tenantId }),
    }),
}

/**
 * Server endpoint that starts "Sign in with Google". The whole OAuth flow runs
 * on the server, so this is a full-page navigation rather than a fetch.
 */
export const GOOGLE_SIGN_IN_PATH = '/api/auth/google/start'

/**
 * The start URL, with `loginHint` (an email) pre-selecting the Google account.
 * That matters when verifying an address: only that account proves it.
 */
export function googleSignInUrl(loginHint?: string): string {
  const hint = loginHint?.trim()
  return hint ? `${GOOGLE_SIGN_IN_PATH}?login_hint=${encodeURIComponent(hint)}` : GOOGLE_SIGN_IN_PATH
}

/** Leaves the SPA for Google's sign-in page. */
export function startGoogleSignIn(loginHint?: string): void {
  window.location.assign(googleSignInUrl(loginHint))
}

/**
 * Leaves the SPA for an address the server named (the Google page that proves an
 * address). Only a web address: whatever else a response contained is not a
 * place to send a browser.
 */
export function leaveFor(url: string): void {
  if (!/^https?:\/\//i.test(url)) throw new Error('refusing to leave for a non-web address')
  window.location.assign(url)
}

/**
 * Finishes a sign-in that happened outside the SPA (Google redirects back to
 * /auth/google/done).
 *
 * The server set the httpOnly refresh cookie and deliberately put no token in
 * the URL, so the access token comes from the same refresh call that restores
 * a session on page load. Resolves to the account's state, or null when no
 * session could be established.
 */
export async function completeGoogleSignIn(): Promise<{ needsProfile: boolean } | null> {
  const token = await refreshAccessToken()
  if (!token) return null
  try {
    const me = await authApi.me()
    // apiFetch may itself have refreshed; use whatever token is current now.
    useAuthStore.getState().login(useAuthStore.getState().accessToken ?? token, sessionUser(me.user))
    return { needsProfile: me.user.needs_profile }
  } catch {
    useAuthStore.getState().logout()
    return null
  }
}
