## Context

Frontend NGAC dùng Vite + TanStack Router + TanStack Query + Zustand, với kiến trúc đã đúng chuẩn ở tầng data layer (queryOptions factory, WebSocket→Query bridge, Zustand chỉ cho UI state). Vấn đề nằm ở tầng UX discipline — thiếu error states, mutation feedback, và type safety. Audit cho thấy 22 violations trên 9 reference documents của frontend-best-practices skill.

### Current State

```
                    ┌──────────────────────────────────────────┐
                    │            Route Pages (6 pages)         │
                    │                                          │
                    │   ┌────────────────────────────────┐     │
                    │   │  Query hooks (useAssets, etc.)  │     │
                    │   │                                 │     │
                    │   │  ✅ queryOptions factory        │     │
                    │   │  ✅ mutation + invalidation     │     │
                    │   │  ❌ 3 inline queries remain    │     │
                    │   │  ❌ `any` in 4 mutation params │     │
                    │   └────────────┬───────────────────┘     │
                    │                │                          │
                    │   ┌────────────▼───────────────────┐     │
                    │   │     Component Rendering         │     │
                    │   │                                 │     │
                    │   │  ✅ Loading state (most pages)  │     │
                    │   │  ✅ Empty state (most pages)    │     │
                    │   │  ❌ Error state: 0 of 6 pages  │     │
                    │   │  ❌ Mutation error: 0 pages     │     │
                    │   │  ❌ div onClick nav (Sidebar)   │     │
                    │   └────────────────────────────────┘     │
                    └──────────────────────────────────────────┘
```

### Constraints

- Không thêm dependency mới (không toast library) — dùng inline error display
- Không thay đổi API contract — chỉ frontend
- Giữ nguyên kiến trúc hiện tại — chỉ harden
- File conventions theo TanStack Router file-based routing
- CSS dùng existing classes từ `index.css`

## Goals / Non-Goals

**Goals:**
- Enforce Loading→Error/Empty/Data state machine trên 100% query-backed components
- Mutation error visible trên 100% mutation-using pages
- Type safety: zero `any` trong hooks layer
- Navigation accessibility: semantic HTML với `<Link>` components
- Code deduplication: shared constants

**Non-Goals:**
- Không thêm toast notification system (chờ đến khi có nhu cầu rõ hơn)
- Không thêm lazy loading (app còn nhỏ)
- Không fix WebSocket token-in-URL (cần backend change)
- Không thêm error boundary (React Error Boundary) — scope chỉ là component-level error handling

## Decisions

### Decision 1: Reusable state components thay vì inline JSX

**Chosen**: Tạo 3 components: `ErrorState`, `LoadingState`, `EmptyState`

**Alternatives considered**:
- *Inline JSX mỗi page*: Code hiện tại — duplicated, inconsistent. Mỗi page tự write loading/error markup
- *Higher-order component wrap*: Quá trừu tượng cho use case đơn giản

**Rationale**: Components nhỏ (~15 lines mỗi cái), enforce consistency. Pattern theo đúng error-handling.md reference:
```
Loading → Error (with retry)
        → Empty (with CTA)  
        → Data (render content)
```

### Decision 2: Inline error display cho mutations (không toast)

**Chosen**: Display `mutation.error` inline dưới form/button, giống pattern login page hiện tại

**Alternatives considered**:
- *Toast library (Sonner/react-hot-toast)*: Better UX cho transient errors nhưng thêm dependency. Chưa cần ở phase này
- *Global error boundary*: Không phù hợp cho mutation errors — error boundary bắt render errors, không bắt async errors

**Rationale**: Login page đã có pattern đúng: `{login.error && <div className="error-msg">{login.error.message}</div>}`. Apply pattern này nhất quán cho mọi mutation-using page. Zero new dependencies.

### Decision 3: Sidebar dùng `<Link>` thay vì `div` + `onClick`

**Chosen**: Replace `div.sidebar-item` + `onClick={navigate}` bằng `<Link>` component từ TanStack Router

**Rationale**:
- `<Link>` render ra `<a>` → semantic HTML, screen reader compatible
- Hỗ trợ cmd/ctrl+click (open in new tab)
- Hỗ trợ right-click context menu
- `activeProps` / `activeOptions` tự động manage active state — xóa được `isActive()` helper
- Performance: `<Link>` prefetch on hover (nếu configured)

### Decision 4: Fix `documentApi.create` — custom upload handler thay vì raw fetch

**Chosen**: Tạo `apiUpload` helper trong `api/client.ts` cho FormData uploads, reuse auth logic

**Rationale**: `documentApi.create` hiện bypass `apiFetch` hoàn toàn — dùng raw `fetch()`, tự inject token, không check `res.ok`. Nếu token expired → không auto-logout. Nếu server error → no error thrown. Cần helper riêng vì `apiFetch` set `Content-Type: application/json` — FormData cần browser tự set boundary.

```tsx
// New helper in api/client.ts
export async function apiUpload<T>(path: string, body: FormData): Promise<T> {
  const token = useAuthStore.getState().token
  const headers: HeadersInit = token ? { Authorization: `Bearer ${token}` } : {}
  
  const res = await fetch(`${API_BASE}${path}`, { method: 'POST', body, headers })
  
  if (res.status === 401) {
    useAuthStore.getState().logout()
    throw new ApiError('Unauthorized', 401)
  }
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new ApiError(data.error || data.message || res.statusText, res.status, data)
  }
  return res.json()
}
```

### Decision 5: Remove `window.location.href` from auth store logout

**Chosen**: Store chỉ clear state (`token: null, user: null`). Layout route `_workspace.tsx` đã handle redirect via `<Navigate to="/login" />`.

**Rationale**: `window.location.href = '/login'` gây full page reload — destroy React app state, TanStack Query cache, WebSocket connections. Layout route đã có guard: `if (!token) return <Navigate to="/login" />` — double redirect là unnecessary.

### Decision 6: Type definitions cho remaining untyped APIs

**Chosen**: Define interfaces cho `AssetHistory`, `AssetSummary`, và proper mutation input types

```tsx
// Missing types to add
export interface AssetHistory {
  action: string
  from_state: string
  to_state: string
  actor_id?: string
  comment?: string
  created_at?: string
}

export interface AssetSummary {
  total: number
  in_use: number
  pending: number
  by_state: Record<string, number>
}

// Replace mutation `any` with proper types
interface CreateAssetTypeInput { name: string; category: string; fields_schema?: object; lifecycle_config?: object }
interface CreateAssetInput { name: string; type_id: string; custom_fields?: object }
interface CreateAssetRequestInput { type_id: string; justification?: string; urgency?: string }
interface CreateChannelInput { name: string; channel_type: string }
```

## Risks / Trade-offs

| Risk | Impact | Mitigation |
|------|--------|------------|
| Reusable components force uniform UX | Một số page có thể cần custom error layout | Components accept props cho customization (title, message, icon). Không quá rigid |
| `<Link>` thay `div` có thể ảnh hưởng CSS | Sidebar styling depend vào `div.sidebar-item` selector | `<Link>` accept `className` — giữ nguyên CSS classes. Chỉ thay tag, không thay structure |
| Remove `window.location.href` từ logout | Nếu layout route guard bị bypass → user không redirect | Guard đã proven work (`_workspace.tsx:23`). Thêm fallback: sau khi clear state, navigate via router nếu available |
| TypeScript strict mode | Có thể surface thêm type errors chưa fix | Apply strict dần — fix errors as-encountered, không block build |
