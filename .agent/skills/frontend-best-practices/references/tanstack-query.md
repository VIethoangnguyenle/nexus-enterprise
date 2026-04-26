# TanStack Query Patterns

## queryOptions Factory (REQUIRED)

Always define queries using the `queryOptions` factory — this enables type-safe reuse across components and loaders:

```tsx
// ✅ REQUIRED: queryOptions factory in hooks file
import { queryOptions } from '@tanstack/react-query'

export const assetsQueryOptions = (wsId: string, params?: Record<string, string>) =>
  queryOptions({
    queryKey: ['assets', wsId, params],
    queryFn: () => assetApi.list(wsId, params),
    enabled: !!wsId,
  })

// Usage in component
function AssetList() {
  const { data, isLoading } = useQuery(assetsQueryOptions(wsId))
}

// ❌ WRONG: Inline query in component
function AssetList() {
  const { data } = useQuery({
    queryKey: ['assets', wsId],
    queryFn: () => assetApi.list(wsId),
  })
}
```

## Query Key Conventions

```
['entity']                          — list all
['entity', wsId]                    — list scoped to workspace
['entity', wsId, { status: 'x' }]  — list with filters
['entity-detail', id]              — single item
['entity-count']                    — aggregation
```

Rules:
- Entity name as first element (noun, kebab-case)
- Scope IDs as second element
- Filter objects as last element
- Detail queries use `-detail` suffix or separate entity name

## Mutation + Invalidation

Always invalidate related queries on success:

```tsx
export function useCreateAsset(wsId: string) {
  return useMutation({
    mutationFn: (data: any) => assetApi.create(wsId, data),
    onSuccess: () => {
      // ✅ Invalidate list queries — triggers refetch
      queryClient.invalidateQueries({ queryKey: ['assets', wsId] })
      // ✅ Also invalidate summary if it exists
      queryClient.invalidateQueries({ queryKey: ['asset-summary', wsId] })
    },
  })
}
```

## Optimistic Updates (when needed)

```tsx
useMutation({
  mutationFn: (id: string) => assetApi.approve(id),
  onMutate: async (id) => {
    await queryClient.cancelQueries({ queryKey: ['asset', id] })
    const prev = queryClient.getQueryData(['asset', id])
    queryClient.setQueryData(['asset', id], (old: any) => ({ ...old, state: 'approved' }))
    return { prev }
  },
  onError: (_err, _id, context) => {
    queryClient.setQueryData(['asset', context?.prev?.id], context?.prev)
  },
  onSettled: (_, __, id) => {
    queryClient.invalidateQueries({ queryKey: ['asset', id] })
  },
})
```

## Loading/Error/Empty Pattern

Every query-backed component MUST handle all 3 states:

```tsx
function AssetList() {
  const { data, isLoading, error } = useAssets(wsId)

  // 1. Loading state
  if (isLoading) return <div className="loading-center"><div className="spinner" /></div>

  // 2. Error state
  if (error) return <div className="error-msg">{error.message}</div>

  // 3. Empty state
  const assets = data?.assets || []
  if (assets.length === 0) return <div className="empty-state-large"><h3>No assets</h3></div>

  // 4. Data state
  return <div>{/* render assets */}</div>
}
```

## Stale/Cache Configuration

Defined in `lib/query-client.ts`:

```tsx
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,     // 30 seconds — avoid refetching on every mount
      gcTime: 5 * 60_000,    // 5 minutes garbage collection
      retry: 1,              // Only 1 retry on failure
      refetchOnWindowFocus: false,
    },
  },
})
```

## WebSocket → Query Invalidation Bridge

WebSocket events automatically invalidate relevant queries:

```tsx
// In websocket.store.ts
case 'asset_updated':
  queryClient.invalidateQueries({ queryKey: ['assets'] })
  queryClient.invalidateQueries({ queryKey: ['asset', event.entity_id] })
  break
case 'new_message':
  queryClient.invalidateQueries({ queryKey: ['messages', event.channel_id] })
  break
```

## Rules

1. **ALWAYS use `queryOptions` factory** — never inline query config in components
2. **ALWAYS invalidate on mutation success** — stale data = bugs
3. **ALWAYS handle loading/error/empty states** — no bare `data?.` without guards
4. **NEVER put server data in Zustand** — that's TanStack Query's job
5. **`enabled: !!id`** — disable queries when params are not yet available
6. **Import `queryClient` from `lib/query-client`** — single instance
