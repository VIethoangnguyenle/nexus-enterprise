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
  location: string
  avatar_url: string
  is_online: boolean
}

interface ContactsResponse {
  contacts: Contact[]
  /** How many people the workspace has, which can be more than were returned. */
  total: number
  /** Present when the directory is served in pages: what to ask for after this one. */
  next_cursor?: string
}

/** A directory that never ends is a bug somewhere; stop asking after this many pages. */
const MAX_PAGES = 100

/**
 * The workspace's directory: everyone who belongs to it, with profile data.
 * A failure is a failure: the screens say so rather than listing members with
 * their names missing.
 */
export function useContacts(workspaceId: string, filters?: ContactFilters) {
  return useQuery<ContactsResponse>({
    queryKey: keys.contacts.list(workspaceId, filters),
    queryFn: async () => {
      const base = new URLSearchParams()
      if (filters?.department) base.set('department', filters.department)
      if (filters?.location) base.set('location', filters.location)
      if (filters?.search) base.set('search', filters.search)
      // The whole directory, page by page: the directory is one list to every
      // screen that names a person, and a page missing from it is a person shown
      // as "Thành viên".
      const people: Contact[] = []
      let total = 0
      let cursor: string | undefined
      for (let page = 0; page < MAX_PAGES; page++) {
        const params = new URLSearchParams(base)
        if (cursor) params.set('cursor', cursor)
        const qs = params.toString()
        const res = await apiFetch<ContactsResponse>(`/workspaces/${workspaceId}/contacts${qs ? '?' + qs : ''}`)
        people.push(...(res.contacts ?? []))
        total = res.total ?? people.length
        cursor = res.next_cursor || undefined
        if (!cursor) break
      }
      return { contacts: people, total }
    },
    enabled: !!workspaceId,
  })
}
