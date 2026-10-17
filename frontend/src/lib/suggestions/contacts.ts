import type { Contact } from '../../hooks/useContacts'
import type { Conversation } from '../conversations'
import { recentConversations } from './chat'

/** How many people Danh bạ suggests at most (DESIGN.md "Gợi ý trong ô tìm"). */
export const RECENTLY_MESSAGED = 5

/**
 * "Nhắn gần đây": people of this workspace's directory one has direct messages with, latest message first. Direct
 * messages are not tied to a workspace, so a partner outside `directory` is left out; so is the person using the app.
 */
export function recentlyMessaged(directs: Conversation[], directory: Contact[], meId: string | undefined, limit = RECENTLY_MESSAGED): Contact[] {
  const byId = new Map(directory.map((c) => [c.user_id, c]))
  const out: Contact[] = []
  for (const dm of recentConversations(directs, Infinity)) {
    const id = dm.partner?.userId
    const person = id && id !== meId ? byId.get(id) : undefined
    if (!person || out.includes(person)) continue
    out.push(person)
    if (out.length === limit) break
  }
  return out
}
