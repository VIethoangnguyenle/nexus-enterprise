import type { ActivityMessage, Channel, ChannelUnread, ConversationActivity } from '../api/messaging'
import type { NotificationLevel } from '../api/notification-levels'
import { toMillis } from './format'
import { displayName, type PeopleDirectory, type Person } from './people'

export type ConversationKind = 'dm' | 'space'

/** One message as a list row quotes it: who, what, when, and whom it names. */
export interface FeedLine {
  id: string
  /** Display name of the sender ("Bạn" for me). */
  author: string
  /** What the row says: the opening of the text, or words for a file or card. */
  text: string
  /** Epoch ms. */
  at: number
  /** Display names of the people the message mentions, for the chips in `text`. */
  mentionNames: string[]
  /** The message names the caller. */
  mentionsMe: boolean
}

/** A thread reply that is newer than the conversation's last main-stream message. */
export interface FeedReply extends FeedLine {
  /** The root message of the thread it belongs to; opens that thread. */
  parentId: string
}

/**
 * One row of the navigator and of Home: a direct message or a space, joined
 * with what we know about its activity. Ids are carried for routing and
 * colour only.
 */
export interface Conversation {
  id: string
  kind: ConversationKind
  /** Space name, or the other person's display name for a DM. */
  title: string
  /** Key for the avatar / space-icon colour. Never rendered. */
  hueKey: string
  /** A space's own picture; absent when it has none, and always absent for a DM. */
  avatarUrl?: string
  /** The other person in a DM, when the directory knows them. */
  partner?: Person
  unread: number
  memberCount: number
  /** Epoch ms of the newest message or thread reply; 0 when unknown. */
  lastActivity: number
  /** Preview text of the newest message, main stream or thread reply, when known. */
  preview?: string
  /** Display name of whoever wrote the preview ("Bạn" for me). */
  previewAuthor?: string
  /** Who wrote the newest message, for realtime attribution. Not rendered. */
  lastSenderId?: string
  /** The newest main-stream message. */
  main?: FeedLine
  /** The newest thread reply, only while it is newer than `main`. */
  reply?: FeedReply
  /** The caller's notification level here; absent means the default, `main`. A DM is never `all`. */
  level?: NotificationLevel
  /** Unread main messages that mention the caller, at every level. */
  mentions?: number
  /** The caller starred this conversation: it is listed under "Đã gắn sao" only. */
  starred?: boolean
  /** When it was starred, in ms: the order of "Đã gắn sao". */
  starredAt?: number
}

/** What the unread list says about one conversation besides how many messages are unread. */
export interface ConversationMeta {
  level?: NotificationLevel
  mentions?: number
  starred?: boolean
  starredAt?: number
}

export type ConversationFilter = 'all' | 'unread' | 'spaces' | 'direct'

/** The newest message a live event announced for a conversation. */
export interface LastMessage {
  id?: string
  content: string
  timestamp: string
  senderName: string
  senderId?: string
  /** Set when the message is a thread reply: the root it belongs to. */
  parentId?: string
  /** User ids the message names. */
  mentions?: string[]
  kind?: ActivityMessage['kind']
  attachmentName?: string
}

/**
 * What a row says for a message: the words of a text, "📎 name" for a file,
 * "Bình chọn: …" and "Công việc: …" for those cards.
 */
export function feedText(kind: ActivityMessage['kind'] | undefined, text: string, attachmentName?: string): string {
  switch (kind) {
    case 'attachment':
      return `📎 ${attachmentName || 'Tệp đính kèm'}`
    case 'poll':
      return text ? `Bình chọn: ${text}` : 'Bình chọn'
    case 'task':
      return text ? `Công việc: ${text}` : 'Công việc'
    case 'card':
      return text || 'Thẻ đính kèm'
    case 'call':
      // Server sends the full preview text: "📹 Cuộc gọi video · 32 phút", "📹 Cuộc gọi nhỡ".
      return text || '📹 Cuộc gọi'
    default:
      return text
  }
}

export function isDirect(ch: Pick<Channel, 'channel_type'>): boolean {
  return ch.channel_type === 'dm'
}

