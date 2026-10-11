import { apiFetch } from './client'
import type { Message } from './messaging'

/**
 * A message of the mentions or starred feed: the usual message plus what the
 * feed adds. Timestamps are `{seconds,nanos}` objects; flags the server omits
 * when false are absent here too.
 */
export interface FeedMessage extends Message {
  /** Starred feed: always true. */
  starred?: boolean
  /** Starred feed: when the caller starred it. */
  starred_at?: unknown
  /** Mentions feed: the caller has not read it. */
  unread?: boolean
}

export interface FeedPage { messages: FeedMessage[]; has_more: boolean }

/** A page cursor: the sort time of the last row held, as RFC 3339 with nanoseconds, and its id as tie-break. */
export interface FeedCursor { before: string; beforeId: string }

function feedPath(path: string, q: { cursor?: FeedCursor; limit: number }): string {
  const qs = new URLSearchParams({ limit: String(q.limit) })
  if (q.cursor?.before) qs.set('before', q.cursor.before)
  if (q.cursor?.beforeId) qs.set('before_id', q.cursor.beforeId)
  return `${path}?${qs}`
}

/** The server leaves empty fields out: an empty page is just `{}`. */
function toPage(page: Partial<FeedPage>): FeedPage {
  return { messages: page.messages ?? [], has_more: !!page.has_more }
}

export const shortcutsApi = {
  starConversation: (channelId: string) =>
    apiFetch<unknown>(`/channels/${channelId}/star`, { method: 'PUT' }),
  unstarConversation: (channelId: string) =>
    apiFetch<unknown>(`/channels/${channelId}/star`, { method: 'DELETE' }),
  starMessage: (messageId: string) =>
    apiFetch<unknown>(`/messages/${messageId}/star`, { method: 'PUT' }),
  unstarMessage: (messageId: string) =>
    apiFetch<unknown>(`/messages/${messageId}/star`, { method: 'DELETE' }),

  listStarred: (q: { cursor?: FeedCursor; limit: number }) =>
    apiFetch<Partial<FeedPage>>(feedPath('/messages/starred', q)).then(toPage),
  listMentions: (q: { cursor?: FeedCursor; limit: number }) =>
    apiFetch<Partial<FeedPage>>(feedPath('/messages/mentions', q)).then(toPage),
  /** Unread messages of the mentions feed, counted by the server over the feed's own scope. */
  mentionsUnreadCount: () =>
    apiFetch<{ count?: number }>('/messages/mentions/unread-count').then((r) => r.count ?? 0),
}
