/**
 * Query keys of the navigator shortcuts. The feeds belong to the signed-in user in
 * the workspace they are in, so each carries the workspace id; the `*All` prefixes
 * address one kind of feed whatever the workspace.
 */
export const shortcutKeys = {
  mentionsAll: () => ['shortcut-feed', 'mentions'] as const,
  mentions: (wsId: string) => ['shortcut-feed', 'mentions', wsId] as const,
  starredMessagesAll: () => ['shortcut-feed', 'starred'] as const,
  starredMessages: (wsId: string) => ['shortcut-feed', 'starred', wsId] as const,
  /** Unread messages of the mentions feed: the pill on the "Được nhắc tới" shortcut. */
  mentionsUnreadAll: () => ['shortcut-mentions-unread'] as const,
  mentionsUnread: (wsId: string) => ['shortcut-mentions-unread', wsId] as const,
}
