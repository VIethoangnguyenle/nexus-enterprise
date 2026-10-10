type Params = object | undefined

/**
 * Asset query keys. Lists, types, requests, the summary and the activity feed
 * are per workspace; a single asset, its history and a single request are
 * addressed by id. The `*All` prefixes cover every workspace, for callers (the
 * WebSocket event, the transition mutation) that know the asset but not the
 * workspace.
 */
export const assetKeys = {
  typesAll: () => ['asset-types'] as const,
  types: (wsId: string) => ['asset-types', wsId] as const,
  listsAll: () => ['assets'] as const,
  lists: (wsId: string) => ['assets', wsId] as const,
  list: (wsId: string, params?: Params) => ['assets', wsId, params] as const,
  asset: (id: string) => ['asset', id] as const,
  summaries: () => ['asset-summary'] as const,
  summary: (wsId: string) => ['asset-summary', wsId] as const,
  activitiesAll: () => ['asset-activity'] as const,
  activity: (wsId: string, limit?: number) => ['asset-activity', wsId, limit] as const,
  requestsAll: () => ['asset-requests'] as const,
  requests: (wsId: string) => ['asset-requests', wsId] as const,
  requestList: (wsId: string, params?: Params) => ['asset-requests', wsId, params] as const,
  request: (id: string) => ['asset-request', id] as const,
  requestDetailsAll: () => ['asset-request'] as const,
  transitions: (id: string) => ['asset-transitions', id] as const,
  history: (id: string) => ['asset-history', id] as const,
}
