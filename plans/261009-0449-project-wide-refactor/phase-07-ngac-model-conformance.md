---
phase: 7
title: "NGAC model conformance"
status: done-with-concerns
priority: P1
effort: 2-3d
dependencies: [2b, 3b]
---

# Phase 07 — NGAC model conformance

## Overview
Đưa code về đúng các quy tắc định danh NGAC và nguyên tắc "graph không chứa object".

## Key insights
- **Chuỗi op inline:** `approval/internal/domain/execution.go:351,412` (`"approve"`),
  `approval/internal/domain/queries.go:63` (`"read"`), `drive/internal/grpc/server.go:181` (`"read"`).
- **Tên node từ input thô:** `workspace/internal/domain/service.go:402` (role UA), `:454` (folder
  OA); `auth/internal/domain/service.go:554` (U node theo username).
- **Helper dùng sai / keyed theo tên:** `ngac.ChannelDriveName(req.ChannelName)` tại
  `drive/internal/grpc/sharing.go:201` (phải là channel ID); `ngac.ChannelsOAName(ws.Name)` tại
  `messaging/internal/domain/service.go:139`; `DeptUAName(name)`, `FolderNodeName(name)`,
  `ShareOAName(itemName, …)` → nguy cơ trùng giữa tenant.
- **Drive root** tìm bằng substring "Documents"/"Docs" rồi fallback OA đầu tiên
  (`drive/internal/grpc/server.go:643`) — phụ thuộc thứ tự.
- **Asset tạo O node mỗi asset** (`asset/internal/grpc/asset_server.go:76-77`) — trái quyết định
  kiến trúc quan trọng nhất; mọi check asset rơi vào CTE fallback.
- Provisioning nhiều bước (CreateWorkspace, CreateDepartment, CreateChannel, initTenantNGAC,
  ensureNGACHierarchy) không có compensation khi lỗi giữa chừng.

## Requirements
- Mọi op dùng hằng `ngac.Op*`; mọi tên node qua helper keyed theo **ID**.
- Drive root resolve bằng FK/ID lưu sẵn, không theo tên.
- Asset check trên type OA (chờ Unresolved Q3).
- Provisioning: idempotent + compensation (hoặc một RPC batch phía policy).
- Migration dữ liệu cho node đã đặt tên theo kiểu cũ.

## Related files
- modify `backend/ngac/ngac_ops.go` (đổi chữ ký helper → compile error ở mọi call site)
- modify các file liệt kê ở Key insights
- create `data/migrations/0NN_ngac_id_keyed_names.sql` → chạy `make db-migrate` và kiểm chứng
- create `docs/specs/asset-authorization/spec.md`; modify `docs/specs/tenant-ngac-init/spec.md`
- add test vectors trong `backend/services/policy/internal/ngac/`

## Implementation steps
1. Thêm lint/grep check trong CI: cấm chuỗi op literal ngoài `backend/ngac`.
2. Đổi chữ ký helper sang ID; sửa đến khi `make build-check` xanh.
3. Viết migration đổi tên node hiện có; test trên DB thật.
4. Asset: test deny/allow trên type OA trước, rồi gỡ O node.

## Success criteria
- [x] `grep` không còn op literal/`fmt.Sprintf` tên node ngoài `backend/ngac` — `scripts/check-ngac-identifiers.sh`
      (CI docs job, `make check-ngac`); `services/approval` tạm bị bỏ qua, còn 4 literal (xem report).
- [x] Hai tenant tạo department cùng tên không đụng nhau (test: `workspace/internal/domain/provisioning_test.go`).
- [x] Không còn node loại O trong graph (`select count(*) from ngac_nodes where node_type='O'` = 0 sau migration 024).

Kết quả chi tiết: `reports/phase-07-report.md`.

## Risks
- Migration đổi tên node ảnh hưởng mọi tenant → cần chạy thử trên bản sao DB, có rollback.
