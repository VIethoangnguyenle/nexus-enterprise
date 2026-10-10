import { useMutation, useQuery } from '@tanstack/react-query'
import { invitationsApi } from '../api/invitations'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'
import { markInvitationRead } from './useNotifications'

/**
 * Offers addressed to the signed-in person. The server returns them only when
 * the account's address is verified, so an empty list does not mean there are
 * none; the screen asks `useMe().email_verified` to tell the two apart.
 */
export function useMyInvitations(enabled = true) {
  return useQuery({
    enabled,
    queryKey: keys.auth.invitations(),
    queryFn: () => invitationsApi.listMine(),
    select: (r) => r.invitations ?? [],
    retry: false,
  })
}

/** Joins the workspace. Inline errors, so no shared toast. */
export function useAcceptInvitation() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (id: string) => invitationsApi.accept(id),
    onSuccess: (_r, id) =>
      Promise.all([
        markInvitationRead(id),
        queryClient.invalidateQueries({ queryKey: keys.workspaces.all() }),
        queryClient.invalidateQueries({ queryKey: keys.auth.workspaces() }),
        queryClient.invalidateQueries({ queryKey: keys.auth.invitations() }),
      ]),
  })
}

export function useDeclineInvitation() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (id: string) => invitationsApi.decline(id),
    onSuccess: (_r, id) =>
      Promise.all([
        markInvitationRead(id),
        queryClient.invalidateQueries({ queryKey: keys.auth.invitations() }),
      ]),
  })
}
