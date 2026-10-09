---
phase: 2
title: "Close authorization gaps"
status: done
priority: P0
effort: 2-3d
dependencies: [1]
---

# Phase 02 — Close authorization gaps

## Overview
Các endpoint dưới đây đổi state hoặc đọc dữ liệu mà **không có PEP check**. Đây là lỗ hổng,
không phải nợ kỹ thuật. Dùng skill `ngac-policy-change` + `security-and-hardening`.

## Key insights (đã kiểm chứng trong code)
- **workspace — 0 policy call trong toàn service.**
  - `POST /api/workspaces/:id/permissions` → `domain/service.go:506 CreatePermission` gọi thẳng
    `policyWrite.CreateAssociation` với UA/OA/ops do client gửi → tự cấp quyền.
  - Invite/remove member, create role/folder, mọi route department admin: không check.
- **messaging:** `CreateChannel` (`domain/service.go:61`) không check `ngac.OpCreateChannel` — op
  này không được check ở đâu trong codebase. `UpdateChannel` (`:242`) không check.
- **drive:** `RevokeShare` (`internal/grpc/sharing.go:115`), `UpdateQuota` (`:285`) không check.
- **asset:** `ListAssets` (`internal/grpc/asset_server.go:136`), `ListRequests`/`GetRequest`
  (`request_server.go:279,299`), toàn bộ `asset_type_server.go` không check.
- **gRPC:** không server nào xác thực caller; `user_ngac_node_id` lấy nguyên từ request.
- **WebSocket subscribe không check membership:** `messaging/internal/grpc/hub.go:470-475` —
  client đã auth gửi `SubscribeRequest{channel_id}` bất kỳ là nhận được tin nhắn của channel đó.
- **Approval event phát cho mọi user đang kết nối, mọi tenant:** `hub.go:565-591`
  (`BroadcastApprovalEvent` lặp `h.users`) → lộ request ID, template name, actor sang tenant khác.
- **frontend:** `AppSidebar.tsx:238`, `routes/assets.tsx:93` gọi store `logout()` thay vì
  `logoutSession()` → refresh cookie còn sống ở server; `queryClient.clear()` không được gọi →
  user kế tiếp trong cùng tab thấy cache của user trước. Banner "OTP code is 999999" render vô
  điều kiện (`routes/_auth/login.tsx:161,256`).

## Requirements
- Mỗi endpoint trên check đúng op trên đúng OA (xem Unresolved Q1 trong plan.md) và trả 403 khi DENY.
- `CreatePermission`: caller phải có `manage` trên OA đích **và** chỉ được cấp các op mà chính
  caller đang có trên OA đó (chống leo thang qua delegation).
- `ListAssets`/`ListRequests`: lọc theo quyền `read`, không trả 403 cho cả danh sách.
- gRPC nội bộ: `user_ngac_node_id` phải lấy từ JWT claims đã verify ở biên REST, truyền qua
  metadata; server từ chối request thiếu metadata. (Nếu quá lớn → tách thành phase riêng, ghi rõ.)
- WebSocket: `Subscribe` check `read` trên OA của channel qua policy; DENY → `ErrorEvent 403`,
  không thêm vào group. Approval/presence chỉ phát trong tenant (và chỉ tới user liên quan với
  approval). Phần realtime còn lại thuộc phase 05.
- Frontend: một hàm logout duy nhất = `logoutSession()` + `queryClient.clear()`; banner OTP chỉ
  khi `import.meta.env.DEV`.

## Related files
- modify `backend/services/workspace/internal/{rest,domain,grpc}/*` (+ policy client wiring trong `cmd/main.go`)
- modify `backend/services/messaging/internal/domain/service.go`
- modify `backend/services/drive/internal/grpc/sharing.go`
- modify `backend/services/asset/internal/grpc/{asset_server,request_server,asset_type_server}.go`
- modify `backend/services/messaging/internal/grpc/hub.go` (subscribe check, tenant-scoped broadcast)
- modify `frontend/src/api/client.ts`, `frontend/src/stores/auth.store.ts`,
  `frontend/src/components/patterns/AppSidebar.tsx`, `frontend/src/routes/assets.tsx`,
  `frontend/src/routes/_auth/login.tsx`
- create `docs/specs/{workspace-admin-authorization,resource-pep-coverage,session-logout}/spec.md`
- modify `docs/specs/README.md` (index)

## Implementation steps
1. Viết bảng PEP coverage (endpoint → op → OA) vào spec `resource-pep-coverage` trước.
2. Mỗi endpoint: test DENY đỏ (user không có association) → thêm check → test ALLOW + DENY xanh.
3. Test riêng cho leo thang: user có `read` gọi `CreatePermission` cấp `manage` → 403.
4. Frontend: test vitest cho logout (gọi `/auth/logout`, cache rỗng sau đó).
5. `doubt-driven-development` trên diff trước khi merge.

## Success criteria
- [ ] Mọi endpoint ở trên có test DENY.
- [ ] Không còn đường nào tạo association mà không qua check `manage`.
- [ ] Logout từ mọi nơi revoke cookie và xoá cache.

## Risks
- UI hiện có thể đang dựa vào việc không bị check (vd. admin screen của user thường) → chạy lại
  e2e `scripts/e2e_approval_test.sh` và `test_app.sh`.
