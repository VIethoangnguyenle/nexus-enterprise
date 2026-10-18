---
phase: 6
title: "Conversation settings"
status: pending
priority: P1
effort: "1.5d"
dependencies: [2]
---

# Phase 06: Conversation settings (group and DM)

## Overview
Tapping the conversation name opens Cài đặt nhóm / Cài đặt trò chuyện (mockup §5) built on data that already
exists; the header keeps only Gọi and Thread.

## Requirements
- Functional: group page = avatar · name · description (rename/description for managers) · quick actions Tìm,
  Thông báo, Gắn sao, Thêm người · Thành viên (list with search, roles once phase 07 lands) · Tệp, ảnh và liên kết ·
  Tin đã ghim · Thông báo level · Mục (section picker, existing sections API) · Quản lý nhóm row (phase 07)
  · Rời nhóm (confirm; the "last owner must transfer first" rule arrives with phase 07). DM page = person header (opens contact card) · Hồ sơ, Tìm, Thông báo, Gắn sao
  · Tệp, Tin đã ghim · Thông báo · Mục · Tạo nhóm với <tên>. Desktop: same content in the right info panel.
- Non-functional: no IDs; every action hidden when the PDP would refuse it (`useObjectPermissions`), server decides.

## Architecture
`SpaceInfoPanel` becomes `ConversationSettings` (one component, phone page / desktop panel). Members reuse
`MembersPanel`; files reuse the Tệp tab grid; pins reuse `usePins`; sections reuse chat-sections hooks;
the quick action "Gắn sao" is the existing star (Zane 2026-10-19: no pin-to-top feature), shown once.

## Related Code Files
- Modify: `frontend/src/components/spaces/SpaceInfoPanel.tsx` → `ConversationSettings.tsx`, `SpaceView.tsx` (header: name button opens settings; remove Tìm and ⓘ on phone), `MembersPanel.tsx`, `ConversationTitleMenu.tsx`
- Create: `SectionPickerSheet.tsx`, `LeaveGroupDialog` reuse (mockup `leave-group.html`), tests

## Implementation Steps
1. Tests: header shows 2 actions on phone; name tap opens settings; DM vs group rows; Rời nhóm confirm; section move.
2. Refactor panel → ConversationSettings; add rows; wire existing hooks.
3. Desktop panel parity snapshot.

## Success Criteria
- [ ] Group and DM settings match mockup §5 rows (except Quản lý nhóm content → phase 07)
- [ ] Leave group works with confirmation (last-owner blocking is phase 07's criterion)
- [ ] A refused action (stale client permission) shows "Bạn không còn quyền làm việc này" and refreshes permissions
- [ ] Existing SpaceInfoPanel tests migrated and green

## Risk Assessment
Renaming a widely used component → keep the old export name as a re-export until all callers move.

## Rollback
Frontend-only; revert the release.
