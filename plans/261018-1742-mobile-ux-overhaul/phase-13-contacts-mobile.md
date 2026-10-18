---
phase: 13
title: "Danh bạ on the phone"
status: pending
priority: P2
effort: "2d"
dependencies: [8, 9]
---

# Phase 13: Danh bạ on the phone

## Overview
Phone-first Danh bạ (mockup §12, DESIGN.md "Danh bạ" → Điện thoại) on top of the approved Lark-style structure:
Thường liên lạc, category grid, people search through the unified search, A–Z index, swipe actions, and a profile card
with Lịch and "Đã cùng làm việc". No change to who sees whom (`contacts-visibility`).

## Requirements
- Functional: home = Thường liên lạc (up to 8 recent direct-message partners still in the viewer's scope; hidden when
  none) · category grid (Phòng ban · Phòng của tôi · Vai trò · Nhóm của tôi; personal: Thành viên · Nhóm của tôi) ·
  Đang trực tuyến (5 + "Xem tất cả"); department / role / group pages with ← up, path, search icon opening the unified
  search scoped to Người and pre-filtered to that place (no in-page search field), A–Z index from 30 people, row swipe
  left = Nhắn tin · Gọi; profile card with 4 actions Nhắn tin · Gọi · Lịch · Email, details, Cùng phòng, Đã cùng làm việc
  (N nhóm chung, tệp đã chia sẻ với bạn), ⋯ (Sao chép email · Thêm vào nhóm… · Mời vào cuộc họp); own card "Sửa hồ sơ" →
  Tôi › Tài khoản và hồ sơ.
- **Departments only in organization workspaces:** personal workspaces show no Phòng ban, Phòng của tôi, Vai trò,
  department filter chip, department rows or Cùng phòng; a person's subtitle is the title alone.
- Backend: "groups in common" (messaging: groups both the viewer and the person are in — only groups the viewer is a
  member of) and "files this person shared with me" (drive: shared-with-me filtered by owner, read re-checked); both
  on the owning services' existing prefixes, membership of the workspace checked.

## Architecture
Contacts screen gets a phone composition sharing the existing category/department hooks; Thường liên lạc reuses
`useRecentDirects`; the A–Z index is a small component over the sorted list (Vietnamese collation already used for
sorting); Lịch action routes to the calendar with the person's calendar added (existing calendar "Tìm người" path).

## Related Code Files
- Modify: `frontend/src/components/contacts/ContactsScreen.tsx` and its category/department/profile components, `frontend/src/hooks/useRecentDirects.ts` (scope filter), messaging and drive REST/domain for the two read endpoints, `docs/specs/contacts-visibility` (Planned change → current text)
- Create: `AlphabetIndex.tsx`, `CommonWork.tsx`, tests

## Implementation Steps
1. Backend tests first, deny first: groups in common never lists a group the viewer is not in; files shared by a person never lists a file the viewer cannot read or from another workspace; non-member → 403.
2. Endpoints; frontend phone composition; personal vs organization e2e at 390px.

## Success Criteria
- [ ] Mockup §12 reproduced at 390px for both workspace kinds; no department UI in a personal workspace
- [ ] Deny tests for both endpoints; strict test runs green for messaging and drive
- [ ] Hidden people (contacts-visibility) never appear in Thường liên lạc, Đang trực tuyến or the A–Z list

## Risk Assessment
Thường liên lạc leaking a person hidden by the admin's scope → filtered by the same scope check as the directory; test.

## Rollback
Frontend plus two read-only endpoints; revert the release.
