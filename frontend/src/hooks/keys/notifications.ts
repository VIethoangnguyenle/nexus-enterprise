/** Notifications of the signed-in user; the unread count sits under `all()`. */
export const notificationKeys = {
  all: () => ['notifications'] as const,
  list: (limit: number) => ['notifications', 'list', limit] as const,
  unreadCount: () => ['notifications', 'unread-count'] as const,
}
