import { CHAT_TABS, type ChatTab } from './nav-destinations'

/** The chip of the phone Chat list this person last chose, kept on this device and apart per person. */
const keyOf = (userId: string) => `ngac-chat-chip:${userId}`

const listeners = new Set<() => void>()

export function readChatChip(userId: string): ChatTab {
  try {
    const stored = localStorage.getItem(keyOf(userId))
    return (CHAT_TABS as readonly unknown[]).includes(stored) ? (stored as ChatTab) : 'all'
  } catch {
    return 'all'
  }
}

/** Storage can be blocked or full; the choice then still holds for this visit through the address, and is forgotten after. */
export function writeChatChip(userId: string, chip: ChatTab): void {
  try {
    localStorage.setItem(keyOf(userId), chip)
  } catch {
    // Not remembering is the only consequence.
  }
  listeners.forEach((l) => l())
}

export function subscribeChatChip(listener: () => void): () => void {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}
