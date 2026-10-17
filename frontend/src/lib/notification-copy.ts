import { formatDate, formatListTime, formatRelative, formatTime, toDate } from './format'

/** One notification as the messaging service lists it (docs/specs/notifications). */
export interface NotificationItem {
  id: string
  type: string
  title: string
  body: string
  read: boolean
  created_at: string
  entity_type?: string
  entity_id?: string
}

export type NotificationDomain = 'approval' | 'asset' | 'invitation' | 'calendar'

/**
 * What a notification says, built here from its `type`. The server's `title`
 * and `body` are English strings with the object's id in them, so they are
 * never shown. The list carries no name for the person who acted or for the
 * object, so the sentences name neither; see the NEEDS_CONTEXT in the
 * app-rail report.
 */
const COPY: Record<string, { text: string; domain: NotificationDomain }> = {
  approval_approved: { text: 'Một đề nghị của bạn đã được duyệt.', domain: 'approval' },
  approval_rejected: { text: 'Một đề nghị của bạn đã bị trả lại.', domain: 'approval' },
  asset_request_approved: { text: 'Yêu cầu cấp tài sản của bạn đã được duyệt.', domain: 'asset' },
  asset_request_rejected: { text: 'Yêu cầu cấp tài sản của bạn đã bị từ chối.', domain: 'asset' },
  asset_assigned: { text: 'Bạn được giao một tài sản.', domain: 'asset' },
  asset_returned: { text: 'Một tài sản đã được nhận lại từ bạn.', domain: 'asset' },
  asset_request: { text: 'Bạn đã gửi yêu cầu cấp tài sản. Đang chờ duyệt.', domain: 'asset' },
  asset_lifecycle: { text: 'Bạn đã đổi trạng thái của một tài sản.', domain: 'asset' },
}

/**
 * An invitation is the one type whose server `body` is shown: the messaging
 * service writes it as "<người mời> mời bạn vào <workspace>", names only and no
 * identifier, and the list has no other way to name them.
 */
const INVITATION_FALLBACK = 'Bạn có một lời mời vào workspace.'

/**
 * What a calendar notification's `body` holds (docs/specs/notifications): JSON of names and times for the sentence
 * to be written from here, in the viewer's own clock. `ws` only builds the link; it is never printed.
 */
export interface CalendarBody {
  ws?: string
  actor?: string
  start?: string
  end?: string
  allDay?: boolean
  occurrence?: string
  response?: string
}

const asText = (v: unknown) => (typeof v === 'string' && v.trim() ? v.trim() : undefined)

/** The body of a calendar notification, or an empty one when it is not what the server writes. */
export function parseCalendarBody(body: string | undefined): CalendarBody {
  if (!body) return {}
  try {
    const raw: unknown = JSON.parse(body)
    if (!raw || typeof raw !== 'object') return {}
    const r = raw as Record<string, unknown>
    return {
      ws: asText(r.ws), actor: asText(r.actor), start: asText(r.start), end: asText(r.end),
      allDay: r.allDay === true, occurrence: asText(r.occurrence), response: asText(r.response),
    }
  } catch {
    return {}
  }
}

const ANSWER: Record<string, string> = { accepted: 'đã nhận lời', declined: 'đã từ chối', tentative: 'có thể tham gia' }

/** The sentence of a calendar notification: the host or guest by name, the event by title, the time by the viewer's clock. */
function calendarText(type: string, title: string | undefined, b: CalendarBody): string {
  const what = asText(title) ?? 'một sự kiện'
  switch (type) {
    case 'calendar_invitation':
      return b.actor ? `${b.actor} mời bạn tới ${what}` : `Bạn được mời tới ${what}`
    case 'calendar_rsvp':
      return `${b.actor ?? 'Một khách'} ${ANSWER[b.response ?? ''] ?? 'đã trả lời'} ${what}`
    case 'calendar_reminder': {
      if (b.allDay) return `${what} diễn ra cả ngày hôm nay`
      const at = b.start ? formatTime(b.start) : ''
      return at ? `${what} bắt đầu lúc ${at}` : `${what} sắp bắt đầu`
    }
    case 'calendar_updated':
      return b.actor ? `${b.actor} đã dời ${what}` : `${what} đã được dời`
    case 'calendar_cancelled':
      return b.actor ? `${b.actor} đã huỷ ${what}` : `${what} đã bị huỷ`
    default:
      return b.actor ? `${b.actor} đã chia sẻ lịch với bạn` : 'Một lịch đã được chia sẻ với bạn'
  }
}

