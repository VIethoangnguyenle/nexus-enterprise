# Tasks — Nexus Hub UI Migration

## Phase 1: Design System Foundation
- [x] 1.1 Rewrite `frontend/index.css` — Material 3 tokens, Manrope typography scale, spacing system
- [x] 1.2 Update `frontend/index.html` — swap Inter → Manrope font import, update title
- [x] 1.3 Update Tailwind v4 `@theme` block with new color, spacing, font tokens

## Phase 2: Layout Architecture
- [x] 2.1 Create `TopBar.tsx` — brand, global search, notification/help/settings icons, avatar
- [x] 2.2 Create `AppSidebar.tsx` — 280px sidebar with workspace switcher, nav items, footer
- [x] 2.3 Rewrite `_workspace.tsx` layout — TopBar + AppSidebar + content area (replace LarkRail)
- [x] 2.4 Update `MobileNav.tsx` — adapt bottom nav for new design tokens
- [x] 2.5 Remove or deprecate `LarkRail.tsx`

## Phase 3: Auth Flow
- [x] 3.1 Rewrite `AuthLayout.tsx` — light mode, centered card, radial gradient background
- [x] 3.2 Rewrite `login.tsx` — single email input → Continue, Google/SSO buttons, Nexus Hub style
- [x] 3.3 Restyle OTP verification — 6-digit inputs, Nexus Hub card layout, resend link
- [x] 3.4 Create "Welcome Back" splash screen (`/_auth/welcome.tsx`) — logo, progress bar, auto-redirect
- [x] 3.5 Create workspace selection page (`/_auth/workspace-select.tsx`) — Organizations vs Personal columns
- [x] 3.6 Create onboarding wizard (`/_auth/onboarding.tsx`) — Step 1: Org info, Step 2: Invite team (optional)

## Phase 4: Backend — User Profile & Workspace Type
- [x] 4.1 DB migration: add `title`, `department`, `location`, `avatar_url` columns to `users` table
- [x] 4.2 DB migration: add `type` column to `workspaces` table (`personal` | `organization`)
- [x] 4.3 Update Auth REST handler: `PATCH /api/me/profile` for updating profile fields
- [x] 4.4 Create Auth REST endpoint: `GET /api/workspaces/:id/contacts` — aggregated member profiles
- [x] 4.5 Update Auth domain service: profile update + contacts listing logic
- [x] 4.6 Update Auth store: new queries for profile fields
- [x] 4.7 Update Workspace gRPC: return `type` field in workspace proto
- [x] 4.8 Update workspace creation: auto-set `type=personal` on registration, `type=organization` on user-created

## Phase 5: Messaging UI
- [x] 5.1 Restyle `ChatListItem.tsx` — Material 3 cards, section headers (Pinned/Departments/DMs), avatars
- [x] 5.2 Restyle `MessageItem.tsx` — light bubbles (surface-bright for others, primary for self), file cards
- [x] 5.3 Restyle `ChatInput.tsx` — rich toolbar (B/I/S, lists), bottom actions (emoji, attach, send), light bg
- [x] 5.4 Update chat header — workspace name, member count, online status

## Phase 6: Drive Module
- [x] 6.1 Create `TreeView.tsx` component — recursive folder tree with expand/collapse, active highlight
- [x] 6.2 Update Drive layout — left panel tree + right panel file grid with breadcrumb
- [x] 6.3 Add grid/list toggle view switcher
- [x] 6.4 Restyle file cards — Nexus Hub file icons, metadata, hover states

## Phase 7: Contacts Module (Full Stack)
- [x] 7.1 Create frontend route `/_workspace/contacts.tsx`
- [x] 7.2 Create `ContactCard.tsx` component — avatar, name, title, department/location badges, actions
- [x] 7.3 Create `ContactsFilterBar.tsx` — department dropdown, location dropdown, More Filters
- [x] 7.4 Create `useContacts.ts` hook — TanStack Query for contacts API
- [x] 7.5 Create contacts page layout — header, filter bar, responsive card grid
- [x] 7.6 Add "Contacts" nav item to `AppSidebar.tsx`
- [x] 7.7 Wire "Message" button on contact card → navigate to DM channel

## Phase 8: Polish & Verification
- [x] 8.1 Responsive testing: 375px (mobile), 768px (tablet), 1280px (desktop)
- [x] 8.2 Visual self-correction checklist (spacing, alignment, hierarchy, balance)
- [x] 8.3 Cross-module navigation testing (sidebar → chat → drive → contacts → back)
- [x] 8.4 Build verification: `npm run build` passes with zero errors
