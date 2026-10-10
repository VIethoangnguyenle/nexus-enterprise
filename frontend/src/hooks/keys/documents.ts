import type { TextScope } from '../../api/documents'

export const documentKeys = {
  /** Every text-document listing of a workspace. */
  all: (wsId: string) => ['documents', wsId] as const,
  list: (wsId: string, scope: TextScope = 'all') => ['documents', wsId, 'list', scope] as const,
  count: (wsId: string, scope: TextScope = 'all') => ['documents', wsId, 'count', scope] as const,
  detail: (id: string) => ['document', id] as const,
}
