# Error Handling — Frontend

## API Error Handling

The `apiFetch` wrapper throws typed errors:

```tsx
// api/client.ts
export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, { ...init, headers })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error || body.message || `Request failed (${res.status})`)
  }
  return res.json()
}
```

This means:
- **Queries** — TanStack Query automatically catches and stores errors in `error` state
- **Mutations** — errors are available via `mutation.error`

## Displaying Errors

### Query Errors (page-level)
```tsx
function AssetList() {
  const { data, isLoading, error } = useAssets(wsId)

  if (error) {
    return (
      <div className="error-state">
        <div className="error-icon">⚠️</div>
        <h3>Failed to load assets</h3>
        <p className="text-muted">{error.message}</p>
        <button className="btn btn-primary" onClick={() => refetch()}>Retry</button>
      </div>
    )
  }
}
```

### Mutation Errors (inline)
```tsx
function LoginPage() {
  const login = useLogin()

  return (
    <form onSubmit={handleSubmit}>
      {login.error && <div className="error-msg">{login.error.message}</div>}
      {/* form fields */}
      <button disabled={login.isPending}>
        {login.isPending ? <span className="spinner" /> : 'Sign In'}
      </button>
    </form>
  )
}
```

## Loading States

Always show loading indicators:

```tsx
// Full-page loading
if (isLoading) return <div className="loading-center"><div className="spinner" /></div>

// Inline loading (buttons)
<button disabled={mutation.isPending}>
  {mutation.isPending ? <span className="spinner" /> : 'Submit'}
</button>
```

## Empty States

Always handle empty data with meaningful UI:

```tsx
if (assets.length === 0) {
  return (
    <div className="empty-state-large">
      <div className="empty-icon">📦</div>
      <h3>No assets yet</h3>
      <p>Create your first asset to get started.</p>
      <button className="btn btn-primary" onClick={...}>Create Asset</button>
    </div>
  )
}
```

## The State Machine Pattern

Every data-driven component follows this state machine:

```
Loading → Error (with retry)
        → Empty (with CTA)
        → Data (render content)
```

**NEVER skip a state.** Users MUST see feedback in every state.

## Rules

1. **Every query component handles: loading, error, empty, data** — no exceptions
2. **Mutation buttons show spinner when pending** — disable to prevent double-submit
3. **Errors show actionable messages** — not raw HTTP codes
4. **Include retry button on error states** — users should be able to recover
5. **Never swallow errors silently** — if a mutation fails, the user MUST know
