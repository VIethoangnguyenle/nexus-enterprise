/**
 * Query keys of the cards a chat message can carry. A poll is addressed by its
 * own id: each viewer's copy differs (counts appear only after they vote), so
 * the key never carries anything but the poll.
 */
export const chatCardKeys = {
  poll: (pollId: string) => ['messaging', 'poll', pollId] as const,
  /** The folder a conversation's files live in, found through its drive. */
  channelFolder: (wsId: string, channelId: string) => ['channel-folder', wsId, channelId] as const,
}
