import { validateWorkspaceSearch } from './workspace'

/**
 * The categories of Danh bạ. `org` is Phòng ban, `members` is a personal workspace's flat list and
 * `search` is the phone's search screen (the pill of the top bar opens it).
 */
export const CONTACTS_CATS = ['org', 'roles', 'groups', 'members', 'search'] as const
export type ContactsCat = (typeof CONTACTS_CATS)[number]

/**
 * What the Danh bạ URL carries: the workspace (`ws`), the open category (`cat`) and the place inside it
 * (`dept`, `role` or `chat`), the open profile (`person`) and the search (`q`). Ids are here and not on
 * screen; keeping the place in the address is what makes back, forward and links land where the person was,
 * and what gives the phone one history entry per level.
 */
export interface ContactsSearch {
  ws?: string
  cat?: ContactsCat
  dept?: string
  role?: string
  /** The chat group that is open in Nhóm của tôi. Not `group`: Tài liệu's `?group=` has other values. */
  chat?: string
  person?: string
  q?: string
}

const text = (v: unknown): string | undefined => (typeof v === 'string' && v ? v : undefined)

/**
 * `validateSearch` for the Danh bạ route. Unknown or malformed values are dropped rather than rejected, so a
 * hand-edited link opens the default category instead of an error page. Only the place that belongs to the
 * category is kept; a place with no category names its own (a `role` is the Vai trò category).
 */
export function validateContactsSearch(search: Record<string, unknown>): ContactsSearch {
  const out: ContactsSearch = { ...validateWorkspaceSearch(search) }
  const dept = text(search.dept)
  const role = text(search.role)
  const chat = text(search.chat)
  let cat = (CONTACTS_CATS as readonly unknown[]).includes(search.cat) ? (search.cat as ContactsCat) : undefined
  if (!cat) cat = chat ? 'groups' : role ? 'roles' : dept ? 'org' : undefined
  if (cat) out.cat = cat
  if (cat === 'org' && dept) out.dept = dept
  if (cat === 'roles' && role) out.role = role
  if (cat === 'groups' && chat) out.chat = chat
  const person = text(search.person)
  if (person) out.person = person
  const q = text(search.q)
  if (q) out.q = q
  return out
}

/** The parts of the address that follow the person around: the workspace, and the profile that is open. */
const keep = (prev: ContactsSearch): ContactsSearch => ({
  ...(prev.ws ? { ws: prev.ws } : {}),
  ...(prev.person ? { person: prev.person } : {}),
})

/** The first screen of the phone: no category, no place, no search; only the workspace. */
export const homeSearch = (prev: ContactsSearch): ContactsSearch => (prev.ws ? { ws: prev.ws } : {})

/** A category at its own top. The search is forgotten; the profile that is open stays. */
export const categorySearch = (prev: ContactsSearch, cat: ContactsCat): ContactsSearch => ({
  ...keep(prev), cat,
})

/** A department (the organisation's root when `deptId` is absent), inside Phòng ban. */
export const deptSearch = (prev: ContactsSearch, deptId: string | undefined): ContactsSearch => ({
  ...keep(prev), cat: 'org', ...(deptId ? { dept: deptId } : {}),
})

export const roleSearch = (prev: ContactsSearch, roleId: string): ContactsSearch => ({
  ...keep(prev), cat: 'roles', role: roleId,
})

export const groupSearch = (prev: ContactsSearch, channelId: string): ContactsSearch => ({
  ...keep(prev), cat: 'groups', chat: channelId,
})

/** A profile over whatever the list shows. */
export const openPersonSearch = (prev: ContactsSearch, userId: string): ContactsSearch => ({ ...prev, person: userId })

export const closePersonSearch = (prev: ContactsSearch): ContactsSearch => {
  const { person: _person, ...rest } = prev
  return rest
}
