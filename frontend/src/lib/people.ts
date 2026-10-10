import type { Contact } from '../hooks/useContacts'

/**
 * A person as the UI knows them. `userId` and `nodeId` exist to call APIs and
 * to pick a colour; they are never rendered. `name` is the display name.
 */
export interface Person {
  userId: string
  nodeId: string
  username: string
  name: string
  /** "Trưởng phòng · Vận hành thanh toán": title and department, either may be empty. */
  role: string
  avatarUrl: string
}

export interface PeopleDirectory {
  list: Person[]
  byUserId: Map<string, Person>
  byNodeId: Map<string, Person>
  byUsername: Map<string, Person>
}

export function personFromContact(c: Contact): Person {
  return {
    userId: c.user_id,
    nodeId: c.ngac_node_id,
    username: c.username,
    name: c.display_name?.trim() || UNKNOWN_PERSON,
    role: [c.title, c.department].filter(Boolean).join(' · '),
    avatarUrl: c.avatar_url || '',
  }
}

export function buildDirectory(contacts: Contact[]): PeopleDirectory {
  const list = contacts.map(personFromContact)
  const byUserId = new Map<string, Person>()
  const byNodeId = new Map<string, Person>()
  const byUsername = new Map<string, Person>()
  for (const p of list) {
    if (p.userId) byUserId.set(p.userId, p)
    if (p.nodeId) byNodeId.set(p.nodeId, p)
    if (p.username) byUsername.set(p.username.toLowerCase(), p)
  }
  return { list, byUserId, byNodeId, byUsername }
}

export const EMPTY_DIRECTORY: PeopleDirectory = buildDirectory([])

/** Shown when nobody can tell us who someone is. Never an id. */
export const UNKNOWN_PERSON = 'Thành viên'

/**
 * Display name for a message author or member: directory first, then the
 * username the API sent, then a neutral word. An id is never a fallback.
 */
export function displayName(dir: PeopleDirectory, userId?: string, username?: string): string {
  const p = (userId && dir.byUserId.get(userId)) || (username && dir.byUsername.get(username.toLowerCase()))
  if (p) return p.name
  return username?.trim() || UNKNOWN_PERSON
}

/** Case- and accent-insensitive match for people search ("duc" finds "Đức"). */
export function normalize(s: string): string {
  return s
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .replace(/đ/g, 'd')
    .replace(/Đ/g, 'D')
    .toLowerCase()
    .trim()
}

export function matchesPerson(p: Person, query: string): boolean {
  const q = normalize(query)
  if (!q) return true
  return normalize(p.name).includes(q) || normalize(p.role).includes(q) || normalize(p.username).includes(q)
}
