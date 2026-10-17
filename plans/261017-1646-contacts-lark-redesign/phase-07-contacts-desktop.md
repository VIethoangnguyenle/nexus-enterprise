---
phase: 7
title: "Danh bạ desktop (≥1024)"
status: "completed (not committed)"
priority: P2
effort: "10h"
dependencies: [6]
---

# Phase 7: Danh bạ desktop (≥ 1024)

## Overview
Replace the flat table/cards screen with mockup §1, §1b, §1c (member views), §2, §2b, §3, §4, §5: category column,
middle column browsing with breadcrumbs, profile card. Spec: `contacts-documents-settings-screens`.

## Requirements
- Left list panel (280, NavRow): org → Phòng ban (sub-line "Chỉ phòng của bạn" when `visibility==='limited'`) · Vai trò ·
  Nhóm của tôi (hidden when the caller has no groups) · lối tắt "Phòng của tôi" (opens caller's `department_id`; hidden
  when none). Personal → Thành viên (default, flat list) · Nhóm của tôi. No "Tất cả thành viên", no Bên ngoài / Danh bạ
  chia sẻ. Counts muted text.
- Lazy: a category's query is enabled on first open; skeleton rows shaped like real rows (folder square / round avatar).
- Middle column: one level per screen; sub-departments first (folder 32 accent-wash, name, "N người" subtree count,
  sub-line "N phòng, nhóm bên dưới" only if it has children, chevron), then people of that level; org root lists
  top-level departments then people without department. Admin sees "Ẩn khỏi danh bạ" label on hidden departments.
- Breadcrumb: building icon = root, every segment a button to that level, last `aria-current`, collapse middle to "…".
- Vai trò: list (Quản trị viên first with shield icon + "quản lý workspace", then custom), open → holders (title · path);
  header name + count, no permissions shown. Hidden in personal workspaces.
- Nhóm của tôi: private groups the caller is in (`channel_type === 'private'`; `#general` excluded, plan Q2) (square 32
  avatar via existing group avatar, name, `member_count`), open → members loaded then
  ("Mở nhóm" secondary navigates to the channel), rows title · path; members that are hidden render via resolver name and
  open `HiddenPersonCard` (via `PersonLink`, built in phase 06) instead of the profile.
- Search (SearchField, placeholder "Tên, chức danh hoặc email", aria-label "…trong cả <ws>" or
  "…trong phạm vi bạn được xem" when limited): flat results across loaded contacts, header "Kết quả trong cả <ws>" + count,
  sub-line title · full path (last segment bold, never cut), match highlight `--color-warning-wash`, ✕/Esc returns to the
  previous level. Keep the existing `SearchSuggestions` panel (`components/patterns/SearchSuggestions.tsx`) +
  `lib/suggestions/contacts.ts` exactly as 261017-1516 shipped them (main 43a1509).
- Chip "Đang trực tuyến" filters the current view with count.
- Profile card (detail panel; overlay at 1024–1279): cover gradient, avatar 64, name text-lg, title, status in words;
  actions Nhắn tin (primary, `useCreateDM({userId, ngacNodeId})` (`hooks/useMessaging.ts:38`; the DM gate needs
  `target_user_id` and a shared workspace, always true here) then navigate), Gọi (`useCreateDM` → `useStartCall({channelId,
  kind})` (`hooks/useCalls.ts:34`) → call screen; **always enabled**, even offline), Gửi email (`mailto:`); own profile →
  "Sửa hồ sơ" (Cài đặt). Rows: Email + Sao chép, Phòng ban full path (each segment opens that department), Nơi làm việc,
  Vai trò (each opens the role), Cùng phòng (stack 24 + sentence, opens the department). Empty rows omitted; personal
  workspace omits Phòng ban, Vai trò, Cùng phòng. Esc / ✕ closes, focus returns to the row.
