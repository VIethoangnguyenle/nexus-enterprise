import type { AppNotification } from '../api/notifications'
import { toDate } from './format'

/**
 * How a notification reads, where it leads and how it groups. The server sends
 * facts (type, actor, subject, params); every word the person reads is built
 * here (design/mockups/notifications.html §6). No id is ever printed: a name
 * that is missing becomes a plain word ("một đề nghị"), never the id.
 */

/** A run of a sentence; `strong` marks a person's or a thing's name (600). */
export interface Piece {
  text: string
  strong?: boolean
}

export interface Sentence {
  /** Who did what to which thing. */
  lead: Piece[]
  /** A second sentence, only when it changes what the reader does next. */
  follow?: string
  /** Why, for a rejection that came with a reason. */
  reason?: string
}

export type NotificationDomain = 'approval' | 'asset' | 'workspace' | 'other'

const strong = (text: string): Piece => ({ text, strong: true })
const plain = (text: string): Piece => ({ text })

/** The words of one type: with the actor, and without one. */
interface Copy {
  /** Builds the lead from the actor's and the subject's names. */
  by: (actor: Piece, subject: Piece) => Piece[]
  alone: (subject: Piece) => Piece[]
  /** What an unnamed subject is called. */
  fallback: string
  follow?: string
  domain: NotificationDomain
  tag: string
}

const COPY: Record<string, Copy> = {
  approval_approved: {
    by: (a, s) => [a, plain(' đã duyệt '), s, plain('.')],
    alone: (s) => [s, plain(' đã được duyệt.')],
    fallback: 'một đề nghị', follow: 'Đề nghị đã hoàn tất.', domain: 'approval', tag: 'vừa duyệt',
  },
  approval_step_approved: {
    by: (a, s) => [a, plain(' đã duyệt bước của mình cho '), s, plain('.')],
    alone: (s) => [plain('Một bước của '), s, plain(' đã được duyệt.')],
    fallback: 'một đề nghị', follow: 'Đề nghị vẫn đang chờ các bước sau.', domain: 'approval', tag: 'vừa duyệt',
  },
  approval_rejected: {
    by: (a, s) => [a, plain(' đã trả lại '), s, plain('.')],
    alone: (s) => [s, plain(' đã bị trả lại.')],
    fallback: 'một đề nghị', domain: 'approval', tag: 'vừa trả lại',
  },
  asset_request_approved: {
    by: (a, s) => [a, plain(' đã duyệt yêu cầu cấp '), s, plain(' của bạn.')],
    alone: (s) => [plain('Yêu cầu cấp '), s, plain(' của bạn đã được duyệt.')],
    fallback: 'một tài sản', domain: 'asset', tag: 'vừa duyệt',
  },
  asset_request_rejected: {
    by: (a, s) => [a, plain(' đã từ chối yêu cầu cấp '), s, plain('.')],
    alone: (s) => [plain('Yêu cầu cấp '), s, plain(' của bạn đã bị từ chối.')],
    fallback: 'một tài sản', domain: 'asset', tag: 'vừa từ chối',
  },
  asset_assigned: {
    by: (a, s) => [a, plain(' đã giao '), s, plain(' cho bạn.')],
    alone: (s) => [s, plain(' đã được giao cho bạn.')],
    fallback: 'một tài sản', domain: 'asset', tag: 'vừa giao',
  },
  asset_returned: {
    by: (a, s) => [a, plain(' đã nhận lại '), s, plain(' từ bạn.')],
    alone: (s) => [s, plain(' đã được nhận lại từ bạn.')],
    fallback: 'một tài sản', domain: 'asset', tag: 'vừa nhận lại',
  },
  workspace_invitation: {
    by: (a, s) => [a, plain(' đã mời bạn vào '), s, plain('.')],
    alone: (s) => [plain('Bạn được mời vào '), s, plain('.')],
    fallback: 'một workspace', domain: 'workspace', tag: 'vừa mời',
  },
}

/** The types that carry a reason in `params.reason`. */
const WITH_REASON = new Set(['approval_rejected'])

/** The sentence a notification reads as. An unknown type says only that there is news. */
export function sentenceOf(n: AppNotification): Sentence {
  const copy = COPY[n.type]
  if (!copy) return { lead: [plain('Có một thông báo mới.')] }
  const subject = strong(n.target_name?.trim() || copy.fallback)
  const actorName = n.actor_name?.trim()
  const lead = actorName ? copy.by(strong(actorName), subject) : copy.alone(subject)
  const reason = WITH_REASON.has(n.type) ? n.params?.reason?.trim() : undefined
  return { lead, ...(copy.follow ? { follow: copy.follow } : {}), ...(reason ? { reason } : {}) }
}

export const plainText = (pieces: Piece[]): string => pieces.map((p) => p.text).join('')

/** The short sentence for a toast: the lead, with no follow-up or reason. */
export const toastText = (n: AppNotification): string => plainText(sentenceOf(n).lead)

export function domainOf(type: string): NotificationDomain {
  return COPY[type]?.domain ?? 'other'
}

/** The "vừa …" tag a fresh arrival wears while its wash fades (DESIGN.md §7). */
export function arrivalTag(type: string): string {
  return COPY[type]?.tag ?? 'vừa có'
}

/** The accessible name of an entry point that carries an unread count: "Thông báo, 3 chưa đọc". */
export function unreadLabel(label: string, count: number, noun = ''): string {
  if (count <= 0) return label
  return `${label}, ${count} ${noun ? `${noun} ` : ''}chưa đọc`
}

/** One line for a burst: "3 thông báo mới · Lê Quang Vinh và 2 người khác". */
export function summaryText(count: number, actors: string[]): string {
  const names = [...new Set(actors.filter(Boolean))]
  const head = `${count} thông báo mới`
  if (names.length === 0) return head
  const others = names.length - 1
  return `${head} · ${names[0]}${others > 0 ? ` và ${others} người khác` : ''}`
}

/** Where a notification leads: a route and the search that opens the thing on it. */
export interface OpenTarget {
  to: '/approval' | '/assets' | '/workspace-select'
  search: Record<string, string>
}

/** The place that shows the notification's subject, or null when it names none. */
export function openTargetOf(n: AppNotification): OpenTarget | null {
  const id = n.target_id
  switch (n.target_type) {
    case 'approval':
      // The requester is the one who is told, so the request is under Của bạn.
      return id ? { to: '/approval', search: { tab: 'mine', request: id } } : null
    case 'asset_request':
      return id ? { to: '/assets', search: { section: 'requests', request: id } } : null
    case 'asset':
      return id ? { to: '/assets', search: { section: 'list', asset: id } } : null
    case 'workspace_invitation':
      return { to: '/workspace-select', search: {} }
    default:
      return null
  }
}

export interface DayGroup {
  label: 'Hôm nay' | 'Trước đó'
  items: AppNotification[]
}

/** Today's notifications, then the rest, in the order given; an empty group is left out. */
export function groupByDay(items: AppNotification[], now: Date = new Date()): DayGroup[] {
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const today: AppNotification[] = []
  const before: AppNotification[] = []
  for (const n of items) {
    const d = toDate(n.created_at)
    ;(d && d.getTime() >= startOfToday ? today : before).push(n)
  }
  const groups: DayGroup[] = []
  if (today.length) groups.push({ label: 'Hôm nay', items: today })
  if (before.length) groups.push({ label: 'Trước đó', items: before })
  return groups
}
