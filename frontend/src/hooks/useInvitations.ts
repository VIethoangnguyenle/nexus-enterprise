import { useMutation, useQuery } from '@tanstack/react-query'
import { invitationsApi } from '../api/invitations'
import { queryClient } from '../lib/query-client'
import { keys } from './keys'
import { useMe } from './useAuth'

/**
 * Offers addressed to the signed-in person. The server returns them only when
 * the account's address is verified, so an empty list does not mean there are
 * none; the screen asks `useMe().email_verified` to tell the two apart.
 */
export function useMyInvitations() {
  return useQuery({
    queryKey: keys.auth.invitations(),
    queryFn: () => invitationsApi.listMine(),
    select: (r) => r.invitations ?? [],
    retry: false,
  })
}

/** How often the signed-in person's open offers are looked up again while the app is open. */
const OFFER_POLL_MS = 60_000

/**
 * How many offers wait for an answer, for the workspace menu. Only an account
 * with a verified address can have any, so nothing is asked for otherwise.
 * Looked up again every minute and when the window regains focus, so an offer
 * sent while the person is already inside shows without a reload. Shares its
 * cache with `useMyInvitations`, so answering an offer updates the count.
 */
export function useOpenInvitationCount(): number {
  const me = useMe().data
  const canHaveOffers = !!me?.email && me.email_verified === true
  const offers = useQuery({
    queryKey: keys.auth.invitations(),
    queryFn: () => invitationsApi.listMine(),
    select: (r) => r.invitations ?? [],
    enabled: canHaveOffers,
    retry: false,
    refetchInterval: OFFER_POLL_MS,
    refetchOnWindowFocus: true,
  })
  return canHaveOffers ? (offers.data?.length ?? 0) : 0
}

/** Joins the workspace. Inline errors, so no shared toast. */
export function useAcceptInvitation() {
  return useMutation({
    meta: { silentError: true },
    mutationFn: (id: string) => invitationsApi.accept(id),
    onSuccess: () =>
      Promise.all([
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
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.auth.invitations() }),
  })
}
