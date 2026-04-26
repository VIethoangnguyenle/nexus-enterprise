# Performance — Frontend

## Code Splitting
TanStack Router auto-splits by route. For heavy in-page components use `React.lazy`:
```tsx
const Chart = lazy(() => import('../components/Chart'))
<Suspense fallback={<div className="spinner" />}><Chart /></Suspense>
```

## Avoid Re-renders
- **Zustand selectors**: `const x = useStore((s) => s.x)` — never destructure whole store
- **Stable callbacks**: get actions from store selector, not inline
- **`useMemo`**: for expensive filter/sort, not trivial lookups

## Query Optimization
- Parallel by default — independent queries fire simultaneously
- `enabled: !!id` for dependent queries only
- `staleTime: 30_000` prevents re-fetch on back-navigation

## Bundle Size
- `npx vite build` shows sizes — flag if JS > 500 kB gzipped
- Named imports for tree-shaking
- Lazy-load below-fold images with `loading="lazy"`
