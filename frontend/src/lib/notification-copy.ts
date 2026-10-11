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

export type NotificationDomain = 'approval' | 'asset'

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

export function describeNotification(n: Pick<NotificationItem, 'type'>): { text: string; domain: NotificationDomain | null } {
  return COPY[n.type] ?? { text: 'Bạn có một thông báo mới.', domain: null }
}

export const DOMAIN_LABEL: Record<NotificationDomain, string> = { approval: 'Phê duyệt', asset: 'Tài sản' }

export type NotificationTarget =
  | { to: '/approval'; search: { request: string } }
  | { to: '/assets'; search: { asset: string } | { section: 'requests'; request: string } }

/** Where a notification leads, or null when it names nothing the app can open. */
export function notificationTarget(n: Pick<NotificationItem, 'entity_type' | 'entity_id'>): NotificationTarget | null {
  const id = n.entity_id
  if (!id) return null
  if (n.entity_type === 'approval') return { to: '/approval', search: { request: id } }
  if (n.entity_type === 'asset') return { to: '/assets', search: { asset: id } }
  if (n.entity_type === 'asset_request') return { to: '/assets', search: { section: 'requests', request: id } }
  return null
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
