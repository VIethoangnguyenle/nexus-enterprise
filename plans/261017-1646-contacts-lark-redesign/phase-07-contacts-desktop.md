---
phase: 7
title: "Danh bạ desktop (≥1024)"
status: pending
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
- Nhóm của tôi: groups (square 32 avatar via existing group avatar, name, `member_count`), open → members loaded then
  ("Mở nhóm" secondary navigates to the channel), rows title · path; members that are hidden render via resolver name and
  open `HiddenPersonCard` (via `PersonLink`, built in phase 06) instead of the profile.
- Search (SearchField, placeholder "Tên, chức danh hoặc email", aria-label "…trong cả <ws>" or
  "…trong phạm vi bạn được xem" when limited): flat results across loaded contacts, header "Kết quả trong cả <ws>" + count,
  sub-line title · full path (last segment bold, never cut), match highlight `--color-warning-wash`, ✕/Esc returns to the
  previous level. Keep the existing `SearchSuggestions` panel + `recentlyMessaged` exactly as 261017-1516 built it.
- Chip "Đang trực tuyến" filters the current view with count.
- Profile card (detail panel; overlay at 1024–1279): cover gradient, avatar 64, name text-lg, title, status in words;
  actions Nhắn tin (primary, `useCreateDM` then navigate), Gọi (`useCreateDM` → `useStartCall`
  (`hooks/useCalls.ts:34`) → call screen; **always enabled**, even offline), Gửi email (`mailto:`); own profile →
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
3. `make dev`, compare with mockup at 1280 and 1024 via `/browse`; `make dev-stop`.

## Success Criteria
- Screenshots at 1280/1024 match mockup §1, §1b, §2, §2b, §3, §4, §5.
- Opening Danh bạ triggers zero department/channel requests until a category is opened (except the default category).

## Risk Assessment
- Large rewrite of a 21 KB test file (Med×Low): rewrite tests category by category; keep the error/no-ID tests.
- Call from Danh bạ requires a DM channel first (Low×Med): reuse the same create-then-start sequence the chat header uses.
- Rollback: revert frontend commit.
