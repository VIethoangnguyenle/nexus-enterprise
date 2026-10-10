import type { Contact } from '../../hooks/useContacts'
import { normalize, UNKNOWN_PERSON } from '../../lib/people'

/** The name a person is shown by. An id is never a fallback. */
export function nameOf(c: Pick<Contact, 'display_name'>): string {
  // A username is a login handle, not a name: never shown.
  return c.display_name?.trim() || UNKNOWN_PERSON
}

/** "Trưởng phòng · Vận hành thanh toán": title and department, either may be missing. */
export function roleLine(c: Pick<Contact, 'title' | 'department'>): string {
  return [c.title, c.department].filter(Boolean).join(' · ')
}

/** People sorted by name the way Vietnamese is collated. */
export function sortContacts(list: Contact[]): Contact[] {
  return [...list].sort((a, b) => nameOf(a).localeCompare(nameOf(b), 'vi', { sensitivity: 'base' }))
}

export interface ContactFilter {
  query: string
  /** A department name; empty is everyone. */
  department: string
  onlineOnly: boolean
}

export const NO_FILTER: ContactFilter = { query: '', department: '', onlineOnly: false }

export const isFiltering = (f: ContactFilter) => !!(f.query.trim() || f.department || f.onlineOnly)

/** Matches name, title and email, ignoring case and accents ("duc" finds "Đức"). */
export function filterContacts(list: Contact[], f: ContactFilter, isOnline: (c: Contact) => boolean): Contact[] {
  const q = normalize(f.query)
  return list.filter((c) => {
    if (f.department && c.department !== f.department) return false
    if (f.onlineOnly && !isOnline(c)) return false
    if (!q) return true
    return [nameOf(c), c.title, c.email].some((part) => normalize(part ?? '').includes(q))
  })
}

export interface DepartmentCount { name: string; count: number }

/** Departments that have people, with how many, by name. */
export function departmentsOf(list: Contact[]): DepartmentCount[] {
  const counts = new Map<string, number>()
  for (const c of list) if (c.department) counts.set(c.department, (counts.get(c.department) ?? 0) + 1)
  return [...counts]
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => a.name.localeCompare(b.name, 'vi', { sensitivity: 'base' }))
}

/** Others in the same department as `of`, never `of` themselves. */
export function colleaguesOf(list: Contact[], of: Contact): Contact[] {
  if (!of.department) return []
  return sortContacts(list.filter((c) => c.department === of.department && c.user_id !== of.user_id))
}
