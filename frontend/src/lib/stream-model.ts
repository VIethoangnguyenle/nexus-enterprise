import type { Message } from '../api/messaging'
import { formatDate, formatTime, toDate, toMillis } from './format'
import { isSystemLine } from './message-model'

/**
 * The shape of a conversation stream (DESIGN.md "Chat: luồng in-line"): which
 * messages group under one name, where the day and "Tin mới" dividers go, and
 * what a thread summary says. Pure, so the rules are tested without a DOM.
 */

export type StreamMessage = Message & { _optimistic?: boolean }

/** A message follows the previous one of the same sender in a group within this window. */
export const GROUP_WINDOW_MS = 5 * 60_000

export type StreamItem =
  | { kind: 'day'; key: string; label: string }
  | { kind: 'unread'; key: string }
  | { kind: 'message'; key: string; message: StreamMessage; mine: boolean; grouped: boolean; system: boolean }

const WEEKDAYS = ['Chủ nhật', 'Thứ Hai', 'Thứ Ba', 'Thứ Tư', 'Thứ Năm', 'Thứ Sáu', 'Thứ Bảy']
const pad = (n: number) => String(n).padStart(2, '0')

function startOfDay(d: Date): number {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
}

function daysAgo(d: Date, now: Date): number {
  return Math.round((startOfDay(now) - startOfDay(d)) / 86_400_000)
}

/** "Hôm nay", "Hôm qua", "Thứ Hai, 06/10" within a week, then "28/09/2026". */
export function dayLabel(value: unknown, now: Date = new Date()): string {
  const d = toDate(value)
  if (!d) return ''
  const ago = daysAgo(d, now)
  if (ago === 0) return 'Hôm nay'
  if (ago === 1) return 'Hôm qua'
  if (ago > 1 && ago < 7) return `${WEEKDAYS[d.getDay()]}, ${pad(d.getDate())}/${pad(d.getMonth() + 1)}`
  return formatDate(d)
}

/** Task notes and other system lines (a manager's deletion is logged) sit between messages and do not group. */
export const isSystemMessage = isSystemLine

export const keyOfMessage = (m: Message & { _clientKey?: string }): string => m._clientKey ?? m.id

/** Messages oldest first → rows with their dividers and grouping flags. */
export function buildStream(
  messages: StreamMessage[],
  opts: { me: string | undefined; now?: Date; unreadFromId?: string | null },
): StreamItem[] {
  const now = opts.now ?? new Date()
  const items: StreamItem[] = []
  let prev: StreamMessage | null = null
  let lastDay = ''
  for (const m of messages) {
    const day = dayLabel(m.created_at, now)
    let broke = false
    if (day && day !== lastDay) {
      items.push({ kind: 'day', key: `day:${day}`, label: day })
      lastDay = day
      broke = true
    }
    if (opts.unreadFromId && m.id === opts.unreadFromId) {
      items.push({ kind: 'unread', key: 'unread' })
      broke = true
    }
    const system = isSystemMessage(m)
    const grouped =
      !broke &&
      !!prev &&
      !system &&
      !isSystemMessage(prev) &&
      prev.sender_id === m.sender_id &&
      !(prev.reply_count && prev.reply_count > 0) &&
      toMillis(m.created_at) - toMillis(prev.created_at) <= GROUP_WINDOW_MS
    items.push({
      kind: 'message', key: keyOfMessage(m), message: m, mine: !!opts.me && m.sender_id === opts.me, grouped, system,
    })
    prev = m
  }
  return items
}

/**
 * The first message the reader has not seen: after their read position, sent by
 * someone else. Messages older than the loaded page cannot be placed, so then
 * the unread count says how many of the newest ones are new.
 */
export function unreadStartId(
  messages: StreamMessage[],
  opts: { lastReadId: string; unreadCount: number; me: string | undefined },
): string | null {
  const persisted = (m: StreamMessage) => !m._optimistic && !m.id.startsWith('temp-')
  const idx = opts.lastReadId ? messages.findIndex((m) => m.id === opts.lastReadId) : -1
  let from: number
  if (idx >= 0) from = idx + 1
  else if (opts.unreadCount > 0) from = Math.max(0, messages.length - opts.unreadCount)
  else return null
  return messages.slice(from).find((m) => persisted(m) && m.sender_id !== opts.me)?.id ?? null
}

