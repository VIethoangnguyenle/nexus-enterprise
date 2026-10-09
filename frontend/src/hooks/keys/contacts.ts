import type { ContactFilters } from '../../components/patterns/ContactsFilterBar'

export const contactKeys = {
  all: (wsId: string) => ['contacts', wsId] as const,
  list: (wsId: string, filters?: ContactFilters) => ['contacts', wsId, filters] as const,
}
