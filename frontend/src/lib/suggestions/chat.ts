import type { Conversation } from '../conversations'

/** How many conversations Tìm trong chat suggests at most (DESIGN.md "Gợi ý trong ô tìm"). */
export const RECENT_CONVERSATIONS = 5

/**
 * "Trò chuyện gần đây": the conversations something was said in lately, newest first. Only the list the server
 * gave the caller is ever passed in, so a conversation they can no longer read is never suggested.
 */
export function recentConversations(list: Conversation[], limit = RECENT_CONVERSATIONS): Conversation[] {
  return list
    .filter((c) => c.lastActivity > 0)
    .sort((a, b) => b.lastActivity - a.lastActivity)
    .slice(0, limit)
}
