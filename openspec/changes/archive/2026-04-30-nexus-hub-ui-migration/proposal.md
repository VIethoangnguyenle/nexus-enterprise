## Why

The NGAC platform currently uses a dark-mode, Inter-font Lark-inspired UI. The product owner has designed a new enterprise identity — **Nexus Hub** — in Google Stitch (project ID `14852434379132121789`) with 17 screens covering auth, workspace, messaging, drive, and contacts. This change adopts Nexus Hub as the **design source of truth**, migrating the entire frontend to its light-mode Material 3 design language and building missing features to match the vision.

Key gaps between current state and Nexus Hub design:
- **Auth flow** is dark and minimal — Nexus Hub has a polished light login card with Google/SSO buttons, 6-digit OTP verification, and a "Welcome Back" loading splash
- **No workspace selection screen** — NGAC auto-picks the first workspace; Nexus Hub has an explicit selection page with Organizations vs Personal columns
- **No onboarding flow** — Nexus Hub has a 2-step wizard (Create Organization → Invite Team)
- **No contacts module** — Nexus Hub has a full Contacts Directory with card grid, department/location badges, and direct message actions
- **Drive lacks folder tree** — Nexus Hub shows a hierarchical Organization Structure tree in the left panel

## What Changes

### Frontend — Design System
- **BREAKING**: Replace dark 14-shade grayscale with Nexus Hub Material 3 light surface system
- **BREAKING**: Switch font from Inter → Manrope
- Update all color tokens, typography utilities, spacing tokens in `index.css`

### Frontend — Auth Flow
- Rewrite `AuthLayout.tsx` → light mode centered card with radial gradient background
- Rewrite `login.tsx` → Nexus Hub login design (single input, Continue →, Google/SSO buttons)
- Update `OtpInput.tsx` → 6-digit with Nexus Hub styling
- **NEW**: "Welcome Back" splash screen after OTP verification

### Frontend — Workspace Selection
- **NEW**: `/_auth/workspace-select.tsx` route — displayed after login when user has workspaces
- Two-column layout: Organizations | Personal
- Workspace cards with name, tier badge, member count
- Bottom actions: "Join an Organization" + "+ Create Workspace"

### Frontend — Onboarding Wizard
- **NEW**: `/_auth/onboarding.tsx` route — 2-step wizard
- Step 1: Company Name, Logo upload, Workspace URL
- Step 2: Invite team by email (optional, can skip)

### Frontend — Messaging UI
- Rewrite sidebar navigation → `AppSidebar.tsx` (280px, workspace switcher, nav items)
- **NEW**: `TopBar.tsx` — global header with brand, search, notifications, avatar
- Update `ChatListItem.tsx` — Material 3 card style with avatars, online dots, section headers
- Update `MessageItem.tsx` — light bubble style (others: surface-bright, self: primary blue)
- Update `ChatInput.tsx` — rich toolbar (B/I/S, lists) + bottom actions row

### Frontend — Drive Module
- **NEW**: TreeView component for folder hierarchy
- Update layout with breadcrumb navigation, grid/list toggle, file type filter tabs

### Frontend — Contacts Module (NEW)
- **NEW**: `/contacts` route with Contacts Directory page
- Grid of contact cards (avatar, name, title, department/location badges, message/email actions)
- Filter bar: department dropdown, location dropdown, "More Filters" button
- Responsive: 1-col mobile → 2-col tablet → 3-col desktop

### Backend — User Profile Extension
- Extend `users` table: add `title`, `department`, `location`, `avatar_url` columns
- New REST endpoints on Auth service: `PATCH /api/me/profile`, `GET /api/workspaces/:id/contacts`
- Update `ListMembers` to return enriched user profile data

### Backend — Workspace Type
- Add `type` column to `workspaces` table (`personal` | `organization`)
- Auto-create personal workspace on user registration
- Update `ListWorkspaces` to return workspace type

## Capabilities

### New Capabilities
- `nexus-auth-flow`: Light-mode auth screens — login card, 6-digit OTP verification, Welcome Back splash, workspace selection, 2-step onboarding wizard
- `nexus-contacts-module`: Full-stack contacts directory — card grid with profile data, department/location filtering, message/email actions
- `nexus-drive-tree`: Hierarchical folder tree navigation in Drive sidebar
- `nexus-layout-architecture`: TopBar global header + AppSidebar (280px) replacing LarkRail

### Modified Capabilities
- `design-system`: Dark grayscale → Material 3 light surfaces, Inter → Manrope, all tokens updated
- `messaging-ui`: Chat list items, message bubbles, input bar restyled to Nexus Hub design
- `workspace-management`: Workspace type differentiation (personal vs org), explicit selection screen

## Impact

- **Routes**: New `workspace-select`, `onboarding`, `contacts` routes; modified `_auth`, `_workspace` layouts
- **Components**: New `TopBar`, `AppSidebar`, `TreeView`, `ContactCard`, `OnboardingWizard`; rewritten `AuthLayout`, `LarkRail`, `MessageItem`, `ChatInput`, `OtpInput`
- **CSS**: Complete rewrite of `index.css` — new token system
- **Backend**: Auth service extended (user profile fields + contacts endpoint), Workspace service (type field)
- **Database**: Migration for `users` (4 new columns) and `workspaces` (1 new column)
