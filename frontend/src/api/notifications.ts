import { apiFetch } from './client'

/**
 * A notification as the server stores it: facts, not wording. The screen builds
 * the sentence from `type` and the names (lib/notification-model.ts). Names are
 * absent when unknown and are never an id; ids exist to open things and pick a
 * colour, and are never shown.
 */
export interface AppNotification {
  id: string
  type: string
  read: boolean
  created_at: string
  workspace_id: string
  actor_user_id?: string
  actor_name?: string
  target_type?: string
  target_id?: string
  target_name?: string
  params: Record<string, string>
}

export interface NotificationPage {
  notifications: AppNotification[]
  total: number
  unread_count: number
}

/** Notifications per page (the API's default and the design's). */
export const NOTIFICATION_PAGE = 25

export const notificationsApi = {
  list: (offset = 0, limit = NOTIFICATION_PAGE) =>
    apiFetch<NotificationPage>(`/notifications?limit=${limit}&offset=${offset}`),
  unreadCount: () => apiFetch<{ count: number }>('/notifications/unread-count'),
  markRead: (id: string) => apiFetch<{ status: string }>(`/notifications/${id}/read`, { method: 'POST' }),
  markAllRead: () => apiFetch<{ status: string }>('/notifications/read-all', { method: 'POST' }),
  /** Reads the caller's notifications about one subject, e.g. an invitation they have answered. */
  markAboutRead: (type: string, id: string) =>
    apiFetch<{ status: string }>('/notifications/read-about', { method: 'POST', body: JSON.stringify({ type, id }) }),
}
