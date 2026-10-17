---
phase: 1
title: "Design + UI specs"
status: pending
priority: P1
effort: "4h"
dependencies: []
---

# Phase 1: Design + UI specs

## Overview
Design-first gate (CLAUDE.md §3, DESIGN.md is source of truth). Write the approved mockup into DESIGN.md, add the
Quản trị › Danh bạ screen to `design/mockups/admin.html`, fix the one stale copy line in `contacts-lark.html`, and update
the screen specs. Code phases start only after Zane approves this phase.

## Requirements
- DESIGN.md §5 Layout: "List panel chỉ ở Tin nhắn và Tài liệu" → "Tin nhắn, Tài liệu và Danh bạ" (280, `--color-base`,
  NavRow 40/15px). Add: Danh bạ left column = categories only; tablet = chips; phone = category home stack.
- DESIGN.md §6 new subsection "Danh bạ (mockup `design/mockups/contacts-lark.html`, duyệt 2026-10-17)": categories (org:
  Phòng ban · Vai trò · Nhóm của tôi + lối tắt Phòng của tôi; personal: Thành viên · Nhóm của tôi), lazy load with
  row-shaped skeletons (no spinner), breadcrumb rules (root building icon, collapse middle with "…", last segment never
  cut), department row (32 accent-wash folder, "N người" incl. subtree, sub-line "N phòng, nhóm bên dưới"), person row
  (avatar 32, title, status in words), sort by given name (last word) then full name, profile card (cover gradient
  person-N-wash→accent-wash, avatar 64 — already allowed by §6, name text-lg, Nhắn tin primary / Gọi / Gửi email,
  own profile → "Sửa hồ sơ"), detail rows (Email + Sao chép, Phòng ban full path clickable, Nơi làm việc, Vai trò,
  Cùng phòng stack 24), hidden-person card, visibility copy ("Chỉ phòng của bạn", "trong phạm vi bạn được xem",
  "Ẩn khỏi danh bạ"), counts are muted text not accent pills, Gọi always enabled.
- `design/mockups/admin.html`: new section "Danh bạ · Phạm vi hiển thị" matching mockup §1c settings block (two lists:
  "Phòng ẩn khỏi danh bạ", "Chỉ thấy phòng của mình"; "Ẩn một phòng" opens DepartmentPicker; "Thêm vai trò hoặc phòng"
  opens a picker with Vai trò list + DepartmentPicker; Bỏ ẩn / Bỏ with confirm; empty, loading, error states; tablet + phone
  frames like sections 8/9). Admin nav gains "Danh bạ".
- `contacts-lark.html` §5 last state: replace "Bạn vẫn xem được Tất cả thành viên" (removed category) with
  "Bạn vẫn tìm được người bằng ô tìm." and its note "Tất cả thành viên và Nhóm của tôi vẫn dùng được" →
  "Vai trò, Nhóm của tôi và ô tìm vẫn dùng được" (pending Zane's answer, plan Q5).
- Specs (UI behaviour only; NGAC model text lands with phase 02 code per CLAUDE.md §5):
  - `docs/specs/contacts-documents-settings-screens/spec.md`: rewrite "Danh bạ lists the workspace's people" → category
    browsing, breadcrumbs, search across visible scope, profile card with Gọi; **delete** "There SHALL be no button for a
    feature that does not exist (no call)"; scenarios: same-named departments, personal workspace, own profile, partial
    tree failure, call while offline, hidden person card. Keep the search-suggestions requirement as phase 06 of
    261017-1516 left it.
  - `docs/specs/admin-screens/spec.md`: Requirement "Quản trị › Danh bạ sets who sees whom" with scenarios.

## Architecture
Docs + static HTML only. No code.

## Related Code Files
- Modify: `DESIGN.md` (§5 ~line 203, §6 new subsection after "Gợi ý trong ô tìm" ~line 705)
- Modify: `design/mockups/admin.html` (new section after §7, nav item)
- Modify: `design/mockups/contacts-lark.html` (§5 partial failure copy only)
- Modify: `docs/specs/contacts-documents-settings-screens/spec.md`
- Modify: `docs/specs/admin-screens/spec.md`

## Implementation Steps
1. Confirm 261017-1516 phase 06 merged (spec file stable); rebase.
2. Edit DESIGN.md §5/§6 from the mockup notes (quote token names, not hex).
3. Build admin.html section reusing its existing tokens/components; open with `/browse` at 1280/800/390.
4. Fix contacts-lark.html §5 copy.
5. Update both specs; run `make check-docs` if it covers specs.
6. Ask Zane to approve (DESIGN.md + admin mockup + Q5 copy + Q6/Q7 answers). Record approval date in DESIGN.md heading.

## Success Criteria
- Zane approval recorded; DESIGN.md no longer forbids a list panel in Danh bạ; spec has no "no call" rule.
- admin.html renders the new section at three widths without raw IDs.

## Risk Assessment
- Spec drift vs search-suggestions phase 06 (Med×Low): blocked on it; rebase before editing.
- Approval delay blocks everything (Med×Med): phases 02–05 (backend) may start in parallel once the admin mockup is
  approved, since UI copy does not affect them. Frontend phases wait.
- Rollback: docs revert.
