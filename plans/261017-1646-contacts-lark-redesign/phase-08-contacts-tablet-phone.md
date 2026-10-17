---
phase: 8
title: "Danh bạ tablet + phone"
status: pending
priority: P2
effort: "6h"
dependencies: [7]
---

# Phase 8: Danh bạ tablet (768–1023) + phone (< 768)

## Overview
Same categories, order and lazy loading as desktop (mockup §6, §7), on the phone shell. Reuses phase-07 components and
route search params; only layout differs. Spec: `contacts-documents-settings-screens`.

## Requirements
- Tablet: phone-shell pill search (`PhoneSearchBar`: ☰ menu, "Tìm trong danh bạ" focuses the screen's search, avatar →
  account sheet); category chips replace the left column (org: Phòng ban · Vai trò · Nhóm của tôi; personal: Thành viên ·
  Nhóm của tôi); profile = right sheet 380, `--color-overlay`, `--shadow-overlay`, `--color-scrim`, 280 ms expo in;
  scrim tap / ✕ / Esc closes, focus returns to the row.
- Phone tier 1: top bar of every list screen, title "Danh bạ", category tiles 64 (icon 40 on `--color-person-N-wash`,
  name, one-line description), shortcut "Phòng của tôi"; no people, no tree, no bottom bar. Personal: Thành viên, Nhóm của
  tôi. No Bên ngoài / Danh bạ chia sẻ rows.
- Tier 2: back bar with current level name + shortened parent path (last segment full); back = up one level, not home;
  search input 16px (no zoom) searching the whole visible scope.
- Tier 3: full-screen profile, floating back button on the cover (44 hit area), three actions 64 high split evenly;
  edge swipe / OS back return (history entries per level via route params).
- Lazy load + skeletons identical to desktop.

## Architecture
- `ContactsScreen` picks layout by `useBottomBarLayout`/existing breakpoint hooks (`hooks/usePhone.ts`); tier stack is
  derived from route params (`cat`, `dept`, `role`, `group`, `person`), so browser back works without extra state.
- New: `ContactsCategoryHome.tsx` (phone tier 1), `ContactsChips.tsx` (tablet), `ContactProfileSheet.tsx` (tablet sheet,
  wraps `ContactProfile`).

## Related Code Files
- Modify: `frontend/src/components/contacts/ContactsScreen.tsx`, `ContactsScreen.test.tsx`
- Create: `ContactsCategoryHome.tsx`, `ContactsChips.tsx`, `ContactProfileSheet.tsx` + tests (all `components/contacts/`)
- Read-only reuse: `components/patterns/PhoneSearchBar.tsx`, `phone-shell-harness.tsx` (test harness)

## Implementation Steps (TDD)
1. Tests with `phone-shell-harness`: tier 1 shows 3 org tiles + shortcut, 2 personal tiles, no people request beyond
   contacts; opening a tile enables only that query; back from A1 goes to A then home; profile full screen with three
   actions; tablet chips switch categories; sheet closes on Esc and returns focus; input font-size 16px.
2. Implement; `npm test`, `npm run lint`, `npm run build`.
3. `/browse` at 800 and 390 against mockup §6/§7; `make dev-stop` after.

## Success Criteria
- Screens at 800 and 390 match mockup; OS back walks the stack one level at a time.

## Risk Assessment
- Breakpoint logic duplicated between desktop/phone (Low×Med): one layout switch in `ContactsScreen`, shared children.
- iOS zoom on focus (Low×Low): enforced 16px, covered by the existing phone rule (#25).
- Rollback: revert with phase 07.
