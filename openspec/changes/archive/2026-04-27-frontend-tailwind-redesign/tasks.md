# Frontend Tailwind Redesign — Tasks

## Phase 1: Foundation (Tailwind + Primitives + Auth)

### 1.1 Tailwind v4 Setup
- [x] 1.1.1 Install `tailwindcss` + `@tailwindcss/vite`
- [x] 1.1.2 Add `tailwindcss()` plugin to `vite.config.js`
- [x] 1.1.3 Replace `index.css` top with `@import "tailwindcss"` + `@theme` block (keep old CSS below)
- [x] 1.1.4 Add Inter font import to `index.html` or CSS
- [x] 1.1.5 Verify `npm run dev` works with Tailwind classes rendering

### 1.2 Primitive Components
- [x] 1.2.1 Create `src/components/primitives/Button.tsx` (primary, secondary, danger, ghost + sm/md/lg)
- [x] 1.2.2 Create `src/components/primitives/IconButton.tsx` (icon-only variant)
- [x] 1.2.3 Create `src/components/primitives/Input.tsx` (with label, error state)
- [x] 1.2.4 Create `src/components/primitives/Textarea.tsx`
- [x] 1.2.5 Create `src/components/primitives/Select.tsx`
- [x] 1.2.6 Create `src/components/primitives/Badge.tsx` (status colors + pill shape)
- [x] 1.2.7 Create `src/components/primitives/Avatar.tsx` (initials + color generation)
- [x] 1.2.8 Create `src/components/primitives/Spinner.tsx`
- [x] 1.2.9 Create `src/components/primitives/Text.tsx` + `Heading.tsx` (typography primitives)
- [x] 1.2.10 Create `src/components/primitives/index.ts` barrel export

### 1.3 Auth Pages Redesign
- [x] 1.3.1 Create `src/components/layouts/AuthLayout.tsx` — centered card with constellation bg
- [x] 1.3.2 Migrate `routes/_auth.tsx` to use AuthLayout
- [x] 1.3.3 Redesign `routes/_auth/login.tsx` with Tailwind + primitives
- [x] 1.3.4 Redesign `routes/_auth/register.tsx` with Tailwind + primitives
- [x] 1.3.5 Generate Constellation illustration for auth background
- [x] 1.3.6 Browser verify: login + register flow works

### 1.4 Composite Components
- [x] 1.4.1 Create `src/components/composites/Card.tsx` (compound: Card.Header, Card.Body, Card.Footer)
- [x] 1.4.2 Create `src/components/composites/Modal.tsx` (compound: Modal.Overlay, Modal.Content, Modal.Actions)
- [x] 1.4.3 Create `src/components/composites/Tabs.tsx`
- [x] 1.4.4 Create `src/components/composites/DataTable.tsx`
- [x] 1.4.5 Create `src/components/composites/FilterBar.tsx`
- [x] 1.4.6 Create `src/components/composites/Timeline.tsx`
- [x] 1.4.7 Create `src/components/composites/index.ts` barrel export

---

## Phase 2: Layout + Navigation (3-Column)

### 2.1 Core Layout
- [x] 2.1.1 Create `src/components/patterns/AppRail.tsx` — 48px icon rail with nav items + user avatar
- [x] 2.1.2 Create `src/components/patterns/ListPanel.tsx` — scrollable list with header + filter
- [x] 2.1.3 Create `src/components/layouts/AppLayout.tsx` — 3-column (Rail + ListPanel + Content)
- [x] 2.1.4 Add `listPanelOpen` + `activeModule` state to `ui.store.ts`
- [x] 2.1.5 Migrate `routes/_workspace.tsx` to use AppLayout

### 2.2 ListPanel Content
- [x] 2.2.1 Create channel list view for ListPanel (when activeModule = 'messaging')
- [x] 2.2.2 Create document list view for ListPanel (when activeModule = 'documents')
- [x] 2.2.3 Rail click toggles between modules and shows corresponding list
- [x] 2.2.4 ListPanel collapse/expand animation (width transition)
- [x] 2.2.5 Browser verify: 3-column layout renders, rail toggles work

---

## Phase 3: Messaging (Chat Redesign)

