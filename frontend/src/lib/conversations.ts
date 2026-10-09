import type { Channel } from '../api/messaging'
import { displayName, type PeopleDirectory, type Person } from './people'

export type ConversationKind = 'dm' | 'space'

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
  /** The other person in a DM, when the directory knows them. */
  partner?: Person
  unread: number
  memberCount: number
  /** Epoch ms of the latest message seen this session; 0 when unknown. */
  lastActivity: number
  /** Latest message preview text, when known. */
  preview?: string
  /** Display name of whoever wrote the preview ("Bạn" for me). */
  previewAuthor?: string
  /** Who wrote the latest message, for realtime attribution. Not rendered. */
  lastSenderId?: string
}

export type ConversationFilter = 'all' | 'unread' | 'spaces' | 'direct'

export interface LastMessage {
  content: string
  timestamp: string
  senderName: string
  senderId?: string
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
  lastMessages: Record<string, LastMessage>
  me: { id?: string; username?: string }
  dir: PeopleDirectory
}

export function buildConversations({ spaces, dms, unread, lastMessages, me, dir }: BuildInput): Conversation[] {
  const seen = new Set<string>()
  const out: Conversation[] = []
  const push = (ch: Channel, kind: ConversationKind) => {
    if (seen.has(ch.id)) return
    seen.add(ch.id)
    const last = lastMessages[ch.id]
    const fromMe = !!last && !!me.id && last.senderId === me.id
    const base = {
      id: ch.id,
      kind,
      unread: unread[ch.id] ?? 0,
      memberCount: ch.member_count ?? 0,
      lastActivity: last ? Date.parse(last.timestamp) || 0 : 0,
      preview: last?.content || undefined,
      previewAuthor: last ? (fromMe ? 'Bạn' : displayName(dir, last.senderId, last.senderName)) : undefined,
      lastSenderId: last?.senderId,
    }
    if (kind === 'dm') {
      const { title, partner } = dmPartner(ch, me, dir)
      out.push({ ...base, title, partner, hueKey: partner?.userId || ch.id })
    } else {
      out.push({ ...base, title: ch.name, hueKey: ch.id })
    }
  }
  // A DM can only be a DM; a space list that ever includes one still files it correctly.
  for (const ch of dms) push(ch, 'dm')
  for (const ch of spaces) push(ch, isDirect(ch) ? 'dm' : 'space')
  return out
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
      return list.filter((c) => c.unread > 0)
    case 'spaces':
      return list.filter((c) => c.kind === 'space')
    case 'direct':
      return list.filter((c) => c.kind === 'dm')
    default:
      return list
  }
}

export function totalUnread(list: Conversation[]): number {
  return list.reduce((n, c) => n + c.unread, 0)
}