const CALENDAR_TYPES = new Set([
  'calendar_invitation', 'calendar_rsvp', 'calendar_reminder', 'calendar_updated', 'calendar_cancelled', 'calendar_shared',
])

export function describeNotification(n: Pick<NotificationItem, 'type'> & { title?: string; body?: string }): { text: string; domain: NotificationDomain | null } {
  if (n.type === 'workspace_invitation') return { text: n.body?.trim() || INVITATION_FALLBACK, domain: 'invitation' }
  if (CALENDAR_TYPES.has(n.type)) return { text: calendarText(n.type, n.title, parseCalendarBody(n.body)), domain: 'calendar' }
  return COPY[n.type] ?? { text: 'Bạn có một thông báo mới.', domain: null }
}

export const DOMAIN_LABEL: Record<NotificationDomain, string> = {
  approval: 'Phê duyệt', asset: 'Tài sản', invitation: 'Lời mời', calendar: 'Lịch',
}

export type NotificationTarget =
  | { to: '/approval'; search: { request: string } }
  | { to: '/assets'; search: { asset: string } | { section: 'requests'; request: string } }
  | { to: '/workspace-select' }
  | { to: '/calendar'; search: { mode: 'week'; date?: string; ws?: string } }

/** Where a notification leads, or null when it names nothing the app can open. */
export function notificationTarget(n: Pick<NotificationItem, 'entity_type' | 'entity_id'> & { type?: string; body?: string }): NotificationTarget | null {
  if (n.type === 'workspace_invitation') return { to: '/workspace-select' }
  if (n.type && CALENDAR_TYPES.has(n.type)) return calendarTarget(parseCalendarBody(n.body))
  const id = n.entity_id
  if (!id) return null
  if (n.entity_type === 'approval') return { to: '/approval', search: { request: id } }
  if (n.entity_type === 'asset') return { to: '/assets', search: { asset: id } }
  if (n.entity_type === 'asset_request') return { to: '/assets', search: { section: 'requests', request: id } }
  return null
}

const pad2 = (n: number) => String(n).padStart(2, '0')

/** The week of the event's day, on the viewer's calendar; the current week when the notification names no day. */
function calendarTarget(b: CalendarBody): NotificationTarget {
  const d = b.start ? new Date(b.start) : null
  const search: { mode: 'week'; date?: string; ws?: string } = { mode: 'week' }
  if (d && !Number.isNaN(d.getTime())) search.date = `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
  if (b.ws) search.ws = b.ws
  return { to: '/calendar', search }
}

const sameDay = (a: Date, b: Date) =>
  a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate()

/** "5 phút trước" within the hour, "09:14" later today, "Hôm qua 16:05", "07/10", "31/12/2025". */
export function notificationTime(value: unknown, now: Date = new Date()): string {
  const d = toDate(value)
  if (!d) return ''
  if (sameDay(d, now)) {
    const minutes = Math.floor((now.getTime() - d.getTime()) / 60_000)
    return minutes < 60 ? formatRelative(d, now) : formatTime(d)
  }
  const stamp = formatListTime(d, now)
  return stamp === 'Hôm qua' ? `Hôm qua ${formatTime(d)}` : stamp || formatDate(d)
}

export interface NotificationGroup { label: 'Hôm nay' | 'Trước đó'; items: NotificationItem[] }

/** "Hôm nay" then "Trước đó", in the order given, by the device's calendar day. */
export function groupByDay(items: NotificationItem[], now: Date = new Date()): NotificationGroup[] {
  const today = items.filter((n) => { const d = toDate(n.created_at); return !!d && sameDay(d, now) })
  const before = items.filter((n) => !today.includes(n))
  return [
    ...(today.length ? [{ label: 'Hôm nay' as const, items: today }] : []),
    ...(before.length ? [{ label: 'Trước đó' as const, items: before }] : []),
  ]
}