- States (§5): loading skeleton; empty department ("<tên> chưa có ai" + back to parent); no results ("Không có ai khớp
  “…” …" + Xoá tìm kiếm); online filter empty ("Hiện mọi người"); contacts error ("Không tải được danh bạ" + Thử lại);
  departments error under the Phòng ban entry with Thử lại while Vai trò, Nhóm, search keep working (copy per phase 01).
- No raw IDs anywhere (ids only in keys and URL search params for the selected person/department, matching the
  existing `/contacts` route style).

## Architecture
- `routes/_workspace/contacts.tsx` search params: `cat`, `dept`, `role`, `group`, `person`, `q` (validated; unknown →
  defaults) so back/forward and deep links work and phase 08 reuses them.
- Components (new, `components/contacts/`): `ContactsNav.tsx` (categories), `ContactsBrowser.tsx` (middle column per
  category), `ContactsBreadcrumb.tsx`, `DepartmentRow.tsx`, `PersonRow.tsx`, `ContactProfile.tsx` (replaces
  `ContactPanel.tsx`), `ContactsStates.tsx`. `ContactsScreen.tsx` becomes the layout shell.
- Delete: `ContactsTable.tsx`, `ContactCards.tsx`, `DepartmentFilter.tsx` (table/card toggle and department chip are gone
  per mockup); remove their preference key usage from `usePreferences` if it becomes dead.

## Related Code Files
- Modify: `frontend/src/components/contacts/ContactsScreen.tsx`, `ContactsScreen.test.tsx`,
  `frontend/src/routes/_workspace/contacts.tsx`
- Create: files listed above + `*.test.tsx` per component
- Delete: `ContactsTable.tsx`, `ContactCards.tsx`, `DepartmentFilter.tsx`, `ContactPanel.tsx`

## Implementation Steps (TDD)
1. Vitest (RTL) first, per mockup note: category home without "Tất cả thành viên"; departments not requested until Phòng
   ban opened (assert fetch calls); drill A → A1 → breadcrumb back; same-named departments; Vai trò excludes Thành viên,
   owners labelled Quản trị viên; group members fetched only on open; search finds "doi soat"; Gọi enabled for offline
   person and starts DM + call; own profile shows Sửa hồ sơ; personal workspace categories; limited copy; admin hidden
   label; every state; no element text matches a UUID regex.
2. Build components to green; `npm test`, `npm run lint`, `npm run build`.
3. `make dev`, sign in as the seeded accounts (`scripts/seed-contacts-demo.sh`, phase 04), compare with mockup at 1280 and
   1024 via `/browse`; `make dev-stop`.

## Success Criteria
- Screenshots at 1280/1024 match mockup §1, §1b, §2, §2b, §3, §4, §5.
- Opening Danh bạ triggers zero department/channel requests until a category is opened (except the default category).

## Risk Assessment
- Large rewrite of a 21 KB test file (Med×Low): rewrite tests category by category; keep the error/no-ID tests.
- Call from Danh bạ requires a DM channel first (Low×Med): reuse the same create-then-start sequence the chat header uses.
- Rollback: revert frontend commit.

## Todo / Status (2026-10-18)
- [x] Route `/contacts` validates `cat`, `dept`, `role`, `chat`, `person`, `q` (`lib/contacts-search.ts`: `validateContactsSearch` drops what it does not know, keeps only the place of the category, a place alone names its category; `categorySearch`, `deptSearch`, `roleSearch`, `groupSearch`, `openPersonSearch`, `closePersonSearch`, `homeSearch`). The group key is `chat`, not `group`: see Deviations.
- [x] Category column `ContactsNav` (280): Danh bạ title, categories (`CategoryItem`, 40 icon square on `--color-person-N-wash`, name, one muted line), Phòng của tôi shortcut, the tree-failure alert under Phòng ban. Org: Phòng ban · Vai trò · Nhóm của tôi; personal: Thành viên · Nhóm của tôi; Bên ngoài / Danh bạ chia sẻ / Tất cả thành viên are not rendered. A category known to be empty is left out (`categoriesOf`: roles with nobody, groups once the chat list is known to hold none).
- [x] Lazy: `useContactsBrowse` asks for departments only for Phòng ban (and wherever a department path is shown: a search, a role's holders, a group's members, an open profile), the chat list only for Nhóm của tôi, a group's members only for the opened group. Nothing but `/contacts` is requested on `?cat=roles`. Skeleton rows are shaped like real rows (`RowSkeletons`, `aria-busy`, no spinner). "Not loaded" is loading, never "nobody here" (the first paint before the workspace is known was flashing "Workspace này chưa có ai khác").
- [x] Middle column `ContactsBrowser`: one level per screen; sub-departments first (`DepartmentRow`: folder 32 accent-wash, name, "N người" with the subtree, "N phòng, nhóm bên dưới", chevron, "Ẩn khỏi danh bạ" for an administrator), then people (`PersonRow`, sorted by given name); root lists "Khối" then "Chưa thuộc phòng ban nào"; breadcrumbs (`ContactsBreadcrumb`: building root, every segment a button, last `aria-current`, middle folds to "…" and unfolds); Vai trò list (Quản trị viên first with the shield, "quản lý workspace", no Thành viên, holders with title · path); Nhóm của tôi (private groups only, members loaded on open, "Mở nhóm", a member the directory hides shown by name through the resolver and opening `HiddenPersonCard`).
- [x] Search across the caller's visible scope (placeholder "Tên, chức danh hoặc email", label "…trong cả <ws>" / "…trong phạm vi bạn được xem"), header "Kết quả trong cả <ws>" + "N người khớp", title · full path with the last segment whole, match highlight, ✕/Esc back to the previous level; typing is local and the address follows 300 ms later (`replace`). The `SearchSuggestions` panel and `lib/suggestions/contacts.ts` are untouched. Chip "Đang trực tuyến · N" per level.
- [x] Profile card `ContactProfile` (cover in the person's hue, avatar 64 with presence dot, Nhắn tin / Gọi (always enabled; `useCreateDM` with `target_user_id`, then `useStartCall` voice, then the call screen) / Gửi email, "Sửa hồ sơ" on one's own, Email + Sao chép, whole department path (each segment opens it), Vai trò (each opens its holders), Nơi làm việc, Cùng phòng); empty rows omitted; personal workspace has no Phòng ban / Vai trò / Cùng phòng; Esc or ✕ closes and focus goes back to the row; `?person=` opens it.
- [x] States: loading, empty department ("<tên> chưa có ai" + "Về <cha>"), no results (+ "Xoá tìm kiếm"), nobody online ("Hiện mọi người"), directory error ("Không tải được danh bạ" + "Thử lại"), partial tree failure ("Không tải được cây phòng ban. Bạn vẫn tìm được người bằng ô tìm." + "Thử lại" under Phòng ban; search and Vai trò keep working), empty workspace.
- [x] Deleted `ContactsTable`, `ContactCards`, `DepartmentFilter`, `ContactPanel`; removed the dead `contactsView` preference (`lib/preferences.ts`, test) and the name-based `departmentsOf` / department filter from `contacts-model`.
- [x] `design/mockups/contacts-lark.html`: the leftover "Tất cả thành viên" removed from the intro, §7 note, §8 sample and the two JS titles.
- [x] Spec `contacts-documents-settings-screens`: Status line now says the screens are built, with the address keys.
- [x] `npm test` 3672 pass (295 files), `npm run lint` clean, `npm run typecheck:diff` clean, `npm run build` green.
- [x] Visual check (Playwright headless Chromium against the real dev stack seeded by `scripts/seed-contacts-demo.sh`, owner, a limited role, a plain member) at 1440 and 1024, light and dark: no console errors, no horizontal scroll, compared with mockup §1/§1b/§2b/§3 (owner root, department + profile, role holders, group with a hidden member and its card, limited view). The stack was stopped and the `.env.dev` link removed afterwards.

### New files
`lib/contacts-search.ts`, `hooks/useContactsBrowse.ts`, `test/contacts-fixtures.tsx`, `components/contacts/`: `CategoryItem`, `ContactProfile`, `ContactsBreadcrumb`, `ContactsBrowser`, `ContactsNav`, `ContactsStates`, `DepartmentRow`, `ListRows` (RoleRow, GroupRow, HiddenMemberRow), `PathTrail`, `PersonRow`, each with its test (`ListRows` and `ContactsBrowser` through the screen tests). Edited outside the list: `hooks/useContactCategories.ts` (`useMyGroups().known`), `lib/preferences.ts`.

### Tests that failed first (red for the right reason, then green)
- `contacts-search.test`, `ContactsBreadcrumb`, `PersonRow`, `DepartmentRow`, `ContactsStates`, `ContactProfile`, `ContactsNav`: the module did not exist (Vite "Failed to resolve import").
- `contacts-model.test`: `pathSegments`, `rootLabel`, `categoriesOf` "is not a function".
- `useContactCategories.test`: `known` was `undefined`, expected `false`.
- `ContactsScreen.test` (rewritten, 51 tests): 48 of 48 failed against the table screen (no "Danh mục" navigation, `departmentsOf` gone from the model); 46 passed after the first implementation. The four that did not exposed real defects, fixed with the test kept: the workspace-switch effect also ran on mount and cleared `?q=` (test "keeps the search in the address"); the first paint, before the workspace was known, said "Workspace này chưa có ai khác" (regression test "shows rows to come…", verified red by reverting the status logic); the directory error also lit the tree alert (two "Thử lại").
- `preferences.test`: the old `contactsView` was still parsed and defaulted.
- `ContactProfile.test` (presence dot): no dot on the 64 avatar.
- Rename `group` -> `chat`: tests changed first (`validateContactsSearch` dropped `chat`).

## Deviations
- **Search key `chat`, not `group`.** `navigate({ search })` types its reducer's `prev` as the union of every route's search; `group: string` clashed with Tài liệu's `group: 'drafts' | 'shared'` and `typecheck:diff` failed in Drive/Documents files. `dept` and `role` are shared with the admin route and are both strings.
- **Roles and Nhóm lists have no search box or chip** (as mockup §2b/§3 draw them); a tablet's top-bar search has nothing to point at there and shows the module name, as the shell does for a screen without a search.
- **"Gọi" starts a voice call** in the direct message (`kind: 'audio'`, `answer=audio`); the phase left the kind open.
- **Tree-failure copy on a desktop** is said under Phòng ban only; the column says "Chưa duyệt được theo phòng ban. Bạn vẫn tìm được người bằng ô tìm." without a second button. Tablet and phone, which have no list panel, say it in the column with "Thử lại".
- The people's second line gives way title-first on narrow rows (the title ellipsizes before the earlier path segments; the last segment is never cut), a rule the mockup does not specify.
- A group is opened from its row; the nav hides Nhóm của tôi once the chat list is known to hold no private group (the chat list is not requested just to decide that).

## Open issues
- The demo group "Đối soát liên khối" (Vinh, Bảo, Lan, Yến) was added to the dev organisation through the API for the visual check; it stays in the dev database.
- Nothing is committed (instruction).


## Review fixes (2026-10-18)
- [x] M1: a search with nothing typed lists nobody (it listed the whole workspace, a "Tất cả thành viên" by another name); the column says "Gõ tên, chức danh hoặc email để tìm người." (also while the directory loads).
- [x] M2: `useModalFocus` counts open layers; the page is freed only when the last one closes (a profile over a level of the phone's stack freed it too early). After going down a level on a phone, focus is on the new level's back button.
- [x] M3: the refetch functions of the data hooks are stable (`useCallback`), so the view is not recomputed every render; rows are keyed by what they are (`Children.toArray` keys), `PersonRow` is memoised with one stable `onSelect`, and the filter follows the box through `useDeferredValue`. A person's row is the same element before and after typing.
- [x] L2: the typed search is written into the address with the person that is opened. L3: closing a profile that has no row to return to focuses the name of the list. L4: a side panel that opens takes focus to the person's name. L5: the breadcrumb folds again when its path changes. L6 (shared code of the data layer): a hidden group member is "Thành viên" until the resolver answers, never the login name (`useGroupMembers`). L7: `PersonLink` keeps `ws`. L9: the mailto address is URL-encoded (`@` kept).
- Tests that failed first: `useModalFocus.test` (closing the upper layer freed the page), `ContactsScreen.test` (row identity while typing; `q` missing from the address with `person`; no focus on the name; no focus on the list heading), `ContactProfile.test` (mailto, focus), `ContactsBreadcrumb.test` (stayed unfolded), `PersonLink.test` (no `ws`), `useContactCategories.test` (username shown), `ContactsScreen.layouts.test` (search screen listed everyone; double back arrow; focus after a level change).
