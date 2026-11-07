import type { ChannelUnread } from '../api/messaging'
import type { ConversationActivityEvent } from '../generated/proto/messaging/ws'
import { mentionsMe } from './shortcut-feed'

/** The unread counts as the cache holds them (`keys.messaging.unreadCounts()`). */
export type UnreadAnswer = { channels: ChannelUnread[] }

/** Who is reading: the signed-in user, and the conversation they have on screen now. */
export interface ActivityReader {
  me: string | undefined
  viewing: string | null
}

export interface ActivityOutcome {
  next: UnreadAnswer | undefined
  /** The counts cannot be followed from this event alone: ask the server for them. */
  refetch: boolean
}

/**
 * The unread counts after a conversation's activity (docs/specs/chat-unread-counts).
 *
 * A main message sent by someone else to a conversation not on screen adds one
 * unread (none when it is muted) and one mention when it names the reader. Its
 * number is remembered, so a repeated event counts nothing and a skipped number
 * (an event that never arrived) asks for the counts again. A deleted main
 * message the reader had not read takes one off. Replies are counted by the
 * thread events of the threads the reader follows; only a conversation set to
 * hear every reply asks again on a reply.
 */
export function unreadAfterActivity(
  old: UnreadAnswer | undefined,
  ev: ConversationActivityEvent,
  reader: ActivityReader,
): ActivityOutcome {
  if (!old) return { next: old, refetch: false }
  const item = old.channels.find((c) => c.channel_id === ev.channelId)
  if (!item) return { next: old, refetch: true }

  const level = item.notification_level ?? 'main'
  const seq = Number(ev.seq) || 0
  const fromMe = !!reader.me && ev.senderId === reader.me
  const onScreen = reader.viewing === ev.channelId

  if (ev.rootMessageId) {
    const heard = level === 'all' && !fromMe && !onScreen
    return { next: old, refetch: heard || (ev.kind === 'deleted' && level === 'all') }
  }

  const patch = (p: Partial<ChannelUnread>): UnreadAnswer => ({
    ...old,
    channels: old.channels.map((c) => (c.channel_id === ev.channelId ? { ...c, ...p } : c)),
  })

  if (ev.kind === 'deleted') {
    const unreadOne = seq > (item.last_read_seq ?? 0) && level !== 'off' && item.unread_count > 0
    // Whether the deleted message named the reader is not told: ask only when a mention is shown.
    return { next: unreadOne ? patch({ unread_count: item.unread_count - 1 }) : old, refetch: (item.mention_count ?? 0) > 0 }
  }

  const known = item.last_seq ?? 0
  if (seq === 0 || seq <= known) return { next: old, refetch: false }
  // A missing last_seq is 0: a conversation never written to, or counts held from before the
  // numbers existed; either way a message numbered above 1 shows that some were not counted here.
  const gap = seq > known + 1
  if (fromMe || onScreen) {
    return { next: patch({ last_seq: seq }), refetch: gap }
  }
  return {
    next: patch({
      last_seq: seq,
      unread_count: level === 'off' ? item.unread_count : item.unread_count + 1,
      mention_count: (item.mention_count ?? 0) + (mentionsMe(ev.mentions, ev.senderId, reader.me) ? 1 : 0),
    }),
    refetch: gap,
  }
}
