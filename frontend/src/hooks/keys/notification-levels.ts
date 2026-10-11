/**
 * Notification-level query keys: one conversation's own level, and the
 * Trang chủ list of followed threads with unread replies.
 */
export const notificationLevelKeys = {
  /** The signed-in user's level for one conversation. */
  level: (channelId: string) => ['notification-level', channelId] as const,
  /** Followed threads with unread replies, across every unmuted conversation. */
  unreadThreads: () => ['unread-threads'] as const,
}
