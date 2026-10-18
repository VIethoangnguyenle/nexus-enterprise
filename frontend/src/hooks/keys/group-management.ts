import { messagingKeys } from './messaging'

/**
 * Group management keys. The management record sits under the channel's own
 * key, so every invalidation of a channel (any realtime channel event,
 * `management_changed` included) refreshes it too.
 */
export const groupManagementKeys = {
  management: (channelId: string) => [...messagingKeys.channel(channelId), 'management'] as const,
}
