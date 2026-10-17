---
phase: 1
title: "Design + UI specs"
status: completed
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
  frames like sections 8/9). Admin nav gains "Danh bạ". Release-A state for a workspace whose directory is not ready
  (rules endpoint 503): ~~reuse the section's error state copy~~ **superseded by plan decision Q4 (2026-10-17): the "Danh bạ" tab is not shown at all; a pasted link lands on Tổng quan.** No new layout.
- `contacts-lark.html` §5 last state: replace "Bạn vẫn xem được Tất cả thành viên" (removed category) with
  "Bạn vẫn tìm được người bằng ô tìm." and its note "Tất cả thành viên và Nhóm của tôi vẫn dùng được" →
  "Vai trò, Nhóm của tôi và ô tìm vẫn dùng được" (Zane decision C5, 2026-10-17).
- Specs (UI behaviour only; NGAC model text lands with phase 02 code per CLAUDE.md §5):
  - `docs/specs/contacts-documents-settings-screens/spec.md`: rewrite "Danh bạ lists the workspace's people" → category
    browsing, breadcrumbs, search across visible scope, profile card with Gọi; **delete** "There SHALL be no button for a
    feature that does not exist (no call)"; scenarios: same-named departments, personal workspace, own profile, partial
    tree failure, call while offline, hidden person card. Keep the shipped requirement "Danh bạ and Tài liệu suggest
    before anything is typed" (`spec.md:23`, from 261017-1516, merged) unchanged.
  - `docs/specs/admin-screens/spec.md`: Requirement "Quản trị › Danh bạ sets who sees whom" with scenarios.

## Architecture
Docs + static HTML only. No code.

## Related Code Files
- Modify: `DESIGN.md` (§5 line 203, §6 new subsection after "Gợi ý trong ô tìm" line 795)
- Modify: `design/mockups/admin.html` (new section after §7, nav item)
- Modify: `design/mockups/contacts-lark.html` (§5 partial failure copy only)
- Modify: `docs/specs/contacts-documents-settings-screens/spec.md`
- Modify: `docs/specs/admin-screens/spec.md`

## Implementation Steps
1. Work on `feat/contacts-lark` (main 1dbbbe7); the search-suggestions spec text is already on main.
2. Edit DESIGN.md §5/§6 from the mockup notes (quote token names, not hex).
3. Build admin.html section reusing its existing tokens/components; open with `/browse` at 1280/800/390.
4. Fix contacts-lark.html §5 copy.
5. Update both specs; run `make check-docs` if it covers specs.
6. Ask Zane to approve (DESIGN.md + admin mockup; C5/C6/C7 already decided). Record approval date in DESIGN.md heading.

## Success Criteria
- Zane approval recorded; DESIGN.md no longer forbids a list panel in Danh bạ; spec has no "no call" rule.
- admin.html renders the new section at three widths without raw IDs.

## Risk Assessment
- Approval delay blocks everything (Med×Med): phases 02–05 (backend) may start in parallel once the admin mockup is
  approved, since UI copy does not affect them. Frontend phases wait.
- Rollback: docs revert.

## Todo
- [x] DESIGN.md §5: list panel allowed in Danh bạ (280, `--color-base`, rows 40/15px, NavRow selection); categories-only
      left column, tablet chips, phone category stack
- [x] DESIGN.md §6: new subsection "Danh bạ" (categories, lazy load + row skeletons, breadcrumbs, department/person rows, sort,
      search, profile card with avatar 64 and Nhắn tin/Gọi/Gửi email, own profile "Sửa hồ sơ", detail rows, visibility copy,
      hidden-person card, states, Quản trị › Danh bạ); avatar line reads "thẻ hồ sơ"; no "no call" rule anywhere (Gọi always enabled)
- [x] `design/mockups/admin.html` §7b "Danh bạ: phạm vi hiển thị": desktop (interactive pickers, confirm, toasts, load/error/
      save-failure demos), tablet 820 with picker, two phone frames (bottom sheet); state cards incl. not-ready (Q4) and no access;
      "Danh bạ" tab added to every Quản trị tab row; toc link
- [x] `design/mockups/contacts-lark.html` §5 partial-failure copy (C5)
- [x] `docs/specs/contacts-documents-settings-screens/spec.md`: Danh bạ requirements rewritten, "no call" removed, scenarios
      (same-named departments, personal workspace, own profile, partial tree failure, call while offline, hidden person card),
      Status note (specified ahead of code); search-suggestions requirement untouched
- [x] `docs/specs/admin-screens/spec.md`: four tabs, "Quản trị › Danh bạ sets who sees whom" + scenarios, Status note
- [x] Verified: headless Chromium (Playwright) — admin.html and contacts-lark.html, 1280/800/375 (+320 for admin), light and
      dark: no JS errors, no horizontal page scroll; picker → confirm → row + toast, Esc closes topmost layer, reduced motion,
      no UUID text. `make check-docs` green.
- [x] Design gate

## Design gate (2026-10-17)
Self-approved by Claude per Zane's delegation ("Quyết định sau đồng bộ" in plan.md), checked against the approved mockup
`design/mockups/contacts-lark.html` (§1–§7, §1c settings block). Zane may review DESIGN.md "Danh bạ" and admin.html §7b later;
nothing here changes a decision Zane made. Phases 02–05 may start; frontend phases follow this design.

Fixes made while verifying (review chrome only, admin.html): `.page`/`.screen` columns `minmax(0, 1fr)`, `.frames > div`
and `.tablet` max-width, `.sheet` min column — the page scrolled sideways at 800 and 375 before this phase (860px wide).
