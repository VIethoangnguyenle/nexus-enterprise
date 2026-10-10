/**
 * The signed-in person's notifications. The list and the unread count are
 * scoped to the workspace in the session token, and switching workspace clears
 * the whole cache, so these carry no workspace id. `all()` prefixes both: the
 * WebSocket `notification` event and the resync invalidate it.
 */
export const notificationKeys = {
  all: () => ['notifications'] as const,
  /** The paged list (an infinite query: every loaded page lives under this key). */
  list: () => ['notifications', 'list'] as const,
  unreadCount: () => ['notifications', 'unread-count'] as const,
}
