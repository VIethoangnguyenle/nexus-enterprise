# Design Audit — NGAC Platform

> Audited: 2026-05-01 | Scope: Full system

---

## 1. Pattern Extraction — Current UI Patterns

### Design System State: CONFLICTED

Two competing design documents exist:

| Aspect | `DESIGN.md` (root, 386 lines) | `.stitch/DESIGN.md` (Stitch, 146 lines) | `index.css` (implemented) |
|--------|-------------------------------|----------------------------------------|---------------------------|
| Theme | Dark mode native | Light mode M3 | ✅ **Light mode M3** |
| Font | Inter Variable | Manrope | ✅ **Manrope** |
| Primary | `#3370FF` | `#004AC6` / `#2563EB` | ✅ **`#2563EB`** |
| Surfaces | `#08090a` → `#323843` (14-shade gray) | `#F9F9FF` → `#D8E3FB` (M3 containers) | ✅ **M3 containers** |
| Rail width | 48px | 72px | Mix (Lark Rail: varies) |
| Layout | 4-column | 3-column | **Hybrid** |
| Row height | 36px (dense) | Balanced 6/10 | Partial |
| Density | Max 18px text | 32px heading | `text-h1`: 32px (Stitch wins) |

**Verdict**: `index.css` follows Stitch/M3 light system. Root `DESIGN.md` is **dead documentation**.

### Token Layer: Functional but Polluted

`index.css` defines 3 token layers:
1. **M3 Surface tokens** (canonical): `--color-surface-*`, `--color-on-surface-*`
2. **Legacy gray aliases**: `--color-gray-1` through `--color-gray-13` (mapped to M3, names misleading)
3. **Legacy semantic aliases**: `--color-bg-primary`, `--color-text-primary` etc.

**Usage distribution** (component/route files):

| Token system | Usage in components | Usage in routes |
|-------------|--------------------|-----------------| 
| M3 tokens (`surface-container-*`, `on-surface-*`) | Primitives: ✅ 100% | ~60% |
| Legacy gray (`gray-3`, `gray-10`, etc.) | Composites: ~50%, Patterns: ~40% | ~30% |
| Hardcoded Tailwind (`bg-green-500`, etc.) | ContactsTable, ContactCard: ❌ | 0% |

### Component Architecture

```
                 ┌──────────────────┐
                 │   primitives/    │  10 components — CLEAN M3 tokens
                 │   (foundation)   │  Button, Input, Avatar, Badge,
                 │                  │  IconButton, Spinner, Text, Heading,
                 │                  │  Select, Textarea
                 └────────┬─────────┘
                          │ composes
                 ┌────────▼─────────┐
                 │   composites/    │  10 components — MIXED tokens
                 │   (building      │  Modal, DataTable, Card, Tabs,
                 │    blocks)       │  PeekPanel, ConfirmDialog,
                 │                  │  AlertBanner, Breadcrumbs,
                 │                  │  FilterBar, Timeline
                 └────────┬─────────┘
                          │ composes
         ┌────────────────▼───────────────────┐
         │           patterns/                │  19 components — DUMPING GROUND
         │                                    │  
         │  Layout:   AppSidebar, LarkRail,   │
         │            TopBar, MobileNav       │  ← belongs in layouts/
         │                                    │
         │  Chat:     ChatList, ChatListItem, │
         │            ChatInput, MessageItem  │  ← duplicates chat/
         │                                    │
         │  Contacts: ContactsTable, Card,    │
         │            FilterBar, Sidebar,     │
         │            ProfilePanel            │  ← should be contacts/
         │                                    │
         │  Drive:    ChannelDrivePanel,       │
         │            FilePreviewCard,         │
         │            ImagePreviewCard         │  ← should be drive/
         │                                    │
         │  Generic:  ListPanel, TreeView     │  ← should be composites/
         └────────────────────────────────────┘
```

---

## 2. Anti-Pattern Detection

### 2.1 Duplicate Components

| Duplicate | Location A | Location B | Issue |
|-----------|-----------|-----------|-------|
| Close button | `PeekPanel.tsx` inline `<button>` | `Modal.tsx` uses `<IconButton>` | PeekPanel creates own close button instead of reusing IconButton |
| Tab-like navigation | `Tabs.tsx` composite | `approval.tsx` route inline tabs | Approval page builds own tab system |
| File preview | `FilePreviewCard.tsx` patterns/ | `DrivePreviewDialog.tsx` drive/ | Two different preview implementations |

### 2.2 Token Inconsistency

| File | Issue | Specific |
|------|-------|----------|
| `DataTable.tsx` | Uses legacy gray tokens | `bg-gray-3`, `text-gray-10`, `text-gray-12`, `hover:bg-gray-6` |
| `PeekPanel.tsx` | Uses legacy gray tokens | `bg-gray-4`, `text-gray-13`, `text-gray-10`, `hover:bg-gray-6` |
| `Card.tsx` | Uses legacy gray tokens | `bg-gray-5` |
| `Tabs.tsx` | Uses legacy gray tokens | `text-gray-13`, `text-gray-10` |
| `ContactsTable.tsx` | Uses RAW Tailwind colors | `bg-green-500`, `bg-orange-400`, `bg-gray-300`, `bg-green-50`, etc. |
| `ContactCard.tsx` | Uses RAW Tailwind colors | `bg-green-500` |
| `ContactProfilePanel.tsx` | Uses RAW Tailwind colors | `bg-green-500`, `bg-gray-300` |
| `EmojiPicker.tsx` | Dark theme in light app | `theme="dark"` |

