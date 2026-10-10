import { useMutation, useQuery } from '@tanstack/react-query'
import {
  authApi,
  sessionUser,
  type OTPRequestPayload,
  type OTPVerifyPayload,
  type ProfilePatch,
} from '../api/auth'
import { queryClient } from '../lib/query-client'
import { tenantIdFromToken, useAuthStore } from '../stores/auth.store'
import { keys } from './keys'

/**
 * Sign-in methods the server has configured. A failed lookup hides Google
 * (a button that cannot work is worse than none) and keeps the code form, so
 * `isError` is exposed for the screen to decide.
 */
export function useProviders() {
  return useQuery({
    queryKey: keys.auth.providers(),
    queryFn: () => authApi.providers(),
    staleTime: 5 * 60 * 1000,
    retry: false,
  })
}

// The sign-in screens show a failure inline, next to the field it belongs to,
// so none of these mutations also raise the shared toast.
export function useRequestOTP() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (data: OTPRequestPayload) => authApi.requestOTP(data),
  })
}

export function useVerifyOTP() {
  const login = useAuthStore((s) => s.login)
  return useMutation({
    meta: { silentError: true },
    mutationFn: (data: OTPVerifyPayload) => authApi.verifyOTP(data),
    onSuccess: (res) => login(res.access_token, sessionUser(res.user)),
  })
}

/** The signed-in person as the server sees them. */
export function useMe() {
  const signedIn = useAuthStore((s) => !!s.user)
  return useQuery({
    queryKey: keys.auth.me(),
    queryFn: () => authApi.me(),
    enabled: signedIn,
    select: (r) => r.user,
  })
}

/** Saves the profile step; only the fields given change. */
export function useSaveProfile() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (patch: ProfilePatch) => authApi.updateProfile(patch),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.auth.me() }),
  })
}

/** The person's workspaces with role and headcount. */
export function useMyWorkspaces() {
  return useQuery({
    queryKey: keys.auth.workspaces(),
    queryFn: () => authApi.listMyWorkspaces(),
    select: (r) => r.workspaces ?? [],
    // The picker decorates the app's own list with these; losing them costs a
    // subtitle, not the screen.
    retry: false,
  })
}

/** Creates a workspace from its name alone. */
export function useCreateMyWorkspace() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (name: string) => authApi.createWorkspace(name),
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: keys.workspaces.all() }),
        queryClient.invalidateQueries({ queryKey: keys.auth.workspaces() }),
      ]),
  })
}

/**
 * Makes the session speak for a workspace before the app opens it. Services
 * that keep per-workspace data read the workspace from the token, so a token
 * scoped to another workspace would show that one's data. A no-op when the
 * token already is for this workspace.
 */
export function useSwitchToWorkspace() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: async (workspaceId: string) => {
      const { accessToken, setAccessToken } = useAuthStore.getState()
      if (tenantIdFromToken(accessToken) === workspaceId) return workspaceId
      const res = await authApi.switchTenant(workspaceId)
      setAccessToken(res.access_token)
      return workspaceId
    },
  })
}

// --- proving the address of the account you are signed in to -----------------
// None of these signs anyone in: they only prove the address, so none of them
// touch the session.

/** Sends a code to the address on the account. */
export function useRequestEmailVerification() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: () => authApi.requestEmailVerification(),
  })
}

/** Checks the code the account asked for. */
export function useConfirmEmailVerification() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (code: string) => authApi.confirmEmailVerification(code),
  })
}

/** Starts proving the address with Google; resolves to the Google URL to go to. */
export function useStartEmailVerificationWithGoogle() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: async () => (await authApi.startEmailVerificationWithGoogle()).url,
  })
}
