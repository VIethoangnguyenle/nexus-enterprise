import { unreadOf, type Conversation } from './conversations'

/**
 * A personal section of the chat navigator (docs/specs/chat-sections). It only
 * arranges one person's list: `channel_ids` are for placing rows, never shown.
 */
export interface ChatSection {
  id: string
  name: string
  position: number
  channel_ids: string[]
}

export const MAX_SECTION_NAME = 40

const lower = (s: string) => s.trim().toLocaleLowerCase('vi')

/**
 * Why a section name cannot be used, in words for the person, or null when it
 * can. `exceptId` is the section being renamed, which may keep its own name.
 * The server repeats these checks; this only answers before the round trip.
 */
export function validateSectionName(raw: string, sections: ChatSection[], exceptId?: string): string | null {
  const name = raw.trim()
  if (!name) return 'Nhập tên mục'
  if (Array.from(name).length > MAX_SECTION_NAME) return `Tên mục tối đa ${MAX_SECTION_NAME} ký tự`
  if (sections.some((s) => s.id !== exceptId && lower(s.name) === lower(name))) return 'Bạn đã có mục tên này'
  return null
}

export interface NavSection {
  id: string
  name: string
  items: Conversation[]
  /** Unread in the whole section, by notification level; the header shows it while collapsed. */
  unread: number
}

export interface Arranged {
  /** Starred conversations: listed here only, whatever section they belong to on the server. */
  starred: Conversation[]
  sections: NavSection[]
  /** Direct messages that sit in no section. */
  directs: Conversation[]
  /** Spaces that sit in no section. */
  spaces: Conversation[]
}

const byPosition = (a: ChatSection, b: ChatSection) => a.position - b.position

/**
 * Splits the conversations between the user's sections and the two default
 * lists. A conversation is listed once: starred, else in the first section
 * that claims it, else in its default list (a starred one keeps its section on
 * the server and goes back to it when unstarred). Conversations keep the order
 * they came in.
 */
export function arrange(all: Conversation[], sections: ChatSection[]): Arranged {
  // Newest star first; one with no known time goes last (the sort is stable).
  const starred = all.filter((c) => c.starred).sort((a, b) => (b.starredAt ?? 0) - (a.starredAt ?? 0))
  const conversations = all.filter((c) => !c.starred)
  const taken = new Set<string>()
  const out: NavSection[] = [...sections].sort(byPosition).map((s) => {
    const claimed = new Set(s.channel_ids)
    const items = conversations.filter((c) => claimed.has(c.id) && !taken.has(c.id))
    items.forEach((c) => taken.add(c.id))
    return { id: s.id, name: s.name, items, unread: items.reduce((n, c) => n + unreadOf(c), 0) }
  })
  const rest = conversations.filter((c) => !taken.has(c.id))
  return {
    starred,
    sections: out,
    directs: rest.filter((c) => c.kind === 'dm'),
    spaces: rest.filter((c) => c.kind === 'space'),
  }
}

export function sectionOfChannel(sections: ChatSection[], channelId: string): ChatSection | undefined {
  return sections.find((s) => s.channel_ids.includes(channelId))
}

// Optimistic edits of the cached list. Each mirrors what the server does, so a
// refetch after the call settles changes nothing the person has already seen.

export function withRenamed(sections: ChatSection[], id: string, name: string): ChatSection[] {
  return sections.map((s) => (s.id === id ? { ...s, name } : s))
}

export function withMoved(sections: ChatSection[], id: string, dir: 'up' | 'down'): ChatSection[] {
  const ordered = [...sections].sort(byPosition)
  const i = ordered.findIndex((s) => s.id === id)
  const j = dir === 'up' ? i - 1 : i + 1
  if (i < 0 || j < 0 || j >= ordered.length) return sections
  const a = ordered[i]!
  const b = ordered[j]!
  ordered[i] = { ...b, position: a.position }
  ordered[j] = { ...a, position: b.position }
  return ordered
}

export function withUnplaced(sections: ChatSection[], channelId: string): ChatSection[] {
  return sections.map((s) =>
    s.channel_ids.includes(channelId) ? { ...s, channel_ids: s.channel_ids.filter((c) => c !== channelId) } : s,
  )
}

export function withPlaced(sections: ChatSection[], id: string, channelId: string): ChatSection[] {
  return withUnplaced(sections, channelId).map((s) =>
    s.id === id ? { ...s, channel_ids: [...s.channel_ids, channelId] } : s,
  )
}

export function withoutSection(sections: ChatSection[], id: string): ChatSection[] {
  return sections.filter((s) => s.id !== id)
}

/** Puts a section back at its position, with its conversations (Undo, or a create that names them). */
export function withRestored(sections: ChatSection[], section: ChatSection): ChatSection[] {
  const rest = section.channel_ids.reduce((acc, c) => withUnplaced(acc, c), withoutSection(sections, section.id))
  const ordered = [...rest].sort(byPosition)
  const at = ordered.filter((s) => s.position < section.position).length
  ordered.splice(at, 0, section)
  return ordered
}
