# Design: Fix Workspace Select Flow

## Architecture

```
Login OTP Success
       │
       ▼
┌──────────────────────────────┐
│   /workspace-select          │  ← NEW redirect target
│                              │
│   useWorkspaces() → API call │
│        │                     │
│        ├── 0 workspaces ────────▶ /onboarding
│        ├── 1 workspace  ────────▶ /documents?ws={id} (auto-skip)
│        └── 2+ workspaces ───────▶ Show selection UI
│                              │
└──────────────────────────────┘
```

## Changes

### 1. `login.tsx` — Redirect target

**Before:**
```tsx
onSuccess: () => {
  navigate({ to: '/documents', search: ... })
}
```

**After:**
```tsx
onSuccess: () => {
  navigate({ to: '/workspace-select' })
}
```

Lý do: Login không cần biết user có bao nhiêu workspace. Để workspace-select xử lý routing logic.

### 2. `workspace-select.tsx` — Auto-skip logic

Thêm `useEffect` auto-navigate khi:
- `workspaces.length === 0` → redirect `/onboarding`
- `workspaces.length === 1` → redirect `/documents?ws={workspaces[0].id}`
- `workspaces.length >= 2` → hiển thị UI chọn

```tsx
useEffect(() => {
  if (isLoading || !data) return
  const ws = data.workspaces || []
  if (ws.length === 0) {
    navigate({ to: '/onboarding' })
  } else if (ws.length === 1) {
    navigate({ to: '/documents', search: { ws: ws[0].id } })
  }
  // else: show selection UI (default behavior)
}, [data, isLoading, navigate])
```

### 3. `workspace-select.tsx` — Real metadata

**Hiện tại:** Hardcode "Enterprise Tier" và "Free Tier · 1 Member"

**Sau:** Hiện tên workspace + member count (nếu có).

Vì `Workspace` type chưa có `member_count` hay `type`, ta sẽ:
- Hiển thị workspace name (đã có)
- Subtitle: `"Workspace"` (generic) — không fake tier name
- Không cần call listMembers (N+1 problem, sẽ giải quyết sau khi backend thêm member_count vào list response)

### 4. Edge case: URL search params

Login hiện forward `window.location.search` vào navigate. Workspace-select cần giữ lại logic này — sau khi chọn workspace, forward bất kỳ search params nào từ original URL.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Auto-skip 1 workspace? | Yes | Giảm friction cho single-workspace users |
| Show real member count? | No (deferred) | Cần backend thêm field vào list response, tạo N+1 nếu client gọi |
| Forward search params? | Yes | Giữ deep-link capability |
