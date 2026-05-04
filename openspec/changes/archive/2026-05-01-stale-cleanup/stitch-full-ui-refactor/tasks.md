# Tasks — Stitch Full UI Refactor

## Phase 0: Token Foundation ✅
- [x] 0.1 Audit primitives (Button, Input, Badge, Avatar, etc.) against Stitch source tokens
- [x] 0.2 Update primitives to use Material 3 tokens directly (remove legacy alias usage)
- [x] 0.3 Add any missing M3 tokens to index.css from DESIGN.md (verified — all present)

## Phase 1: Auth Flow (6 screens) ✅
- [x] 1.1 Fetch & extract: login screen → .stitch/designs/login.html
- [x] 1.2 Fetch & extract: welcome-back screen → .stitch/designs/welcome-back.html
- [x] 1.3 Fetch & extract: verification screen → .stitch/designs/verification.html
- [x] 1.4 Fetch & extract: onboarding-1, onboarding-2 → .stitch/designs/onboarding-{1,2}.html
- [x] 1.5 Refactor: login.tsx + _auth.tsx (AuthLayout merged) + OtpInput.tsx
- [x] 1.6 Refactor: welcome.tsx
- [x] 1.7 Refactor: onboarding.tsx
- [x] 1.8 workspace-select.tsx legacy token cleanup (bg-nexus-auth → bg-background)

## Phase 2: Workspace Selection (3 screens) ✅
- [x] 2.1 Fetch & extract: workspace-selection, workspace-tablet, workspace-mobile
- [x] 2.2 Refactor: workspace-select.tsx with bento grid, blur BG, group hover cards
- [x] 2.3 Build verified

## Phase 3: Workspace Shell (Critical Foundation) ✅
- [x] 3.1 Fetch & extract: nexus-chat screen → .stitch/designs/nexus-chat.html (27KB, full 3-col layout)
- [x] 3.2 Refactor: _workspace.tsx — bg-background, border-outline-variant/30, bg-surface-bright mobile overlay
- [x] 3.3 Refactor: AppSidebar.tsx — w-[280px], p-6, rounded-lg, active:bg-surface-container-highest
- [x] 3.4 Refactor: TopBar.tsx — rounded-lg search, text-h3 brand, p-2 action buttons
- [x] 3.5 Refactor: ListPanel.tsx — bg-surface-bright, border-outline-variant/30
- [x] 3.6 Refactor: MobileNav.tsx — border-outline-variant/30
- [x] 3.7 Build verified

## Phase 4: Contacts (2 screens) ✅
- [x] 4.1 Fetch & extract: contacts-directory.html (23KB), contacts-profile.html (26KB)
- [x] 4.2 Refactor: contacts.tsx — px-8 py-6 header, rounded-full member badge, breadcrumb, inline search
- [x] 4.3 Refactor: ContactsSidebar.tsx — w-64, bg-surface-container-lowest, bg-primary-fixed active, tree pl-9
- [x] 4.4 Refactor: ContactsTable.tsx — CSS Grid grid-cols-[auto_1.5fr_1fr_1.5fr_1fr], rounded-xl card, status dots
- [x] 4.5 Refactor: ContactProfilePanel.tsx — w-96, w-24 avatar, bg-surface-container-low details, rounded-xl buttons
- [x] 4.6 Build verified

## Phase 5: Chat (6 screens) ✅
- [x] 5.1 Fetch & extract: nexus-chat, engineering-dept
- [x] 5.2 Fetch & extract: chat-tablet, dept-chat-tablet, dept-chat-mobile, chat-list-mobile
- [x] 5.3 Refactor: channels.$channelId.tsx — h-[72px] header, w-12 rounded-xl icon, p-4 md:p-6 messages area
- [x] 5.4 Refactor: ChatList.tsx + ChatListItem.tsx — p-4 header, font-label-caps sections, Pin/Building2 icons
- [x] 5.5 Refactor: MessageItem.tsx — w-10 avatar, rounded-2xl/tl-sm bubbles, hover action bar
- [x] 5.6 Refactor: ChatEditor.tsx + EditorToolbar.tsx — rounded-xl container, bg-surface-container-low toolbar
- [x] 5.7 Refactor: ChannelInfoPanel.tsx (existing — deferred to later)
- [x] 5.8 Refactor: ChatInput.tsx + HoverActionBar.tsx + ReactionBar.tsx — Stitch tokens aligned
- [x] 5.9 Build verified ✅

