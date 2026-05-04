## Tasks

### Phase 1: Documentation — Agent Rules & Skills
- [x] Update `AGENTS.md` — Add "Responsive-First UI" section to non-negotiable rules
- [x] Update `frontend-best-practices/references/styling.md` — Fix Tailwind contradiction, document actual stack
- [x] Create `frontend-best-practices/references/responsive.md` — Detailed responsive patterns, breakpoints, per-module guidance
- [x] Create `.agent/skills/tailwindcss/SKILL.md` — NGAC-specific Tailwind v4 skill (CSS-first config, `@theme`, `@utility`, responsive modifiers)
- [x] Update `frontend-best-practices/SKILL.md` — Add responsive section reference

### Phase 2: CSS Infrastructure
- [x] Refactor `index.css` — Remove `@media (max-width: 768px)` blocks, convert to Tailwind responsive classes in JSX
- [x] Audit and remove `!important` overrides in mobile CSS
- [x] Keep `@theme`, `@utility`, `@keyframes` — they're correct Tailwind v4 patterns
- [x] Convert `.lark-sidebar` responsive rules to Tailwind utilities on components
- [x] Fix `.lark-sidebar` CSS `display: flex` conflict with Tailwind `hidden` utility
- [x] Convert `.chat-header` responsive rules to Tailwind utilities
- [x] Convert `.app-rail-mobile`, `.list-panel-mobile`, `.workspace-main-mobile` to Tailwind
- [x] Convert `.side-panel-mobile`, `.header-btn-text`, `.editor-toolbar-wrap` to Tailwind
- [x] Add `scrollbar-none` Tailwind utility
- [x] Add PeekPanel desktop-width CSS custom property rule

### Phase 3: Layout Shell Responsive
- [x] `_workspace.tsx` — Mobile: bottom nav + content only; Tablet: collapsible panel; Desktop: 3-column
- [x] `_drive.tsx` — Mobile: full-width content; Tablet: collapsed sidebar; Desktop: sidebar + content
- [x] `_assets.tsx` — Mobile: full-width content; Tablet: collapsed sidebar; Desktop: sidebar + content
- [x] `_auth.tsx` — Already responsive (polish only: verify form spacing on small screens)

### Phase 4: Module View Responsive
- [x] `channels.$channelId.tsx` — Responsive header, message padding, editor layout
- [x] `documents.tsx` — Responsive document list with hidden columns on mobile
- [x] `drive/drive.tsx` — Mobile: responsive padding, hidden button text, scrollable tabs
- [x] `assets/list.tsx` — Responsive padding, scrollable filter bar
- [x] `assets/dashboard.tsx` — Responsive grid (1 → 2 → 4 columns)
- [x] `assets/requests.tsx` — Responsive header stacking, card layout
- [x] `assets/types.tsx` — Responsive grid (1 → 2 → 3 columns)

### Phase 5: Component Responsive
- [x] `LarkRail` component — Hidden on mobile, visible on lg+
- [x] `ListPanel` component — Overlay on mobile, inline on desktop
- [x] `DataTable` — Wrapped in overflow-x-auto for mobile scrolling
- [x] `PeekPanel` — Full-screen overlay on mobile, side panel on desktop (CSS custom property width)
- [x] `ChannelInfoPanel` — Full-screen overlay on mobile, side panel on desktop
- [x] `Modal` — Full-screen on mobile, centered dialog on md+
- [x] `DriveSidebar` — Hidden on mobile, visible on lg+
- [x] `DriveFileRow` — Hidden size/modified/actions columns on mobile
- [x] `MobileNav` — Bottom navigation for mobile (created in earlier session)
- [x] `EditorToolbar` — Horizontal scroll with hidden scrollbar on mobile

### Phase 6: Verification
- [x] Vite production build passes (0 errors)
- [x] Verified at 375px (iPhone SE) — sidebar hidden, bottom nav visible, full-width content
- [x] Verified at 768px (iPad portrait) — sidebar hidden, bottom nav visible, header side-by-side
- [x] Verified at 1280px (laptop) — full 3-column layout, sidebar visible, bottom nav hidden
- [x] No horizontal scroll at any breakpoint
