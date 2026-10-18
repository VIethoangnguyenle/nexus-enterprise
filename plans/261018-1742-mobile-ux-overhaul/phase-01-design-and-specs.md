---
phase: 1
title: "Design and specs"
status: pending
priority: P1
effort: "2d"
dependencies: []
---

# Phase 01: Design and specs

## Overview
Turn the approved mockup into the design source of truth (DESIGN.md) and the capability specs every later phase
implements against. No product code.

## Requirements
- Functional: DESIGN.md describes bar A, Chat chips + Mục, module header grammar, Tôi/module/conversation settings,
  Quản lý nhóm, unified search, gestures (app + Chat), Tài liệu phone, personal vs organization differences.
- Non-functional: mockup `design/mockups/mobile-shell.html` stays the visual reference; B variant removed from it.

## Architecture
DESIGN.md §5 "Điều hướng di động" is rewritten (drawer and Chat-only bar retired); §6 gains "Cài đặt", "Cài đặt cuộc
trò chuyện", "Quản lý nhóm", "Tìm cho mọi tác vụ", "Vuốt", and the Tài liệu phone paragraph is replaced; "Chat trên
điện thoại" updates Trang chủ/Trực tiếp/Mục/Thêm → chips. Specs live in `docs/specs/<capability>/spec.md`.

## Related Code Files
- Modify: `DESIGN.md` (§5, §6 Chat trên điện thoại, Tài liệu, Gợi ý trong ô tìm, Danh bạ notes), `design/mockups/mobile-shell.html` (drop §1 variant B, mark approved), `docs/specs/README.md`
- Create: `docs/specs/mobile-shell/spec.md`, `docs/specs/user-settings/spec.md`, `docs/specs/conversation-settings/spec.md`, `docs/specs/chat-group-management/spec.md`, `docs/specs/chat-mention-all/spec.md`, `docs/specs/unified-search/spec.md`, `docs/specs/mobile-gestures/spec.md`
- Modify specs: `tenant-ngac-init` (Managers UA beside Members; `create_channel` lock = right removed, not a prohibition; rows `:66`, `:160`, `:176`, `:211`), `chat-sections`, `search-suggestions`, `chat-threads`, `drive-tree-navigation`, `drive-permission-engine`, `file-preview`, `notifications`, `workspace-admin-authorization`, `resource-pep-coverage`, `tenant-identity` (no kind conversion), `contacts-documents-settings-screens`, `pwa-install`
- Modify mockup: §9 keyboard row becomes "kéo luồng tin xuống quá N px thì đóng bàn phím" (iOS web cannot move the keyboard with the finger); §8 drops the read-receipt lock row; §4 drops "Cho người khác biết bạn đã xem"

## Implementation Steps
1. Rewrite DESIGN.md §5 phone navigation from mockup §1/§1b/§8 (bar metrics, compact state, quick jump, Thêm, Sửa thanh đáy, defaults per workspace kind, workspace tile).
2. Add §6 sections from mockup §2–§10, keeping token names and numbers from the mockup notes.
3. Write the seven new specs as behaviour + acceptance (Given/When/Then), each naming its deny cases where authorization is involved (`chat-group-management`, `unified-search`, `user-settings` locks).
4. Update the modified specs (scope paragraphs only; mark superseded text).
5. Remove variant B from the mockup, add "duyệt 2026-10-19" to its header; `scripts/check-docs-drift.sh` passes.

## Success Criteria
- [ ] DESIGN.md has no reference to the ☰ drawer as phone navigation, nor to convert-to-organization
- [ ] Seven new specs exist and are indexed in `docs/specs/README.md`
- [ ] `chat-group-management` spec lists every decision (top-level post, reply, invite, mention all on send/reply/edit, appoint/remove manager, transfer owner, disband) with allow + deny cases, says which are PDP decisions and which messaging enforces, and lists every existing `manage` check with who it is for
- [ ] `chat-mention-all` spec defines @Tất cả as a new capability
- [ ] Zane approves the DESIGN.md diff before phase 02 starts

## Risk Assessment
Design text drifts from the mockup → each DESIGN.md paragraph cites its mockup §; review compares side by side.
