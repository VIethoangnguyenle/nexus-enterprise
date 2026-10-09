type Params = Record<string, string> | undefined

/**
 * Asset query keys. Lists, types, requests and the summary are per workspace;
 * a single asset and its history are addressed by asset id. The `*All`
 * prefixes cover every workspace, for callers (the WebSocket event, the
 * transition mutation) that know the asset but not the workspace.
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
  requests: (wsId: string) => ['asset-requests', wsId] as const,
  requestList: (wsId: string, params?: Params) => ['asset-requests', wsId, params] as const,
  transitions: (id: string) => ['asset-transitions', id] as const,
  history: (id: string) => ['asset-history', id] as const,
}
