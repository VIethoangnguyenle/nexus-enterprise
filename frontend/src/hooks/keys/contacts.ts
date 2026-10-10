import type { ContactFilters } from '../useContacts'

export const contactKeys = {
  /** Every workspace's directory: a change to a person shows in all of them. */
  every: () => ['contacts'] as const,
  all: (wsId: string) => ['contacts', wsId] as const,
  list: (wsId: string, filters?: ContactFilters) => ['contacts', wsId, filters] as const,
}
