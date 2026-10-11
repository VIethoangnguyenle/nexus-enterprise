---
phase: 5
title: "Chuyển sang tổ chức"
status: in-progress (backend xong)
priority: P2
effort: "1-1.5d"
dependencies: [3, 4]
---

# Phase 05: Chuyển sang tổ chức

## Requirements
- REST `POST /api/workspaces/:id/convert-to-organization` (thêm vào khối regex `/api/workspaces` trong
  `vite.config.js`); chỉ chủ workspace (`manage` trên OA workspace, op từ `backend/ngac`); một chiều.
- Khởi tạo những gì workspace tổ chức cần mà cá nhân thiếu (scout `tenant-ngac-init`, schema phê duyệt theo tenant);
  mọi ghi `ngac_*` qua EPP invalidation; idempotent.
- Phát sự kiện realtime để mọi phiên của workspace hiện ngay các mục mới.
- Test **deny**: không phải chủ → 403; đã là tổ chức → 409; test allow.
- UI Cài đặt theo mockup phase 01; ghi nhật ký quản trị (ai chuyển, lúc nào).

## Success Criteria
- [ ] Chủ chuyển được, các mục xuất hiện không cần tải lại; người khác bị từ chối; spec cập nhật.

## Tiến độ
- [x] Backend (2026-10-11): `POST /api/workspaces/:id/convert-to-organization` (chủ + `manage`, 403/404/409
  `already_organization`, atomic `UPDATE ... WHERE type='personal'`, realtime `KindUpdated`); migration 046
  (`converted_by`, `converted_at`); `GET .../details` thêm `type`, `is_owner`, `owner_name`, `converted_by_name`,
  `converted_at`; approval tự tạo schema khi tổ chức gọi lần đầu; test deny/allow; spec `tenant-identity` +
  `approval-screens`. Không ghi đồ thị (tổ chức không cần node mà cá nhân thiếu). Không có nhật ký quản trị riêng ở
  workspace service: `converted_by`/`converted_at` là bản ghi.
- [ ] Frontend: UI Cài đặt theo mockup (worker khác).