## Phase 6: Drive (4 screens) ✅
- [x] 6.1 Fetch & extract: drive-finance.html (24KB, grid-cols-12 table, tree sidebar)
- [x] 6.2 Refactor: drive.tsx — p-6 header, h-8 px-3 rounded action buttons, bg-primary-container CTA
- [x] 6.3 Refactor: DriveSidebar.tsx — w-[260px], bg-surface-container-low, tree border-l border-outline-variant
- [x] 6.4 Refactor: DriveFileList.tsx — bg-surface-container-low rounded-lg table, grid grid-cols-12 header
- [x] 6.5 Refactor: DriveFileRow.tsx — grid grid-cols-12 rows, col-span-5 name, hover:bg-surface-container-high
- [x] 6.6 Refactor: DriveContextPanel.tsx — bg-surface-container-lowest, text-primary tabs, surface-container details
- [x] 6.7 Refactor: DriveFilterPills.tsx + DrivePreviewDialog.tsx — h-8 rounded-full pills, bg-primary-container active
- [x] 6.8 Refactor: DriveTreePanel.tsx — bg-surface-container-high active, border-l tree lines
- [x] 6.9 Build verified ✅

## Phase 7: Approvals / Assets (3 screens) ✅
- [x] 7.1 Fetch & extract: approval-dashboard.html (18KB, Kanban columns, surface-container cards)
- [x] 7.2 Refactor: _assets.tsx layout shell — w-[260px], bg-surface-container-low sidebar, bg-surface canvas
- [x] 7.3 Refactor: requests.tsx — font-h1 heading, error-container alerts, text-outline empty state
- [x] 7.4 Refactor: list.tsx — rounded-full filter pills, border-outline-variant, text-on-surface table cells
- [x] 7.5 Refactor: dashboard.tsx — primary-container/tertiary-container stat cards, font-h2 values
- [x] 7.6 Refactor: types.tsx — surface-variant icon bg, text-on-surface-variant labels
- [x] 7.7 Build verified ✅

## Phase 8: Remaining Pages ✅
- [x] 8.1 Refactor: settings.tsx — font-h1, text-on-surface-variant subtitle
- [x] 8.2 Contacts already Stitch-compliant (verified)
- [x] 8.3 Legacy token scan: 0 legacy tokens in routes/ ✅
- [x] 8.4 Legacy token scan: 0 legacy tokens in drive/ ✅
- [x] 8.5 Final build verified ✅

## Phase 9: Deep Component Sweep ✅
- [x] 9.1 ChannelInfoPanel.tsx — bg-surface-container-lowest, text-primary tabs, surface-container cards, primary-container avatars
- [x] 9.2 Breadcrumbs.tsx — text-on-surface active, text-primary navigable, text-on-surface-variant separator
- [x] 9.3 Timeline.tsx — text-on-surface title, text-on-surface-variant timestamp/body
- [x] 9.4 NotificationBell.tsx — surface-container-lowest dropdown, border-outline-variant, bg-primary-container unread, bg-error badge
- [x] 9.5 ChannelDrivePanel.tsx — surface-container-lowest bg, border-outline-variant, text-on-surface-variant labels
- [x] 9.6 InviteMemberForm.tsx — text-on-surface-variant label, text-tertiary success, text-error error
- [x] 9.7 ImagePreviewCard.tsx — surface-container bg, border-outline-variant, primary hover glow
- [x] 9.8 $assetId.tsx — font-h1 heading, error-container alerts, text-primary transitions
- [x] 9.9 request/new.tsx — urgency M3 colors, font-h1 heading, surface-container-high active state
- [x] 9.10 FINAL AUDIT: `grep` — 0 legacy tokens across entire frontend/src ✅
- [x] 9.11 Build verified ✅ (769ms)

---
## 🎉 REFACTOR COMPLETE — All 9 phases finished, 0 legacy tokens remaining.
