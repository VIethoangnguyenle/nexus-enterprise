# Design: Responsive-First UI + Tailwind CSS Refactor

## Architecture Decision

### Tailwind v4 as the primary styling system

The project already has `tailwindcss: ^4.2.4` and `@tailwindcss/vite` installed. The `index.css` uses `@import "tailwindcss"` and `@theme {}` for design tokens. **Tailwind IS the CSS framework** — the skill/docs just haven't caught up.

**Decision**: Embrace Tailwind v4 fully. Use utility classes for responsive behavior. Keep `@theme` tokens in CSS (correct pattern for v4). Reduce custom CSS to component-specific styles that can't be expressed as utilities.

### Responsive Strategy

```
┌─────────────────────────────────────────────────────────────┐
│ MOBILE-FIRST with Tailwind breakpoints                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Base styles = mobile (< 768px)                            │
│       ↓ md:  = tablet (≥ 768px)                            │
│       ↓ lg:  = desktop (≥ 1024px)                          │
│       ↓ xl:  = large (≥ 1280px)                            │
│                                                             │
│  Example:                                                   │
│  className="flex flex-col md:flex-row lg:gap-6"            │
│  = mobile: stacked                                          │
│  = tablet: side by side                                     │
│  = desktop: side by side with gap                          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Layout Pattern: Shell + Drawer

```
MOBILE (< 768px)                    DESKTOP (≥ 1024px)
┌──────────────────┐                ┌────┬─────────┬────────────┐
│   Content Area   │                │Rail│ListPanel│  Content   │
│   (full width)   │                │    │         │            │
│                  │                │ 56 │  240px  │   flex-1   │
│                  │                │ px │resizable│            │
│                  │                │    │         │            │
├──────────────────┤                │    │         │            │
│ ☰  📄  💬  📁  👤│                │    │         │            │
│   Bottom Nav     │                └────┴─────────┴────────────┘
└──────────────────┘
```

**Mobile navigation**: Sidebar becomes fixed bottom bar.
**ListPanel**: Hidden by default on mobile, opens as full-screen overlay.
**Content**: Takes full width, with bottom padding for nav bar.

## Breakpoint System

Using Tailwind v4 defaults (already configured):

| Prefix | Min-width | Use case |
|--------|-----------|----------|
| *(none)* | 0px | Mobile default |
| `sm:` | 640px | Large phone / small tablet |
| `md:` | 768px | Tablet portrait |
| `lg:` | 1024px | Tablet landscape / laptop |
| `xl:` | 1280px | Desktop |
| `2xl:` | 1536px | Large monitor |

**Primary breakpoints for this project**: `md:` (tablet) and `lg:` (desktop).

## Per-Module Responsive Design

### 1. Workspace Shell (`_workspace.tsx`)

**Current**: `flex h-screen` → 3 fixed columns
**Target**:
- Mobile: Content only + bottom nav + hamburger for ListPanel
- Tablet: Collapsible ListPanel + content
- Desktop: Rail + ListPanel + Content (current)

```tsx
// Mobile: bottom nav, no sidebar
// md: sidebar collapses
// lg: full 3-column
<div className="flex flex-col lg:flex-row h-screen">
  {/* Rail: hidden on mobile, bottom bar via CSS */}
  <LarkRail className="hidden lg:flex" />

  {/* Mobile bottom nav */}
  <MobileNav className="fixed bottom-0 inset-x-0 lg:hidden" />

  {/* ListPanel: overlay on mobile, inline on desktop */}
  {showList && (
    <div className="fixed inset-0 z-40 lg:relative lg:z-auto lg:w-60">
      <ListPanel />
    </div>
  )}

  {/* Content */}
  <main className="flex-1 min-w-0 pb-14 lg:pb-0">
    <Outlet />
  </main>
</div>
```

### 2. Chat/Channel View

**Current**: Fixed header + messages + editor
**Target**: Same but responsive sizing

```tsx
// Header: reduce padding on mobile
<div className="px-3 md:px-4 h-10 md:h-11">

// Messages: full width on mobile
<div className="flex-1 overflow-y-auto px-3 md:px-4">

// Editor: fixed bottom, responsive padding
<div className="px-3 md:px-4 py-2">
```

### 3. Drive (`_drive.tsx`)

**Current**: Fixed sidebar + table
**Target**:
- Mobile: No sidebar, card/list view
- Tablet: Collapsed sidebar, reduced table columns
- Desktop: Full sidebar + full table

### 4. Assets (`_assets.tsx`)

**Current**: Fixed sidebar + dashboard/list
**Target**: Same pattern as Drive

## CSS Refactor Strategy

### What stays in `index.css`

1. `@theme {}` block — all design tokens (correct Tailwind v4 pattern)
2. `@utility` definitions — custom text utilities
3. `@keyframes` — animations
4. Complex component styles that can't be utilities (TipTap editor, scrollbar, etc.)
5. Scrollbar styles

### What moves to Tailwind utilities in JSX

1. **All `@media` queries** — replace with `md:` / `lg:` prefixes
2. **Simple layout classes** — `.lark-sidebar` width/flex → Tailwind
3. **Simple visual classes** — colors, padding, borders → Tailwind
4. **Responsive overrides** — remove `!important` hacks

### Migration order

```
Phase 1: AGENTS.md + Skills (documentation)
Phase 2: CSS infrastructure (index.css cleanup)
Phase 3: Layout shells (_workspace, _drive, _assets)
Phase 4: Module views (channels, documents, drive items)
Phase 5: Components (primitives, composites)
Phase 6: Verification (all breakpoints)
```

## Touch & Interaction Rules

| Rule | Implementation |
|------|---------------|
| Touch targets ≥ 44px | `min-h-11 min-w-11` on mobile buttons |
| No hover-only actions | Always provide `onClick` + `onTouchEnd` |
| Swipe gestures | Future enhancement, not in this change |
| Long press | Not needed currently |

## Testing Strategy

Visual verification at 4 widths:
1. **375px** (iPhone SE) — mobile
2. **768px** (iPad portrait) — tablet
3. **1280px** (laptop) — desktop
4. **1920px** (monitor) — large

Use browser DevTools responsive mode for each breakpoint.
