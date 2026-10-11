import { apiFetch } from './client'
import type { NotificationItem } from '../lib/notification-copy'

/** One page of `GET /api/notifications` (docs/specs/notifications). */
export interface NotificationPage {
  notifications: NotificationItem[]
  total: number
  unread_count: number
}

export const NOTIFICATION_PAGE_SIZE = 25

export const notificationApi = {
  list: (limit: number, offset: number) =>
    apiFetch<NotificationPage>(`/notifications?limit=${limit}&offset=${offset}`),
  unreadCount: () => apiFetch<{ count: number }>('/notifications/unread-count'),
  markRead: (id: string) =>
    apiFetch<{ status: string }>(`/notifications/${encodeURIComponent(id)}/read`, { method: 'POST' }),
  markAllRead: () => apiFetch<{ status: string }>('/notifications/read-all', { method: 'POST' }),
}
