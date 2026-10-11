import { queryClient } from './query-client'
import { keys } from '../hooks/keys'
import type { ThreadReplyEvent, ThreadStateEvent } from '../generated/proto/messaging/ws'
import type { Message, ThreadState } from '../api/messaging'
import { toMillis } from './format'

type MessageList = { messages: Message[]; has_more?: boolean }

/** Patch one root message wherever it is cached: its channel's stream and its open thread. */
export function patchRoot(channelId: string, rootId: string, patch: (m: Message) => Partial<Message> | null): void {
  const apply = (old: MessageList | undefined) => {
    if (!old) return old
    let changed = false
    const messages = (old.messages || []).map((m) => {
      if (m.id !== rootId) return m
      const p = patch(m)
      if (!p) return m
      changed = true
      return { ...m, ...p }
    })
    return changed ? { ...old, messages } : old
  }
  queryClient.setQueryData(keys.messaging.messages(channelId), apply)
  queryClient.setQueryData(keys.messaging.thread(rootId), apply)
}

/**
 * A reply landed: every viewer of the channel takes the thread summary the
 * server computed (count, last reply time, newest repliers). A summary with a
 * lower count than the one shown arrived late and is ignored. The channel's
 * thread lists are reordered by the server, so they are marked stale.
 */
export function applyThreadReplySummary(ev: ThreadReplyEvent): void {
  const channelId = ev.message?.channelId
  if (!channelId || !ev.parentMessageId) return
  // A reply announced without its summary (the server could not build one) only adds the reply.
  if (ev.replyCount > 0) {
    patchRoot(channelId, ev.parentMessageId, (m) =>
      (m.reply_count ?? 0) > ev.replyCount
        ? null
        : { reply_count: ev.replyCount, last_reply_at: ev.lastReplyAt, reply_participant_ids: ev.replyParticipantIds },
    )
  }
  queryClient.invalidateQueries({ queryKey: keys.messaging.threadsListOf(channelId) })
}

/** Newest server read time applied per thread, so a late frame cannot undo a newer one. */
const stateAsOf = new Map<string, number>()

function asOfMs(ev: ThreadStateEvent): number {
  return ev.asOf ? Number(ev.asOf.seconds) * 1000 + Math.floor((ev.asOf.nanos ?? 0) / 1e6) : 0
}

function applyState(channelId: string, rootId: string, following: boolean, unread: number, at: number): void {
  if (!channelId || !rootId) return
  if (at && at < (stateAsOf.get(rootId) ?? 0)) return
  if (at) stateAsOf.set(rootId, at)
  patchRoot(channelId, rootId, () => ({ following, thread_unread_count: unread }))
  queryClient.invalidateQueries({ queryKey: keys.messaging.unreadCounts() })
  queryClient.invalidateQueries({ queryKey: keys.messaging.threadsListOf(channelId) })
}

/**
 * The signed-in user's own state of a thread changed — a reply in a thread
 * they follow, or a follow, unfollow or read on any of their devices.
 */
export function applyThreadState(ev: ThreadStateEvent): void {
  applyState(ev.channelId, ev.rootMessageId, ev.following, ev.unreadCount, asOfMs(ev))
}

/** The same state, as the follow / unfollow / read request answered it. */
export function applyThreadStateAnswer(st: ThreadState): void {
  applyState(st.channel_id, st.root_message_id, st.following, st.unread_count ?? 0, toMillis(st.as_of))
}

/** Forget applied read times (sign-out, tests). */
export function resetThreadStateClock(): void {
  stateAsOf.clear()
}
