---
phase: 8
title: "Danh bạ tablet + phone"
status: "completed (not committed)"
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
3. `/browse` at 800 and 390 against mockup §6/§7 with the seeded org (phase 04); `make dev-stop` after.

## Success Criteria
- Screens at 800 and 390 match mockup; OS back walks the stack one level at a time.

## Risk Assessment
- Breakpoint logic duplicated between desktop/phone (Low×Med): one layout switch in `ContactsScreen`, shared children.
- iOS zoom on focus (Low×Low): enforced 16px, covered by the existing phone rule (#25).
- Rollback: revert with phase 07.

## Todo / Status (2026-10-18)
- [x] One layout switch in `ContactsScreen` (`useBottomBarLayout` < 1024, `usePhone` < 768) over shared children; the whole stack is derived from the address, so back/forward and links work without extra state.
- [x] Tablet (768-1023): title "Danh bạ" + `ContactsChips` (Phòng ban · Vai trò · Nhóm của tôi; personal: Thành viên · Nhóm của tôi; `aria-pressed`, 36 high, scrolls inside itself), the same middle column, the top bar's search focuses the column's search box. Profile = `ContactProfileSheet`: right sheet 380 on `--color-scrim`, `--color-overlay` + `--shadow-overlay`, 280 ms expo from the right (fade under reduced motion), a modal layer in the body (page inert, focus moves in and is trapped); scrim, ✕ and Esc close it and focus returns to the row.
- [x] Phone tier 1 `ContactsCategoryHome`: title, a 64 high tile per category (icon 40 on the person wash, name, one line, chevron), the Phòng của tôi shortcut, 16 px gutters; it lists no one and asks for nothing but the directory. A hidden control carries `data-module-search`, so the top bar's pill opens the search screen.
- [x] Phone tier 2 (`PhoneLayer` + `PhoneBackBar`; `phone-bar.ts` says what the bar reads): a full-screen layer with a back bar naming the level and the path above it (last segment whole; the organisation for a top-level department), then the same tools and list as the column; the search box takes the whole row and is 16 px; suggestions cover the list with "Quay lại danh sách" as before. Back = the previous entry of the stack when the app made it (`history.back()`), else one level up with `replace` (a link into a department climbs level by level, then home); Esc (a hardware keyboard) is the same step; a search typed over a level is cleared first.
- [x] Phone tier 3: `ContactProfile` in `screen` mode (cover 112, floating back with a 44 hit area, three actions 64 high), a layer of its own; `?person=` opens it over the first screen; system back closes it.
- [x] Lazy loading and skeletons are the desktop's (same hook); the first screen asks for the directory only.
- [x] Reduced motion: layers and sheet fade (120 ms) instead of sliding.
- [x] `npm test` 3672 pass, `npm run lint`, `npm run typecheck:diff`, `npm run build` green (same run as phase 07).
- [x] Visual check (Playwright headless Chromium, real dev stack, touch emulation for the small widths) at 820 (light only) and 375 (light and dark; the dark run also with reduced motion): no console errors, no horizontal scroll (`scrollWidth == innerWidth`), input `font-size` computed 16 px at 375; compared with mockup §6 and §7 (first screen, department level and leaf, search, group, full-screen profile, tablet sheet).

### New files
`components/contacts/`: `ContactsChips`, `ContactsCategoryHome`, `ContactProfileSheet` (also `PhoneLayer` is in `ContactsPhone.tsx` with `PhoneBackBar` and `phonePush`), `phone-bar.ts`; tests for each, and `ContactsScreen.layouts.test.tsx` (22 tests at 820 and 375 over the shared `test/contacts-fixtures.tsx`).

### Tests that failed first (red for the right reason, then green)
- `ContactsChips`, `ContactsCategoryHome`, `ContactProfileSheet`, `phone-bar`: the module did not exist; `homeSearch` "is not a function"; `PhoneLayer` focused the first control instead of the one given (`initialFocusRef`).
- `ContactsScreen.layouts.test`: 21 of 22 failed against the desktop-only screen (no chips, no tiles, no sheet, no stack); after the layouts were written three failed on genuine causes, fixed: the phone's first screen asked for departments (it used the default category: it now uses an inert one), a personal-workspace tile text join, and a link to a category the workspace does not have.

## Deviations
- **Pill hint stays "Tìm người"** (the shell's existing contract, tested in `PhoneSearchBar.test`), not "Tìm trong danh bạ" as the mockup draws it.
- **Tiers 2 and 3 are full-screen layers in the body** (like `SidePanel` as a sheet), not routes that replace the shell, so the app's own top bar steps aside without touching `isPhoneDetailPath`; the page behind is inert while one is open.
- **"Mở nhóm" on a phone** is an icon button at the end of the group's bar (the mockup draws none there, and a phone would otherwise have no way into the conversation).
- On a phone the search box has the whole row and the online chip the row below (mockup §7); on a tablet they share a row as in §6.
- The tablet says a failed department tree in the column (with "Thử lại"), not under a chip.

## Open issues
- The pill of a tablet has nothing to focus on the Vai trò and Nhóm lists (they carry no search box, as drawn); it shows the module name there.
- A narrow row (375) ellipsizes the title before the earlier path segments; the last segment is never cut.


## Review fixes (2026-10-18)
- [x] H1: typing in the search box of a level on a phone no longer replaces the screen: the level is keyed by the place in the address (`cat`/`dept`/`role`/`chat`), not by what is shown, and `BrowseView.level` keeps the level's name for the bar while a search is typed over it (`phone-bar.ts` reads it). The box keeps focus and both letters. Test at 375 first red (the box was a new element).
- [x] M1: the search screen lists nobody until something is typed, says what to type, and has one back arrow (the one in the bar), not two.
- [x] M2: see phase 07 (layer counting, focus to the new level's back button).
- [x] L1: the phone's back uses `history.back()` only when the entry before is a Danh bạ level (pushed entries carry `contactsStack` in the history state, kept across `replace`); otherwise it goes up one level and replaces, so it never leaves Danh bạ for the page the person came from. Test: Chat entry, then a department, back stays in Danh bạ one level up.
- [x] L8: the tablet's chips end with Phòng của tôi (pressed while it is the page).
- [x] `PersonRow` memo, deferred filter, stable row handler: see phase 07.
- Note: one test added after its fix, not before: "keeps the name of the level while something is typed over it" in `phone-bar.test.ts` (the screen-level test of H1 was red first).