### 2.3 Raw Button Usage

| Area | `<Button>` primitive | Raw `<button>` elements |
|------|---------------------|-------------------------|
| Primitives | N/A (is the primitive) | 0 |
| Composites | Some (ConfirmDialog) | Tabs, PeekPanel |
| Patterns | 0 Button imports | **4 raw `<button>` elements** |
| Routes | Some imports | **8 raw `<button>` elements** |

### 2.4 Oversized Route Files

| Route | Lines | Should be | Issue |
|-------|-------|-----------|-------|
| `channels.$channelId.tsx` | 503 | <200 | Chat view + info panel + all state in one file |
| `approval.tsx` | 482 | <200 | Template list + request list + tab logic + detail panels inline |
| `drive.tsx` | 364 | <200 | File list + sidebar + upload logic + context menu inline |
| `contacts.tsx` | 254 | <150 | Table + sidebar + profile panel in one file |

### 2.5 Mobile Overlay Pattern Inconsistency

5 different inline mobile overlay implementations found:

```
_workspace.tsx:     fixed inset-0 bottom-14 z-40  ← different z-index
drive.tsx:          fixed inset-0 z-50             ← z-50
approval.tsx:       fixed inset-0 z-50             ← z-50 (x2 occurrences)
contacts.tsx:       fixed inset-0 z-50             ← z-50
_assets.tsx:        fixed inset-0 z-50             ← z-50
```

No shared `<MobileOverlay>` component exists.

---

## 3. Constraint Mapping

### NGAC Permission Model → UI

| Backend Constraint | Current UI Behavior | Assessment |
|-------------------|--------------------|----|
| Policy Service is sole PDP | Drive checks access per-item ✅ | Correct |
| Owner vs Member permissions | Workspace settings shows roles ✅ | Correct |
| Channel-level access | Channel list filters by access ✅ | Correct |
| File-level NGAC nodes | Drive shows only accessible items ✅ | Correct |
| Approval workflow permissions | Template CRUD requires admin ⚠️ | UI doesn't clearly indicate permission state |

### Backend Constraints → UX Adaptation

| Constraint | UI Adaptation | Gap |
|-----------|--------------|-----|
| WebSocket for real-time | Chat uses WS store (15KB) | ⚠️ Store is monolithic |
| Presigned URL upload | Drive upload → confirm flow | ✅ Correct |
| Pagination | No cursor-based pagination in UI | ❌ All lists load everything |
| Quota system | No quota indicator in UI | ❌ Missing |

---

## 4. Inconsistency Report

### Wording Inconsistency

| Concept | Location A | Location B | Issue |
|---------|-----------|-----------|-------|
| Delete confirmation | Drive: "Delete" | Approval: "Confirm" | Different confirm labels |
| Empty state | Drive: custom EmptyState | Chat: inline text | No unified empty state |
| Loading | Some: `<Spinner>` primitive | Some: inline SVG spinner | Button has inline SVG, should use Spinner |

### Interaction Inconsistency

| Pattern | Chat | Drive | Approval | Contacts |
|---------|------|-------|----------|----------|
| Detail view | PeekPanel ✅ | PeekPanel (DriveContextPanel) ✅ | PeekPanel ✅ | PeekPanel ✅ |
| List selection | Click row → active class | Click row → active class | Click row → active class | Click row → active class |
| Create action | Modal ✅ | Modal (upload) | Modal ✅ | Invite form ⚠️ |
| Mobile nav | MobileNav bottom bar | Sidebar overlay | Sidebar overlay | Sidebar overlay |

### Layout Inconsistency

| Module | Desktop Layout | Notes |
|--------|---------------|-------|
| Chat | Rail + ChatList + Content + InfoPanel | 4 columns |
| Drive | Rail + DriveSidebar + Content + ContextPanel | 4 columns |
| Approval | Rail + (no sidebar) + Tabs + Content + DetailPanel | 3.5 columns (inline tabs) |
| Contacts | Rail + ContactsSidebar + Content + ProfilePanel | 4 columns |
| Assets | Separate layout (no Rail) | **Completely different layout** |

---

## 5. Summary of Findings

### Critical (must fix before standardization)

1. **Dead DESIGN.md** — root design doc describes a dark system that doesn't exist
2. **Dual token system** — legacy gray aliases create confusion, should migrate to pure M3
3. **Hardcoded Tailwind colors** — Contacts components bypass design tokens entirely
4. **Patterns/ is a dumping ground** — 19 components without organization

### High (standardization blockers)

5. **Raw `<button>` in 12 places** — bypasses Button/IconButton primitives
6. **Route files too large** — 4 routes exceed 300 lines
7. **No shared MobileOverlay** — 5 inline implementations with inconsistent z-index
8. **Assets layout divergence** — completely different from workspace layout

### Medium (consistency issues)

9. **Wording inconsistency** — delete vs confirm, empty states
10. **Button inline spinner** — Button.tsx has inline SVG instead of using `<Spinner>`
11. **No pagination** — all list views load everything
12. **EmojiPicker dark theme** — incorrect in light app