/**
 * The other participant of a DM. The backend names a DM after both
 * usernames ("alice, bob", see ngac.DMChannelName), so the partner is the
 * name that is not mine. Falls back to the channel name as-is.
 */
export function dmPartner(
  ch: Pick<Channel, 'name'>,
  me: { username?: string },
  dir: PeopleDirectory,
): { title: string; partner?: Person } {
  const parts = ch.name.split(',').map((s) => s.trim()).filter(Boolean)
  const mine = me.username?.toLowerCase()
  const idx = parts.findIndex((p) => p.toLowerCase() === mine)
  const others = idx >= 0 ? parts.filter((_, i) => i !== idx) : parts
  // Only trust the split when exactly one other name is left; otherwise the
  // channel name itself is the safest title.
  const other = others.length === 1 ? others[0] : undefined
  if (other) {
    const partner = dir.byUsername.get(other.toLowerCase())
    return { title: partner?.name ?? other, partner }
  }
  return { title: ch.name }
}

interface BuildInput {
  spaces: Channel[]
  dms: Channel[]
  unread: Record<string, number>
  /** Level, @mentions and star per conversation, from the unread list. */
  meta?: Record<string, ConversationMeta>
  /** What the server says each conversation last said; the base of every preview. */
  activity?: Record<string, ConversationActivity>
  /** The newest main-stream message live events announced this session; shown when newer than the server's. */
  lastMessages: Record<string, LastMessage>
  /** The newest thread reply live events announced this session, beside `lastMessages`. */
  lastThreadMessages?: Record<string, LastMessage>
  me: { id?: string; username?: string }
  dir: PeopleDirectory
}

function lineFrom(
  m: { id?: string; text: string; at: number; senderId?: string; senderName: string; mentions?: string[] },
  me: BuildInput['me'],
  dir: PeopleDirectory,
): FeedLine {
  const ids = m.mentions ?? []
  return {
    id: m.id ?? '',
    author: me.id && m.senderId === me.id ? 'Bạn' : displayName(dir, m.senderId, m.senderName),
    text: m.text,
    at: m.at,
    mentionNames: ids.flatMap((id) => dir.byUserId.get(id)?.name ?? []),
    mentionsMe: !!me.id && ids.includes(me.id),
  }
}

/** A line with who wrote it, for realtime attribution; the row itself never shows the id. */
type Line = FeedLine & { senderId?: string }

const fromActivity = (m: ActivityMessage, me: BuildInput['me'], dir: PeopleDirectory): Line => ({
  ...lineFrom(
    {
      id: m.id, text: feedText(m.kind, m.preview, m.attachment_name), at: toMillis(m.created_at),
      senderId: m.sender_id, senderName: m.sender_name, mentions: m.mentions,
    },
    me, dir,
  ),
  senderId: m.sender_id,
})

/**
 * The main-stream message and the newer thread reply of one conversation: the
 * server's answer, unless a live event is newer. The two sides are kept apart,
 * each merged against its own server counterpart, so a live reply never hides
 * a newer live main message. A reply is shown only when it is newer than the
 * main message, as the server answers it.
 */
function mergeLines(
  a: ConversationActivity | undefined,
  liveMain: LastMessage | undefined,
  liveReply: LastMessage | undefined,
  me: BuildInput['me'],
  dir: PeopleDirectory,
): { main?: Line; reply?: Line & { parentId: string } } {
  const liveLine = (live: LastMessage): Line => ({
    ...lineFrom(
      {
        id: live.id, text: feedText(live.kind, live.content, live.attachmentName), at: toMillis(live.timestamp),
        senderId: live.senderId, senderName: live.senderName, mentions: live.mentions,
      },
      me, dir,
    ),
    senderId: live.senderId,
  })
  let main = a ? fromActivity(a.latest, me, dir) : undefined
  let reply = a?.latest_reply
    ? { ...fromActivity(a.latest_reply, me, dir), parentId: a.latest_reply.parent_message_id ?? '' }
    : undefined
  if (liveMain && toMillis(liveMain.timestamp) > (main?.at ?? 0)) main = liveLine(liveMain)
  if (liveReply && toMillis(liveReply.timestamp) > (reply?.at ?? 0)) {
    reply = { ...liveLine(liveReply), parentId: liveReply.parentId ?? '' }
  }
  if (reply && main && reply.at <= main.at) reply = undefined
  return { main, reply }
}

