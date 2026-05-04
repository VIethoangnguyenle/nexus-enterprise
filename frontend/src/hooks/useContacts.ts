import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '../api/client'
import type { ContactFilters } from '../components/patterns/ContactsFilterBar'

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
  total: number
  page: number
  limit: number
}

/** Fetch contacts (workspace members with profile data). Falls back to workspace members API. */
export function useContacts(workspaceId: string, filters?: ContactFilters) {
  return useQuery<ContactsResponse>({
    queryKey: ['contacts', workspaceId, filters],
    queryFn: async () => {
      const params = new URLSearchParams()
      if (filters?.department) params.set('department', filters.department)
      if (filters?.location) params.set('location', filters.location)
      if (filters?.search) params.set('search', filters.search)

      try {
        // Try the contacts endpoint first (requires backend Phase 4)
        const qs = params.toString()
        return await apiFetch(`/workspaces/${workspaceId}/contacts${qs ? '?' + qs : ''}`)
      } catch {
        // Fallback: use workspace members API and map to contact shape
        const data = await apiFetch(`/workspaces/${workspaceId}/members`)
        const members = data?.members || []
        return {
          contacts: members.map((m: any) => ({
            user_id: m.user_id || m.id || '',
            ngac_node_id: m.ngac_node_id || '',
            username: m.username || '',
            display_name: m.display_name || m.username || 'Unknown',
            email: m.email || '',
            title: m.title || '',
            department: m.department || '',
            location: m.location || '',
            avatar_url: m.avatar_url || '',
            is_online: false,
          })),
          total: members.length,
          page: 1,
          limit: 50,
        }
      }
    },
    enabled: !!workspaceId,
  })
}
