import type { Contact } from '../hooks/useContacts'
import type { ResolvedEntry, ResolverSnapshot } from './people-resolver'

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

/**
 * Everyone the UI can name. `list` is the people the signed-in user may browse: pickers, mentions
 * and suggestions read it, so they never offer anyone the directory hides. `byUserId` and
 * `byNodeId` also hold people found by the resolver (hidden, removed) so that a message from one
 * of them still carries a name; those are not in `list` and `isVisible` is false for them.
 */
export interface PeopleDirectory {
  list: Person[]
  byUserId: Map<string, Person>
  byNodeId: Map<string, Person>
  byUsername: Map<string, Person>
  /**
   * False until the contacts have loaded, and for good if they fail. A directory that is not ready
   * knows nobody, so "not listed" then means "not known yet", never "hidden".
   */
  ready: boolean
  /** True for a person the directory lists, false for everyone else (hidden, removed, unknown). */
  isVisible: (userId: string) => boolean
  /** Asks for a name the directory lacks. Only queues; the answer arrives with a new directory. */
  request: (userId: string) => void
  /** The same for a person known only by NGAC node id. */
  requestNode: (nodeId: string) => void
}

export interface DirectoryOptions {
  /** Departments belong to company workspaces; a personal workspace passes `departments: false`. */
  departments?: boolean
  /** People the resolver has found, kept apart from the listed ones. */
  resolved?: ResolverSnapshot
  /** Queues a user id for the resolver; without it a miss stays a miss. */
  request?: (userId: string) => void
  /** Queues a node id for the resolver. */
  requestNode?: (nodeId: string) => void
}

const noRequest = () => {}

/**
 * A map of people that asks for a key it lacks. The ids in content (a mention, an approver, a
 * file's owner) are looked up straight in `byUserId`/`byNodeId` all over the app; this makes each
 * such lookup of a hidden or removed person queue a name request, so none of those call sites has
 * to know the resolver exists. `get` only queues: it never fetches while rendering, and an id
 * that was answered (found or not) is not asked for again. `peek` reads without asking.
 */
class AskingMap extends Map<string, Person> {
  constructor(private readonly ask: (id: string) => void) { super() }
  override get(key: string): Person | undefined {
    const hit = super.get(key)
    if (!hit && key) this.ask(key)
    return hit
  }
  peek(key: string): Person | undefined { return super.get(key) }
}

export function personFromContact(c: Contact, { departments = true }: DirectoryOptions = {}): Person {
  return {
    userId: c.user_id,
    nodeId: c.ngac_node_id,
    username: c.username,
    name: c.display_name?.trim() || UNKNOWN_PERSON,
    role: [c.title, departments ? c.department : ''].filter(Boolean).join(' · '),
    avatarUrl: c.avatar_url || '',
  }
}

function personFromResolved(r: ResolvedEntry): Person {
  return { userId: r.userId, nodeId: r.nodeId, username: '', name: r.name || UNKNOWN_PERSON, role: '', avatarUrl: r.avatarUrl }
}

export function buildDirectory(contacts: Contact[], options: DirectoryOptions = {}): PeopleDirectory {
  const list = contacts.map((c) => personFromContact(c, options))
  const byUserId = new AskingMap(options.request ?? noRequest)
  const byNodeId = new AskingMap(options.requestNode ?? noRequest)
  const byUsername = new Map<string, Person>()
  // Resolved people first, so a listed person with the same id replaces them.
  for (const r of options.resolved?.byUserId.values() ?? []) byUserId.set(r.userId, personFromResolved(r))
  for (const r of options.resolved?.byNodeId.values() ?? []) byNodeId.set(r.nodeId, personFromResolved(r))
  const visible = new Set<string>()
  for (const p of list) {
    if (p.userId) { byUserId.set(p.userId, p); visible.add(p.userId) }
    if (p.nodeId) byNodeId.set(p.nodeId, p)
    if (p.username) byUsername.set(p.username.toLowerCase(), p)
  }
  return {
    list, byUserId, byNodeId, byUsername, ready: true, isVisible: (id) => visible.has(id),
    request: options.request ?? noRequest, requestNode: options.requestNode ?? noRequest,
  }
}

export const EMPTY_DIRECTORY: PeopleDirectory = { ...buildDirectory([]), ready: false }

/** Shown when nobody can tell us who someone is. Never an id. */
export const UNKNOWN_PERSON = 'Thành viên'

/** The directory's person for an id or username; a miss on an id is queued for the resolver. */
function findPerson(dir: PeopleDirectory, userId?: string, username?: string): Person | undefined {
  // `peek` where the map has it: the miss is queued below only when the username does not name them either.
  const peek = (m: Map<string, Person>, k: string) => (m instanceof AskingMap ? m.peek(k) : m.get(k))
  const byId = userId ? peek(dir.byUserId, userId) : undefined
  if (byId) return byId
  const byName = username ? dir.byUsername.get(username.toLowerCase()) : undefined
  if (!byName && userId) dir.request(userId)
  return byName
}

/**
 * Display name for a message author or member: directory first, then the
 * username the API sent, then a neutral word. An id is never a fallback.
 */
export function displayName(dir: PeopleDirectory, userId?: string, username?: string): string {
  const p = findPerson(dir, userId, username)
  if (p) return p.name
  return username?.trim() || UNKNOWN_PERSON
}


/** The person's picture, found the way displayName finds their name; undefined when they have none. */
export function avatarOf(dir: PeopleDirectory, userId?: string, username?: string): string | undefined {
  const p = findPerson(dir, userId, username)
  return (p && p.avatarUrl) || undefined
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
