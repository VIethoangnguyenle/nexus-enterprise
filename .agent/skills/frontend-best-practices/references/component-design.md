# Component Design

## Component File Structure

```tsx
// 1. Imports — grouped: framework, hooks, api, components, types
import { useState, useEffect } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useAssets } from '../../hooks/useAssets'
import type { Asset } from '../../api/assets'

// 2. Props interface (if component accepts props)
interface AssetCardProps {
  asset: Asset
  onAction?: (id: string) => void
}

// 3. Component function (named export for shared, default export for routes)
export function AssetCard({ asset, onAction }: AssetCardProps) {
  // 3a. Hooks first
  const navigate = useNavigate()
  const [expanded, setExpanded] = useState(false)

  // 3b. Derived values
  const isActive = asset.state === 'in_use'

  // 3c. Handlers
  const handleClick = () => onAction?.(asset.id)

  // 3d. Render
  return (
    <div className={`asset-card ${isActive ? 'active' : ''}`}>
      {/* ... */}
    </div>
  )
}
```

## Naming Conventions

| Type | Convention | Example |
|------|-----------|---------|
| Route component | PascalCase, suffix with Page/Layout | `AssetDashboard`, `WorkspaceLayout` |
| Shared component | PascalCase | `Sidebar`, `NotificationBell` |
| Hook | camelCase, prefix with `use` | `useAssets`, `useLogin` |
| API module | camelCase | `assetApi`, `authApi` |
| Store | camelCase, suffix `.store.ts` | `auth.store.ts`, `ui.store.ts` |
| CSS class | kebab-case | `asset-card`, `sidebar-item` |
| Event handler | prefix with `handle` | `handleSubmit`, `handleClick` |

## Composition Patterns

### Prefer composition over configuration:

```tsx
// ✅ Composition
<Card>
  <CardHeader><h3>Assets</h3></CardHeader>
  <CardBody>{children}</CardBody>
</Card>

// ❌ Configuration props
<Card title="Assets" headerSize="h3" bodyPadding="1rem" />
```

### Render pattern for lists:

```tsx
// ✅ Map in JSX — simple, readable
{assets.map(a => <AssetCard key={a.id} asset={a} />)}

// ❌ Separate render function (unnecessary indirection)
const renderAsset = (a: Asset) => <AssetCard key={a.id} asset={a} />
```

## State Colocation

Keep state as close to where it's used as possible:

```tsx
// ✅ Form state lives in the form component
function AssetRequestForm() {
  const [typeId, setTypeId] = useState('')
  const [justification, setJustification] = useState('')
  // ...
}

// ❌ Form state lifted to parent or global store
```

## Rules

1. **One component per file** — for shared components in `components/`
2. **Route components are colocated** — component function lives in the route file
3. **Props interfaces defined in the same file** — or in a shared `types.ts` if reused
4. **No `any` in props** — always type props interfaces
5. **Hooks at the top** — before any conditional logic
6. **No business logic in render** — extract to handlers or hooks
7. **Key prop on every mapped element** — use stable IDs, not array index
