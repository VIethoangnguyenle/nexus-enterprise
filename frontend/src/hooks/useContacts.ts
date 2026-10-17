import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api/client'
import { keys } from './keys'

export interface ContactFilters {
  department: string
  location: string
  search: string
}

/**
 * A person of the workspace as the directory returns them. `user_id` and
 * `ngac_node_id` are for calling APIs and picking a colour; they are never shown.
 */
export interface Contact {
  user_id: string
  ngac_node_id: string
  username: string
  display_name: string
  email: string
  title: string
  department: string
  /** The department's id (a leaf of the tree); empty for a person in none. Tells apart two departments with one name. */
  department_id: string
  /** The roles the person holds; the built-in Members role comes with a blank name. */
  roles: ContactRole[]
  location: string
  avatar_url: string
  is_online: boolean
}

export interface ContactRole {
  id: string
  /** Blank for the built-in roles: the screen words them from `kind`. */
  name: string
  kind: 'owners' | 'members' | 'custom'
}

/** `limited`: the signed-in user sees only part of the workspace, and the screens say "trong phạm vi bạn được xem". */
export type DirectoryVisibility = 'all' | 'limited'

interface ContactsResponse {
  contacts: Contact[]
  /** How many people the workspace has, which can be more than were returned. */
  total: number
  /** Present when the directory is served in pages: what to ask for after this one. */
  next_cursor?: string
  visibility?: DirectoryVisibility
}

/** The directory as the screens read it: every page, with the fields an older server may lack filled in. */
export interface Directory {
  contacts: Contact[]
  total: number
  visibility: DirectoryVisibility
}

/** A server that predates departments and roles omits them; the screens can count on both. */
const complete = (c: Contact): Contact => ({ ...c, department_id: c.department_id ?? '', roles: c.roles ?? [] })

/** The largest page the contacts endpoint serves; a workspace that fits needs one request. */
const PAGE_SIZE = 500

/** A directory that never ends is a bug somewhere; stop asking after this many pages. */
const MAX_PAGES = 100

/**
 * The workspace's directory: everyone who belongs to it, with profile data.
 * A failure is a failure: the screens say so rather than listing members with
 * their names missing.
 */
export function useContacts(workspaceId: string, filters?: ContactFilters) {
  return useQuery<Directory>({
    queryKey: keys.contacts.list(workspaceId, filters),
    queryFn: async () => {
      const base = new URLSearchParams({ limit: String(PAGE_SIZE) })
      if (filters?.department) base.set('department', filters.department)
      if (filters?.location) base.set('location', filters.location)
      if (filters?.search) base.set('search', filters.search)
      // The whole directory, page by page: the directory is one list to every
      // screen that names a person, and a page missing from it is a person shown
      // as "Thành viên".
      const people: Contact[] = []
      let total = 0
      let visibility: DirectoryVisibility = 'all'
      let cursor: string | undefined
      for (let page = 0; page < MAX_PAGES; page++) {
        const params = new URLSearchParams(base)
        if (cursor) params.set('cursor', cursor)
        const qs = params.toString()
        const res = await apiFetch<ContactsResponse>(`/workspaces/${workspaceId}/contacts${qs ? '?' + qs : ''}`)
        people.push(...(res.contacts ?? []).map(complete))
        total = res.total ?? people.length
        if (res.visibility === 'limited') visibility = 'limited'
        cursor = res.next_cursor || undefined
        if (!cursor) break
      }
      return { contacts: people, total, visibility }
    },
    enabled: !!workspaceId,
  })
}