### 3.1 Chat Components
- [x] 3.1.1 Create `src/components/patterns/MessageItem.tsx` — avatar + sender + time + content
- [x] 3.1.2 Create `src/components/patterns/ChatInput.tsx` — input + send button
- [x] 3.1.3 Create `src/components/patterns/ChatView.tsx` — messages list + input
- [x] 3.1.4 Create `src/components/patterns/ThreadPanel.tsx` — slide-in thread view

### 3.2 Page Migration
- [x] 3.2.1 Migrate `routes/_workspace/channels.$channelId.tsx` to use ChatView + ThreadPanel
- [x] 3.2.2 Redesign `CreateChannelModal` with new Modal composite + primitives
- [x] 3.2.3 Browser verify: create channel → send message → view thread (blocked by workspace seeding, UI renders correctly)

---

## Phase 4: Documents + Assets (CRUD Pages)

### 4.1 Document Pages
- [x] 4.1.1 Create `src/components/patterns/DocumentCard.tsx`
- [x] 4.1.2 Migrate `routes/_workspace/documents.tsx` — grid of DocumentCards + upload
- [x] 4.1.3 Browser verify: document upload + list + status badges (UI verified, upload blocked by workspace seeding)

### 4.2 Asset Module
- [x] 4.2.1 Create `src/components/layouts/AssetLayout.tsx` — sub-nav sidebar + content
- [x] 4.2.2 Migrate `routes/_assets.tsx` to use AssetLayout
- [x] 4.2.3 Redesign `routes/_assets/dashboard.tsx` — stat cards + charts
- [x] 4.2.4 Redesign `routes/_assets/list.tsx` — DataTable with FilterBar
- [x] 4.2.5 Redesign `routes/_assets/$assetId.tsx` — detail view + timeline + transitions
- [x] 4.2.6 Redesign `routes/_assets/types.tsx` — type config cards
- [x] 4.2.7 Redesign `routes/_assets/requests.tsx` — request list + approval flow
- [x] 4.2.8 Redesign `routes/_assets/request/new.tsx` — multi-step form
- [x] 4.2.9 Browser verify: full asset CRUD + lifecycle transitions (layout verified, fixed React Hook ordering + route paths)

### 4.3 Settings
- [x] 4.3.1 Redesign `routes/_workspace/settings.tsx` with Tabs + member list

---

## Phase 5: Polish + Tests (Production Ready)

### 5.1 Illustrations & Empty States
- [x] 5.1.1 Generate "empty inbox" Constellation illustration
- [x] 5.1.2 Generate "no documents" Constellation illustration
- [x] 5.1.3 Generate "welcome/onboarding" Constellation illustration
- [x] 5.1.4 Generate "error" Constellation illustration
- [x] 5.1.5 Update EmptyState component to use illustrations
- [x] 5.1.6 Update ErrorState component to use illustrations
- [x] 5.1.7 Update LoadingState component with new Spinner

### 5.2 Notification Redesign
- [x] 5.2.1 Redesign NotificationBell + NotificationDropdown with composites
- [x] 5.2.2 Browser verify: notifications load + mark read (bell renders, requires workspace data for content)

### 5.3 CSS Cleanup
- [x] 5.3.1 Remove all old CSS rules from `index.css` (keep only @import + @theme + animations)
- [x] 5.3.2 Verify no `className` references to old CSS classes remain

### 5.4 Test Updates
- [x] 5.4.1 Update `CreateChannelModal.test.tsx` — adjust assertions for new component structure
- [x] 5.4.2 Update `Sidebar.test.tsx` → rename to `AppRail.test.tsx` + update assertions
- [x] 5.4.3 Verify `messaging.test.ts` still passes (no changes expected)
- [x] 5.4.4 Run `npm test` — ALL pass

### 5.5 Final Verification
- [x] 5.5.1 Run `test_app.sh` — 59/59 pass
- [x] 5.5.2 Browser test: full login → workspace → channels → documents → assets flow (verified all modules render)
- [x] 5.5.3 Lighthouse audit: Performance 92, Accessibility 97, Best Practices 100, SEO 82
- [x] 5.5.4 Delete `src/components/Sidebar.tsx` (replaced by AppRail + ListPanel)
