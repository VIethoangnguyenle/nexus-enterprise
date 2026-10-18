import type { Conversation } from '../conversations'
import type { Person } from '../people'
import { matchesAll } from '../search-text'

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

/** How many people, and how many groups and conversations, the suggestions for typed words show. */
export const TYPED_LIMIT = 3

/**
 * "Người": the people in the directory whose name or username holds every typed word (case and accents ignored),
 * never the person asking. The directory is the list the caller may browse, so nobody it hides is offered.
 */
export function peopleMatching(people: Person[], query: string, meId: string, limit = TYPED_LIMIT): Person[] {
  return people
    .filter((p) => p.userId !== meId && (matchesAll(p.name, query) || matchesAll(p.username, query)))
    .slice(0, limit)
}

/**
 * "Nhóm và trò chuyện": the conversations of the list the server gave the caller whose name holds every typed word.
 * A direct message with someone already listed under "Người" (`skipPartners`) is not repeated.
 */
export function conversationsMatching(
  list: Conversation[], query: string, skipPartners: ReadonlySet<string> = new Set(), limit = TYPED_LIMIT,
): Conversation[] {
  return list
    .filter((c) => matchesAll(c.title, query) && !(c.partner && skipPartners.has(c.partner.userId)))
    .slice(0, limit)
}
