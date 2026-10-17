import { fold } from './search-text'

/** The modules whose search box remembers what was searched. Each keeps its own list. */
export type SearchModule = 'chat' | 'drive' | 'contacts' | 'assets' | 'approval'

export const MAX_RECENT = 8
const MIN_CHARS = 2
const KEY_PREFIX = 'ngac-recent-searches:'
/** Where Chat kept its searches before every module had a list; read once, then moved. */
const LEGACY_CHAT_PREFIX = 'ngac-chat-recent-searches:'
const keyOf = (module: SearchModule, userId: string) => `${KEY_PREFIX}${module}:${userId}`

/** Only strings that hold a word; anything else in storage is ignored. */
function parse(raw: string | null): string[] {
  const list: unknown = JSON.parse(raw ?? '[]')
  if (!Array.isArray(list)) return []
  return list.filter((x): x is string => typeof x === 'string' && x.trim().length > 0).slice(0, MAX_RECENT)
}

/**
 * Chat's searches from before the per-module lists. The old key is removed only once the new one is written, so a
 * full or blocked storage loses nothing; a list already kept in the new place wins over the old one.
 */
function moveLegacyChat(userId: string): string[] | null {
  const legacy = LEGACY_CHAT_PREFIX + userId
  const raw = localStorage.getItem(legacy)
  if (raw === null) return null
  let list: string[]
  try {
    list = parse(raw)
  } catch {
    // Unreadable: there is nothing to move, and leaving it would hide the new list on every read.
    list = []
  }
  try {
    if (localStorage.getItem(keyOf('chat', userId)) === null) localStorage.setItem(keyOf('chat', userId), JSON.stringify(list))
    localStorage.removeItem(legacy)
  } catch {
    // Not moved this time; read again from the old place next time.
  }
  return list
}

/** What this person searched for lately in `module` on this device, newest first. Storage can be blocked: then nothing. */
export function readRecent(module: SearchModule, userId: string): string[] {
  try {
    const legacy = module === 'chat' ? moveLegacyChat(userId) : null
    const raw = localStorage.getItem(keyOf(module, userId))
    return raw === null ? legacy ?? [] : parse(raw)
  } catch {
    return []
  }
}

function write(module: SearchModule, userId: string, list: string[]): string[] {
  try {
    localStorage.setItem(keyOf(module, userId), JSON.stringify(list))
  } catch {
    // Not remembering is the only consequence.
  }
  return list
}

/** Adds a search at the front; one that differs only in case or accents replaces the older one. Returns the new list. */
export function addRecent(module: SearchModule, userId: string, query: string): string[] {
  const q = query.trim().replace(/\s+/g, ' ')
  const current = readRecent(module, userId)
  if (q.length < MIN_CHARS) return current
  const same = fold(q)
  return write(module, userId, [q, ...current.filter((x) => fold(x) !== same)].slice(0, MAX_RECENT))
}

export function removeRecent(module: SearchModule, userId: string, query: string): string[] {
  return write(module, userId, readRecent(module, userId).filter((x) => x !== query))
}

/** Forgets every person's recent searches in every module on this device; run at sign-out so the next person on a shared device cannot read them. */
export function clearAllRecent(): void {
  try {
    const keys: string[] = []
    for (let i = 0; i < localStorage.length; i++) {
      const k = localStorage.key(i)
      if (k?.startsWith(KEY_PREFIX) || k?.startsWith(LEGACY_CHAT_PREFIX)) keys.push(k)
    }
    keys.forEach((k) => localStorage.removeItem(k))
  } catch {
    // Storage blocked: there is nothing stored to clear.
  }
}
