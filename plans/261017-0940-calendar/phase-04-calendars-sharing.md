---
phase: 4
title: "Lịch, đăng ký xem, chia sẻ"
status: completed
priority: P1
effort: "1.5d"
dependencies: [3]
---

# Phase 04: Lịch, đăng ký xem, chia sẻ

## Overview
API lịch: tạo lười 3 loại lịch + node NGAC, danh sách Lịch của tôi / Lịch khác, màu và ẩn hiện, chia sẻ.

## Requirements (REST, JWT, dưới `/api/workspaces/:wsId/calendar`)
- `GET /calendars`: lịch cá nhân (tạo lười), lịch workspace (tạo lười; workspace cá nhân thì không có), lịch nhóm của mọi
  nhóm caller là thành viên (đọc `channel_members`, kiểm `read`, chỉ trả lịch đã tồn tại), lịch khác đã đăng ký. Mỗi mục:
  tên hiển thị (lịch cá nhân = tên người), loại, màu, visible, `access` (`freebusy|read|write|manage`), chủ (dữ liệu
  PersonChip). Không trả ID node NGAC.
- `PUT /calendars/:calId/subscription` {color, visible}; `POST /subscriptions` {userNodeId} thêm lịch cá nhân người khác
  (cùng workspace, access tối thiểu freebusy); `DELETE /subscriptions/:calId`.
- `GET /calendars/:calId/shares`, `PUT /calendars/:calId/shares/:granteeNodeId` {level}, `DELETE …`: cần `share` + `manage`
  (chỉ lịch cá nhân). Đổi mức = gỡ association cũ, tạo mới qua `provision.Creator` (rollback khi lỗi). Người được chia sẻ
  tự được đăng ký; phát `calendar.shared` (phase 07 nối consumer).
- `POST /channels/:channelId/calendar` (tạo lười lịch nhóm, OA dưới `ChannelContentOAName`, cần `write` trên Content OA).
- Domain authz: `authorize(ctx, cal, op)` = `policyclient.Check(caller, cal.ngac_oa_id, op)`; `accessLevel` qua `BatchCheck`.
- Membership workspace: PC trong ancestors (như `workspace/internal/domain/authz.go:75`).

## Related Code Files
- Create: `backend/services/calendar/internal/{domain/calendars.go,domain/sharing.go,domain/authz.go,domain/membership.go,
  store/calendars.go,rest/calendars.go}` + tests

## Implementation Steps
1. TDD domain với fake policy: tạo lười tạo đúng node/assign/assoc; deny chia sẻ lịch người khác; deny đăng ký người ngoài
   workspace; gọi lại không tạo trùng.
2. Store + REST + flow test (DB thật, skip nếu không có DB).

## Success Criteria
- [x] Test deny: chia sẻ lịch của người khác → 403; đăng ký người ngoài workspace → 403; lịch nhóm khi không là thành viên → 403.
- [x] `GET /calendars` gọi hai lần không tạo node trùng.
