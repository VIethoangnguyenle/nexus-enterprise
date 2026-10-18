---
phase: 2
title: "Bottom bar and shell"
status: completed
priority: P1
effort: "2.5d"
dependencies: [1, 4]
---

# Phase 02: Bottom bar and shell

## Overview
Replace the ☰ drawer and Chat-only bar with one floating bottom bar for every module (variant A), the Thêm sheet,
Sửa thanh đáy, the workspace tile in the search pill, Chat chips with Mục, scroll-compact and quick jump.

## Requirements
- Functional: bar on every list screen below 1024px; hidden on detail screens, settings pages and with the keyboard
  open; 4 person-chosen slots + Thêm; defaults by workspace kind; per-workspace bar preference (phase 04 store);
  Thêm = workspace block + module grid + Thông báo + Quản trị (if allowed); long-press bar/Thêm → Sửa thanh đáy;
  tap current slot scrolls to top; Chat chips Tất cả · Mục · Chưa đọc · Được nhắc · Trực tiếp · Nhóm (Mục renders the
  existing sections view); long-press Chat slot → quick jump (Trực tiếp, Được nhắc, Thread đang theo dõi, Đã gắn sao with
  counts); compact bar on scroll down > 24px; + button beside the bar per module.
- Non-functional: badges keep current counting rules (`unreadByLevel`, approvals pending, notifications);
  reduced motion = no slide/fold; a11y: `nav aria-label="Phân hệ"`, `aria-current`, counts in labels.

## Architecture
`MobileNav` becomes `ModuleBar` (+ `ModuleBarCompact` state via a scroll observer on the screen's scroll root).
`PhoneSearchBar` loses ☰: left = workspace tile (opens the new `WorkspaceSwitcherSheet`, built from the
`WorkspaceBlock` that lives in `MainMenuDrawer` today), right = avatar (opens Tôi, phase 05; until then the existing
AccountSheet). `MainMenuDrawer` deleted only after the switcher sheet and the Thêm sheet carry everything it did.
Bar slots come from `nav-destinations.ts` (`mainDestinations(isPersonal)`) filtered by the `mobile.bar` setting
(phase 04, `useServerSettings('workspace')`). Chat chips are phone-only views of the Chat home; the desktop routes
`/channels/mentions` and `/channels/starred` stay exactly as they are (they are desktop panes, not redirects), and
existing `?tab=` links keep resolving to the matching chip.

## Related Code Files
- Modify: `frontend/src/components/patterns/MobileNav.tsx`, `PhoneSearchBar.tsx`, `frontend/src/lib/nav-destinations.ts`, `frontend/src/routes/_workspace.tsx`, `frontend/src/components/spaces/HomeView.tsx`, `NewChatButton.tsx`, `frontend/src/hooks/usePhone.ts`
- Create: `frontend/src/components/patterns/ModuleBar.tsx`, `MoreSheet.tsx`, `WorkspaceSwitcherSheet.tsx`, `EditBarScreen.tsx`, `ChatQuickJump.tsx`, `frontend/src/hooks/useModuleBar.ts` (setting + defaults), tests beside each
- Delete: `frontend/src/components/patterns/MainMenuDrawer.tsx` (+ test) once nothing imports it

## Implementation Steps
1. Tests first: defaults per kind; preference order respected; Quản trị/Phê duyệt/Tài sản never offered in personal; bar hidden on `isPhoneDetailPath`; counts in aria-labels; compact on scroll; long-press opens quick jump.
2. `useModuleBar` reading/writing the `mobile.bar` preference (phase 04), falling back to defaults when unset or a slot is not allowed.
3. Build ModuleBar, MoreSheet (move WorkspaceBlock in), EditBarScreen (drag reorder with keyboard alternative ↑/↓ buttons).
4. Chat chips in HomeView; Mục chip renders the existing sections list; desktop routes untouched (test asserts desktop `/channels/mentions` still renders its pane).
5. Remove ☰ from PhoneSearchBar, add workspace tile; delete MainMenuDrawer.
6. Update e2e at 390px for both workspace kinds.

## Success Criteria
- [ ] Any module reachable in one tap from any list screen for the 4 slots, two taps via Thêm otherwise
- [ ] Personal workspace never shows Phê duyệt, Tài sản, Quản trị anywhere in the shell
- [ ] Bar preference survives reload and is separate per workspace
- [ ] No ☰ drawer remains; vitest + lint + build green

## Risk Assessment
Bottom bar overlapping content or the composer → every list screen pads bottom 108 + safe-area (existing token);
e2e checks last row visible. Old deep links to Chat tabs → still resolve (desktop panes kept, `?tab=` mapped to chips).
A cached old shell in the PWA → the existing "Có bản mới" prompt; the e2e checks it appears after deploy.

## Rollback
Frontend-only; revert the release. The `mobile.bar` rows are ignored by the old version.