/**
 * The reply a thread's "Tin mới" divider goes before: the reader has `unreadCount`
 * unread replies, all by other people and the newest ones, so count back over
 * replies that are not theirs.
 */
export function firstUnreadReplyId(replies: StreamMessage[], unreadCount: number, me: string | undefined): string | null {
  if (unreadCount <= 0) return null
  let left = unreadCount
  let first: string | null = null
  for (let i = replies.length - 1; i >= 0 && left > 0; i--) {
    const r = replies[i]!
    if (r.sender_id === me || r._optimistic) continue
    first = r.id
    left--
  }
  return first
}

/** "09:20" today, "Hôm qua 17:05", "06/10" this year, "31/12/2025" before. */
export function formatLastReply(value: unknown, now: Date = new Date()): string {
  const d = toDate(value)
  if (!d) return ''
  const ago = daysAgo(d, now)
  if (ago === 0) return formatTime(d)
  if (ago === 1) return `Hôm qua ${formatTime(d)}`
  if (d.getFullYear() === now.getFullYear()) return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}`
  return formatDate(d)
}

export interface ThreadSummaryView {
  count: number
  /** Newest replier first, at most three, by display name. */
  faces: { id: string; name: string }[]
  last: string
  following: boolean
  /** Unread replies; 0 unless the reader follows the thread. */
  unread: number
  /** What a screen reader says for the whole row. */
  label: string
}

/** The summary row under a root message, or null when nobody has replied. */
export function threadSummary(
  m: Message,
  nameOf: (userId: string) => string,
  now: Date = new Date(),
): ThreadSummaryView | null {
  const count = m.reply_count ?? 0
  if (count <= 0) return null
  const ids = [...new Set(m.reply_participant_ids ?? [])].slice(0, 3)
  const last = formatLastReply(m.last_reply_at, now)
  const following = !!m.following
  const unread = following ? Math.max(0, m.thread_unread_count ?? 0) : 0
  const parts = ['Thread', `${count} trả lời`]
  if (last) parts.push(`lần cuối ${last}`)
  if (unread > 0) parts.push(`${unread} trả lời mới`)
  return {
    count, last, following, unread, label: parts.join(', '),
    faces: ids.map((id) => ({ id, name: nameOf(id) })),
  }
}

/**
 * RFC 3339 with the full precision the server stored, for `before=`: the
 * messages query compares with `<`, so a time rounded to the millisecond would
 * skip or repeat rows. Empty when the value cannot be read.
 */
export function timestampToRfc3339(value: unknown): string {
  if (value && typeof value === 'object' && 'seconds' in (value as object)) {
    const { seconds, nanos } = value as { seconds: number | string; nanos?: number }
    const d = new Date(Number(seconds) * 1000)
    if (isNaN(d.getTime())) return ''
    const frac = String(nanos ?? 0).padStart(9, '0').replace(/0+$/, '')
    return d.toISOString().replace('.000Z', frac ? `.${frac}Z` : 'Z')
  }
  const d = toDate(value)
  return d ? d.toISOString() : ''
}

/**
 * The newest page just fetched, plus the older pages the reader had already
 * loaded by scrolling up: a refetch (after a reconnect, say) would otherwise
 * throw those away and move the reader. Lists are newest first.
 */
export function keepOlderPages(
  fresh: { messages: Message[]; has_more: boolean },
  previous: { messages: Message[]; has_more: boolean } | undefined,
): { messages: Message[]; has_more: boolean } {
  const oldestFresh = fresh.messages[fresh.messages.length - 1]
  if (!previous || !oldestFresh) return fresh
  const cutoff = toMillis(oldestFresh.created_at)
  const have = new Set(fresh.messages.map((m) => m.id))
  // Older pages join on only where the fresh page reaches them. When more than
  // a page arrived meanwhile (a laptop asleep), what lies between is unknown:
  // gluing the old pages on would hide that gap and its "more history" flag.
  if (fresh.has_more && !previous.messages.some((m) => have.has(m.id))) return fresh
  const older = previous.messages.filter(
    (m) => !have.has(m.id) && !m.id.startsWith('temp-') && toMillis(m.created_at) < cutoff,
  )
  if (older.length === 0) return fresh
  return { messages: [...fresh.messages, ...older], has_more: previous.has_more }
}
