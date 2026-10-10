---
phase: 2b
title: "gRPC caller identity"
status: done
priority: P2
effort: 3-4d
dependencies: [1]
---

# Phase 02b — gRPC caller identity

## Overview
Phần còn lại của phase 02. Các gRPC server vẫn lấy danh tính caller từ body request
(`user_ngac_node_id`, 47 chỗ trong 5 file `.proto`); không server nào đọc metadata. Cổng gRPC chỉ
nằm trong mạng nội bộ (xác nhận 2026-10-10), nên đây là phòng thủ chiều sâu, không phải lỗ hổng mở.

## Key insights
- `asset` dùng giá trị context trong tiến trình (`internal/caller`) cho các RPC không có field
  caller (`UpdateQuotaRequest`, `GetTypeRequest`, `ListTypesRequest`, `GetRequestReq`) → gọi qua
  mạng luôn bị DENY (commit `e135d4f`).
- Biên REST đã verify JWT; chỉ thiếu bước chuyển claims sang gRPC một cách đáng tin.

## Requirements
- Biên REST đặt caller (từ JWT claims đã verify) vào metadata gRPC qua một interceptor client dùng
  chung trong `backend/pkg`.
- Interceptor server đọc metadata, từ chối request thiếu caller; handler lấy caller từ context,
  không từ body.
- Bỏ workaround `internal/caller` của asset. Field `user_ngac_node_id` trong body: deprecate, không
  còn được tin.
- Proto đổi → `make proto` **và** `cd frontend && npm run proto:gen`.

## Success criteria
- [ ] Mỗi gRPC server có test: thiếu metadata → `Unauthenticated`; body nói user A, metadata nói
      user B → quyết định theo B.
- [ ] Không handler nào đọc `user_ngac_node_id` từ body.
- [ ] Các RPC của asset ở trên chạy được qua mạng.

## Risks
- Rộng (8 service). Có thể làm theo từng service, mỗi service một commit.
- Xác thực service-to-service (mTLS / token nội bộ) **ngoài phạm vi** — ghi lại nếu cần sau.

## Spec
Modify `docs/specs/resource-pep-coverage/spec.md` — nguồn danh tính caller cho gRPC.
