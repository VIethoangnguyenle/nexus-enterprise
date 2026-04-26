import { apiFetch } from './client'

export interface Notification { id: string; type: string; title?: string; body?: string; message?: string; read: boolean; entity_type?: string; entity_id?: string; created_at?: string }

export const notificationApi = {
  list: (limit = 10) => apiFetch<{ notifications: Notification[] }>(`/notifications?limit=${limit}`),
  unreadCount: () => apiFetch<{ count: number }>('/notifications/unread-count'),
  markRead: (id: string) => apiFetch(`/notifications/${id}/read`, { method: 'POST' }),
  markAllRead: () => apiFetch('/notifications/read-all', { method: 'POST' }),
}
