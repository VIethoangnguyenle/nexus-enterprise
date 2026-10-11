import { apiFetch } from './client'
import type { Message } from './messaging'

/** `all` every message, `main` main messages and followed threads (default), `off` nothing. */
export type NotificationLevel = 'all' | 'main' | 'off'

export const DEFAULT_LEVEL: NotificationLevel = 'main'

/** Root messages whose followed thread has unread replies, newest reply first. */
export interface UnreadThreadsAnswer { messages: Message[] }

export const notificationLevelApi = {
  getNotificationLevel: (channelId: string) =>
    apiFetch<{ level: NotificationLevel }>(`/channels/${channelId}/notification-level`),
  setNotificationLevel: (channelId: string, level: NotificationLevel) =>
    apiFetch<{ level: NotificationLevel }>(`/channels/${channelId}/notification-level`, {
      method: 'PUT',
      body: JSON.stringify({ level }),
    }),
  unreadThreads: (limit: number) =>
    apiFetch<UnreadThreadsAnswer>(`/threads/unread?limit=${encodeURIComponent(String(limit))}`),
}
