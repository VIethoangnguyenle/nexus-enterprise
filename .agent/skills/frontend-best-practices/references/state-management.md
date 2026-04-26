# State Management — Zustand + TanStack Query

## The Golden Rule

> **TanStack Query = ALL server state. Zustand = UI-only state.**

This is NOT negotiable. Violating this rule creates data inconsistency bugs.

## What Goes Where

### TanStack Query (server state)
- ✅ Asset lists, documents, channels, messages, notifications
- ✅ User profile, workspace info
- ✅ Any data that comes from the API
- ✅ Any data that other users might change

### Zustand (UI state only)
- ✅ Sidebar collapsed/expanded
- ✅ Active modal name
- ✅ WebSocket connection status
- ✅ Typing indicators
- ✅ Theme preference
- ✅ Unsaved form drafts (local-only)

### ❌ NEVER in Zustand
- ❌ Asset list cache → use TanStack Query
- ❌ Current user data → use TanStack Query
- ❌ Notification count → use TanStack Query
- ❌ Channel messages → use TanStack Query

## Zustand Store Convention

```tsx
// stores/ui.store.ts
import { create } from 'zustand'

interface UiState {
  sidebarCollapsed: boolean
  activeModal: string | null
  toggleSidebar: () => void
  openModal: (name: string) => void
  closeModal: () => void
}

export const useUiStore = create<UiState>((set) => ({
  sidebarCollapsed: false,
  activeModal: null,
  toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
  openModal: (name) => set({ activeModal: name }),
  closeModal: () => set({ activeModal: null }),
}))
```

Rules:
- One file per store domain (`auth.store.ts`, `ui.store.ts`, `websocket.store.ts`)
- Export typed hook directly: `export const useUiStore = create<UiState>(...)`
- Actions inside the store — no external dispatch functions
- Use `persist` middleware only for auth token (localStorage)

## WebSocket → TanStack Query Bridge

The WebSocket store bridges real-time events to TanStack Query invalidation:

```tsx
// websocket.store.ts
import { queryClient } from '../lib/query-client'

// Inside message handler:
const handleEvent = (event: WsEvent) => {
  switch (event.type) {
    case 'new_message':
      queryClient.invalidateQueries({ queryKey: ['messages', event.channel_id] })
      break
    case 'notification':
      queryClient.invalidateQueries({ queryKey: ['notifications'] })
      queryClient.invalidateQueries({ queryKey: ['unread-count'] })
      break
    case 'asset_transition':
      queryClient.invalidateQueries({ queryKey: ['assets'] })
      queryClient.invalidateQueries({ queryKey: ['asset', event.entity_id] })
      break
  }
}
```

This pattern ensures:
- Real-time updates without polling
- Single source of truth (TanStack Query cache)
- No manual state synchronization

## Auth Store (special case)

Auth store uses `persist` middleware for token survival across page reloads:

```tsx
import { persist } from 'zustand/middleware'

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      login: (token, user) => set({ token, user }),
      logout: () => set({ token: null, user: null }),
    }),
    { name: 'ngac-auth' }
  )
)
```

## Rules

1. **Server data → TanStack Query. UI state → Zustand.** No exceptions.
2. **Zustand stores have no API calls** — they manage UI behavior only
3. **WebSocket events invalidate queries** — don't update Zustand with server data
4. **One store per concern** — auth, ui, websocket (3 stores total)
5. **Persist only auth token** — everything else is ephemeral or cached by Query
