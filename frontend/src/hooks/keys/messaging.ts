/**
 * Messaging query keys. Channel lists are per workspace; everything inside a
 * conversation is addressed by its channel or message id, which is already
 * unique, so those keys carry no workspace id.
 */
export const messagingKeys = {
  /** Prefix of every workspace's channel list. */
  channelsAll: () => ['channels'] as const,
  channels: (wsId: string) => ['channels', wsId] as const,
  /** Direct messages are not workspace-scoped on the backend. */
  dms: () => ['dms'] as const,
  /** Prefix of every channel's message list. */
  messagesAll: () => ['messages'] as const,
  /** One channel's own record (name, type), as opposed to its messages. */
  channel: (channelId: string) => ['channel', channelId] as const,
  messages: (channelId: string) => ['messages', channelId] as const,
  threadsAll: () => ['thread'] as const,
  thread: (messageId: string) => ['thread', messageId] as const,
  reactionsAll: () => ['reactions'] as const,
  reactions: (messageId: string) => ['reactions', messageId] as const,
  pinsAll: () => ['pins'] as const,
  pins: (channelId: string) => ['pins', channelId] as const,
  /** Per-channel unread counts for the signed-in user. */
  unreadCounts: () => ['unread-counts'] as const,
  search: (channelId: string, query: string) => ['search', channelId, query] as const,
  tasksAll: () => ['tasks'] as const,
  /** Prefix of a channel's task lists, whatever the status filter. */
  tasksOf: (channelId: string) => ['tasks', channelId] as const,
  tasks: (channelId: string, status?: string) => ['tasks', channelId, status] as const,
  members: (channelId: string) => ['channelMembers', channelId] as const,
}
