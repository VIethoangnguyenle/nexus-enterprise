import { apiFetch } from './client'

/** An open offer addressed to the signed-in person's own account. */
export interface MyInvitation {
  id: string
  workspace_name: string
  inviter_name: string
  role_name: string
  department_name: string
  created_at: string
  expires_at: string
}

export interface AcceptedInvitation {
  workspace_id: string
  workspace_name: string
  /** False when the invitation named a role or department that was not given: the inviter could no longer give it. */
  role_applied: boolean
  department_applied: boolean
}

/**
 * The invitee's side of an invitation. The endpoints take no person: who is
 * asking is the token's, and an invitation is matched to the address on that
 * person's account. The screen that lists these on workspace selection belongs
 * to the sign-in group.
 */
export const invitationsApi = {
  listMine: () => apiFetch<{ invitations: MyInvitation[] }>('/invitations'),
  accept: (id: string) => apiFetch<AcceptedInvitation>(`/invitations/${id}/accept`, { method: 'POST' }),
  decline: (id: string) => apiFetch<void>(`/invitations/${id}/decline`, { method: 'POST' }),
}
