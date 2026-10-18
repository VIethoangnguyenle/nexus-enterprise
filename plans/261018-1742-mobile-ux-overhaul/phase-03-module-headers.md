---
phase: 3
title: "Module headers and one search entry"
status: completed
priority: P2
effort: "1d"
dependencies: [2]
---

# Phase 03: Module headers and one search entry

## Overview
Apply the module header grammar (mockup §2) to Chat, Lịch, Tài liệu, Phê duyệt, Tài sản, Danh bạ on phones.

## Requirements
- Functional: the top pill is the module's only search (in-page search fields hidden below 768px); title row = title
  + at most one secondary button + ⋯; no subtitle line on phones; tab/chip rows scroll with edge fade and keep the
  active tab in view; ⋯ menu's first row "Cài đặt <module>" (not in Danh bạ); every module's create action is the +
  beside the bar (Phê duyệt "Tạo đề nghị", Lịch "Sự kiện mới", Tài sản "Thêm tài sản"); Tài sản pill reads "Tìm tài sản".
- Non-functional: desktop headers unchanged.

## Architecture
A shared `ModuleHeader` (title, secondary, menu items) and `ScrollTabs` (fade mask + `scrollIntoView` on change).
`PhoneSearchBar.findSearch` keeps targeting `[data-module-search]`; module screens hide their own field with
`max-md:hidden` but keep it in the DOM for the pill to focus (until phase 08 replaces it).

## Related Code Files
- Create: `frontend/src/components/patterns/ModuleHeader.tsx`, `ScrollTabs.tsx` (+ tests)
- Modify: `frontend/src/components/drive/DriveToolbar.tsx`, `DriveScreen.tsx`, `frontend/src/components/approval/ApprovalScreen.tsx`, `frontend/src/components/assets/AssetsScreen.tsx`, calendar screen header, `frontend/src/components/contacts/ContactsScreen.tsx`, `frontend/src/lib/nav-destinations.ts` (`searchHintOf`)

## Implementation Steps
1. Tests: one search field visible at 390px per module; ⋯ first item; ScrollTabs keeps active in view; no subtitle on phone.
2. Build ModuleHeader + ScrollTabs; migrate each module screen.
3. Move each module's create button into the + slot via a `useModuleCreateAction` registry read by ModuleBar.

## Success Criteria
- [ ] No phone screen shows two search fields (e2e at 390px across six modules)
- [ ] No clipped tab label without fade
- [ ] Desktop snapshots unchanged

## Risk Assessment
Hidden-but-focusable search may confuse screen readers → `aria-hidden` not used; the field stays reachable and is
labelled; replaced entirely by phase 08 overlay.
