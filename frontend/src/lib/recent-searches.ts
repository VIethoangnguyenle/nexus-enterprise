import { fold } from './search-text'

export const MAX_RECENT = 8
const MIN_CHARS = 2
const keyOf = (userId: string) => `ngac-chat-recent-searches:${userId}`

/** What this person searched for lately in Chat on this device, newest first. Storage can be blocked: then nothing. */
export function readRecent(userId: string): string[] {
  try {
    const raw: unknown = JSON.parse(localStorage.getItem(keyOf(userId)) ?? '[]')
    if (!Array.isArray(raw)) return []
    return raw.filter((x): x is string => typeof x === 'string' && x.trim().length > 0).slice(0, MAX_RECENT)
  } catch {
    return []
  }
}

function write(userId: string, list: string[]): string[] {
  try {
    localStorage.setItem(keyOf(userId), JSON.stringify(list))
  } catch {
    // Not remembering is the only consequence.
  }
  return list
}

/** Adds a search at the front; one that differs only in case or accents replaces the older one. Returns the new list. */
export function addRecent(userId: string, query: string): string[] {
  const q = query.trim().replace(/\s+/g, ' ')
  const current = readRecent(userId)
  if (q.length < MIN_CHARS) return current
  const same = fold(q)
  return write(userId, [q, ...current.filter((x) => fold(x) !== same)].slice(0, MAX_RECENT))
}

export function removeRecent(userId: string, query: string): string[] {
  return write(userId, readRecent(userId).filter((x) => x !== query))
}
