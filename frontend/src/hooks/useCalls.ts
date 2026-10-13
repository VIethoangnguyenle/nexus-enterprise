import { useMutation, useQuery } from '@tanstack/react-query'
import { callsApi, type Call, type CallKind } from '../api/calls'
import { queryClient } from '../lib/query-client'
import { toMillis } from '../lib/format'
import { useAuthStore } from '../stores/auth.store'
import { useMessages } from './useMessages'
import { keys } from './keys'

/**
 * One call as the server has it: who is in, whether it is still open. While
 * `poll` is on (the pre-join screen, where "who is in the call" and "the call
 * is full" change under the reader's eyes) it refreshes every few seconds.
 */
export function useCall(callId: string, opts: { poll?: boolean; enabled?: boolean } = {}) {
  return useQuery<Call>({
    queryKey: keys.calls.detail(callId),
    queryFn: () => callsApi.get(callId),
    enabled: !!callId && (opts.enabled ?? true),
    refetchInterval: opts.poll ? 5000 : false,
    // A call that is over or unreachable does not come back by asking again.
    retry: false,
  })
}

/** The call ringing or running in a conversation, or null; realtime call events keep it fresh. */
export function useOpenCall(channelId: string) {
  return useQuery<Call | null>({
    queryKey: keys.calls.open(channelId),
    queryFn: () => callsApi.open(channelId),
    enabled: !!channelId,
  })
}

/** Starts a call in a conversation, or joins the one already open there. Resolves to the entry ticket. */
export function useStartCall() {
  return useMutation({
    meta: { action: 'bắt đầu cuộc gọi' },
    mutationFn: ({ channelId, kind }: { channelId: string; kind: CallKind }) => callsApi.start(channelId, kind),
  })
}

/** "Mời" in the people panel: a notification to people of the group who are not in the call. */
export function useInviteToCall(callId: string) {
  return useMutation({
    meta: { action: 'mời vào cuộc gọi' },
    mutationFn: (userIds: string[]) => callsApi.invite(callId, userIds),
  })
}

/** The part of a realtime `call_event` a call screen needs. */
export interface CallEventLike { call_id?: string; type?: string; status?: string }

/**
 * For the realtime layer: a `call_event` makes the cached call stale (its people, its status),
 * so the call screens refetch. Only calls already on screen are touched.
 */
export function applyCallEvent(ev: CallEventLike): void {
  if (!ev.call_id) return
  void queryClient.invalidateQueries({ queryKey: keys.calls.detail(ev.call_id) })
}

/**
 * How many messages from other people arrived in the conversation after `sinceMs`:
 * the chat badge on the control bar, and the count on the summary.
 */
export function useMessagesSince(channelId: string, sinceMs: number, opts: { includeMine?: boolean } = {}): number {
  const { data } = useMessages(channelId)
  const me = useAuthStore((s) => s.user?.id)
  return (data?.messages ?? []).filter(
    (m) => !m.parent_message_id && toMillis(m.created_at) > sinceMs && (opts.includeMine || m.sender_id !== me),
  ).length
}
