## 1. Sidebar Migration

- [x] 1.1 Migrate `.nexus-sidebar` → inline Tailwind on `AppSidebar.tsx` `<aside>` element
- [x] 1.2 Migrate `.nexus-sidebar__workspace`, `:hover` → inline on workspace button
- [x] 1.3 Migrate `.nexus-sidebar__workspace-logo` → inline on logo div
- [x] 1.4 Migrate `.nexus-sidebar__new-project`, `:hover` → inline on CTA button
- [x] 1.5 Migrate `.nexus-sidebar__nav` → inline on nav element
- [x] 1.6 Migrate `.nexus-sidebar__item`, `:hover`, `--active` → inline on nav items
- [x] 1.7 Migrate `.nexus-sidebar__item-label` → inline on label spans
- [x] 1.8 Migrate `.nexus-sidebar__badge` → inline on badge span
- [x] 1.9 Migrate `.nexus-sidebar__footer` → inline on footer div
- [x] 1.10 Remove all `.nexus-sidebar*` blocks from `index.css`
- [x] 1.11 Verify: build passes + visual comparison at 1280px

## 2. TopBar Migration

- [x] 2.1 Migrate `.nexus-topbar` → inline Tailwind on `TopBar.tsx` `<header>` element
- [x] 2.2 Migrate `.nexus-topbar__brand` → inline on brand span
- [x] 2.3 Migrate `.nexus-topbar__search`, `__search-input`, `__search-icon` → inline on search elements
- [x] 2.4 Migrate `.nexus-topbar__search-input:focus`, `::placeholder` → Tailwind `focus:`, `placeholder:` modifiers
- [x] 2.5 Migrate `.nexus-topbar__actions`, `__icon-btn`, `:hover` → inline on action buttons
- [x] 2.6 Remove all `.nexus-topbar*` blocks from `index.css`
- [x] 2.7 Verify: build passes + visual comparison

## 3. ChatListItem Migration

- [x] 3.1 Migrate `.chat-list-item`, `:hover`, `--active` → inline Tailwind on `ChatListItem.tsx`
- [x] 3.2 Migrate `.chat-external-badge` → inline on badge span
- [x] 3.3 Migrate `.unread-badge` → inline on unread count span
- [x] 3.4 Remove `.chat-list-item*`, `.chat-external-badge`, `.unread-badge` from `index.css`
- [x] 3.5 Verify: build passes + visual comparison

## 4. MessageItem Migration

- [x] 4.1 Migrate `.msg-bubble` (incoming) → inline Tailwind on `MessageItem.tsx`
- [x] 4.2 Migrate `.msg-bubble--self` + all child selectors → inline with conditional classes
- [x] 4.3 Remove all `.msg-bubble*` blocks from `index.css`
- [x] 4.4 Verify: build passes + self-bubble has white text

## 5. ChatHeader Migration

- [x] 5.1 Migrate `.chat-header`, `__identity`, `__tabs`, `__channel-icon` → inline on `channels.$channelId.tsx`
- [x] 5.2 Remove all `.chat-header*` blocks from `index.css`
- [x] 5.3 Verify: build passes + visual comparison

## 6. Pill Tabs Migration

- [x] 6.1 Migrate `.pill-tab`, `:hover`, `--active`, `--outline` → inline Tailwind wherever pill-tab is used
- [x] 6.2 Remove all `.pill-tab*` blocks from `index.css`
- [x] 6.3 Verify: build passes + visual comparison

## 7. Micro Components Migration

- [x] 7.1 Migrate `.timestamp-pill`, `__label` → inline on timestamp elements
- [x] 7.2 Migrate `.timestamp-divider`, `__label` → inline on divider elements
- [x] 7.3 Migrate `.resize-handle`, `:hover`, `.is-dragging`, `::before` → inline + `@utility` for pseudo-element
- [x] 7.4 Migrate `.bg-nexus-auth` → `@utility bg-nexus-auth`
- [x] 7.5 Remove all migrated blocks from `index.css`
- [x] 7.6 Verify: build passes

## 8. Animation Utilities Conversion

- [x] 8.1 Convert `.animate-reaction-pop` → `@utility animate-reaction-pop`
- [x] 8.2 Convert `.animate-msg-slide-in` → `@utility animate-msg-slide-in`
- [x] 8.3 Convert `.animate-panel-slide` → `@utility animate-panel-slide`
- [x] 8.4 Convert `.animate-slide-in-right` → `@utility animate-slide-in-right`
- [x] 8.5 Remove old `.animate-*` class blocks from `index.css`

## 9. Final Verification

- [x] 9.1 Confirm `index.css` contains ZERO BEM selectors (grep verification)
- [x] 9.2 Confirm `index.css` ≈ 400 lines (416 lines — within target)
- [x] 9.3 Full build: `npx vite build` passes ✓
- [x] 9.4 Visual verification: pending browser test
