# TanStack Router Patterns

## Route Definition

Every route file exports a `Route` constant:

```tsx
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/_workspace/assets')({
  component: AssetList,
})

function AssetList() { /* ... */ }
```

## Auth Guards in Layout Routes

Use layout routes for auth checks — NOT per-page:

```tsx
// ✅ CORRECT: Auth guard in _workspace.tsx layout
function WorkspaceLayout() {
  const token = useAuthStore((s) => s.token)
  if (!token) return <Navigate to="/login" />
  return <div className="app-layout"><Sidebar /><Outlet /></div>
}

// ❌ WRONG: Auth check in every page component
function AssetsPage() {
  const token = useAuthStore((s) => s.token)
  if (!token) return <Navigate to="/login" /> // Duplicated!
}
```

## Navigation

### Programmatic
```tsx
const navigate = useNavigate()
navigate({ to: '/assets/$assetId', params: { assetId: '123' } })
```

### Declarative
```tsx
import { Navigate } from '@tanstack/react-router'
return <Navigate to="/login" />
```

### Links (prefer over <a>)
```tsx
import { Link } from '@tanstack/react-router'
<Link to="/assets/$assetId" params={{ assetId: item.id }}>View</Link>
```

## Route Params

Always use the typed `Route.useParams()`:

```tsx
// ✅ Type-safe params from the route
const { assetId } = Route.useParams()

// ❌ Untyped — avoid
import { useParams } from '@tanstack/react-router'
const { assetId } = useParams({ from: '/_workspace/assets/$assetId' })
```

## Search Params

```tsx
export const Route = createFileRoute('/_workspace/assets')({
  validateSearch: (search) => ({
    page: Number(search.page) || 1,
    status: (search.status as string) || '',
  }),
  component: AssetList,
})

function AssetList() {
  const { page, status } = Route.useSearch()
}
```

## Route Context

Pass shared data (like QueryClient) via router context:

```tsx
// main.tsx
const router = createRouter({ routeTree, context: { queryClient } })

// In route
export const Route = createFileRoute('/_workspace/assets')({
  loader: ({ context }) => {
    // context.queryClient is available here
    return context.queryClient.ensureQueryData(assetsQueryOptions('...'))
  },
})
```

## Rules

1. **Collocate route component in the route file** — don't import from a separate page file
2. **Auth guards belong in layout routes** — not individual pages
3. **Always use typed params** via `Route.useParams()` / `Route.useSearch()`
4. **Prefer `Link` over `useNavigate`** for static navigation
5. **Use `useNavigate` only** for post-action redirects (after mutation success)
