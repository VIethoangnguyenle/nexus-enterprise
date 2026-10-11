---
title: "Xem trước mọi tệp đính kèm"
status: in-progress
priority: P1
branch: claude/inspiring-cray-32y4n1
created: 2026-10-11
---

# Xem trước mọi tệp đính kèm

Yêu cầu (người dùng, 2026-10-11): "Tất cả attachment trên Nexus đều phải có cơ chế preview."

## Contract
- **Outcome:** mọi tệp ở chat (đính kèm), Tài liệu (drive) và mọi nơi khác có tệp đều mở được trình xem chung:
  ảnh, PDF, video, âm thanh, văn bản/mã/CSV, và Office (Word/Excel/PowerPoint/ODF) qua bản PDF do server đổi.
- **Constraints:** không gửi tệp ra ngoài (dịch vụ chuyển đổi chạy nội bộ); quyền xem trước = quyền tải tệp
  (`read` trên tệp/OA như download hiện tại); không UUID trên UI; thiết kế trước (DESIGN.md + mockup).
- **Non-goals:** sửa tệp Office trực tuyến; xem trước tệp nén (zip) — hiện thẻ + Tải xuống.
- **Acceptance:** mở được từng loại trên 1440 và 390; Office lần đầu có trạng thái "Đang chuẩn bị bản xem trước",
  lần sau mở ngay (cache); người không có quyền tải → 403, không lộ tên; tệp quá lớn / lỗi chuyển đổi → thông báo
  rõ + nút Tải xuống.

## Quyết định
1. Office → PDF trên server bằng **Gotenberg** (LibreOffice) chạy như container nội bộ (người dùng chốt 2026-10-11).
2. Bản PDF lưu ở MinIO, khoá theo tệp + phiên bản; chuyển đổi khi được xem lần đầu (lazy), có giới hạn kích thước/thời gian.

## Phases
| # | Phase | Status |
|---|---|---|
| 01 | Thiết kế trình xem tệp chung (mockup `design/mockups/file-viewer.html`) | in-progress |
| 02 | Backend: Gotenberg + `GET /api/drive/files/{id}/preview` + cache MinIO | done 2026-10-11 |
| 03 | Frontend: `FileViewer` dùng chung (chat, Tài liệu, thẻ tệp) — thay `ImageViewer` của chat 06 | pending |
| 04 | Kiểm chứng trên stack thật (mọi loại, 2 cỡ màn, quyền) | pending |

## Capability in `docs/specs/`
Tạo `docs/specs/file-preview/spec.md` (phase 02 viết phần server, phase 03 phần client).