export function buildConversations({ spaces, dms, unread, meta = {}, activity = {}, lastMessages, lastThreadMessages = {}, me, dir }: BuildInput): Conversation[] {
  const seen = new Set<string>()
  const out: Conversation[] = []
  const push = (ch: Channel, kind: ConversationKind) => {
    if (seen.has(ch.id)) return
    seen.add(ch.id)
    const { main, reply } = mergeLines(activity[ch.id], lastMessages[ch.id], lastThreadMessages[ch.id], me, dir)
    const newest = reply ?? main
    const m = meta[ch.id]
    // A direct message has no middle level: "all" is read as the default there.
    const level: NotificationLevel = kind === 'dm' && m?.level === 'all' ? 'main' : (m?.level ?? 'main')
    const base = {
      id: ch.id,
      kind,
      // A muted conversation counts nothing of its own; only its @mentions reach the totals.
      unread: level === 'off' ? 0 : (unread[ch.id] ?? 0),
      level,
      mentions: m?.mentions ?? 0,
      starred: !!m?.starred,
      starredAt: m?.starred ? m.starredAt : undefined,
      memberCount: ch.member_count ?? 0,
      lastActivity: newest?.at ?? 0,
      preview: newest?.text || undefined,
      previewAuthor: newest?.author,
      lastSenderId: newest?.senderId,
      main: main && stripSender(main),
      reply: reply && stripSender(reply),
    }
    if (kind === 'dm') {
      const { title, partner } = dmPartner(ch, me, dir)
      out.push({ ...base, title, partner, hueKey: partner?.userId || ch.id })
    } else {
      out.push({ ...base, title: ch.name, hueKey: ch.id, avatarUrl: ch.avatar_url || undefined })
    }
  }
  // A DM can only be a DM; a space list that ever includes one still files it correctly.
  for (const ch of dms) push(ch, 'dm')
  for (const ch of spaces) push(ch, isDirect(ch) ? 'dm' : 'space')
  return out
}

function stripSender<T extends { senderId?: string }>({ senderId: _senderId, ...line }: T): Omit<T, 'senderId'> {
  return line
}

const collator = new Intl.Collator('vi', { sensitivity: 'base' })

/**
 * Latest activity first. The list API carries no last-message time, so
 * activity is only known for conversations that spoke during this session;
 * the rest fall back to unread first, then name.
 */
export function sortConversations(list: Conversation[]): Conversation[] {
  return [...list].sort((a, b) => {
    if (a.lastActivity !== b.lastActivity) return b.lastActivity - a.lastActivity
    const ua = a.unread > 0 ? 1 : 0
    const ub = b.unread > 0 ? 1 : 0
    if (ua !== ub) return ub - ua
    return collator.compare(a.title, b.title)
  })
}

export function filterConversations(list: Conversation[], filter: ConversationFilter): Conversation[] {
  switch (filter) {
    case 'unread':
      // The same count the badge shows: a muted conversation qualifies by its unread @mentions.
      return list.filter((c) => unreadOf(c) > 0)
    case 'spaces':
      return list.filter((c) => c.kind === 'space')
    case 'direct':
      return list.filter((c) => c.kind === 'dm')
    default:
      return list
  }
}

/**
 * What a conversation adds to a total (Tin nhắn on the sidebar, Trang chủ, a
 * folded section, the phone tab): its unread messages, or, once muted, only
 * the times someone named the caller.
 */
export function unreadOf(c: Pick<Conversation, 'unread' | 'level' | 'mentions'>): number {
  return c.level === 'off' ? (c.mentions ?? 0) : c.unread
}

export function totalUnread(list: Conversation[]): number {
  return list.reduce((n, c) => n + unreadOf(c), 0)
}

/** The same total over the raw unread list, for the sidebar badge that needs no conversation titles. */
export function unreadByLevel(channels: ChannelUnread[] | undefined): number {
  return (channels ?? []).reduce(
    (n, c) => n + (c.notification_level === 'off' ? (c.mention_count ?? 0) : c.unread_count),
    0,
  )
}
