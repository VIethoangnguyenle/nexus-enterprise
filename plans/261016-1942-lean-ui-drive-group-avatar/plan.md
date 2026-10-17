---
title: "UI tinh gọn: ô soạn trên bàn phím iPhone, ảnh đại diện & ảnh nhóm, Drive kiểu Google Drive"
status: completed
priority: P1
branch: main
created: 2026-10-16
brainstorm: plans/reports/brainstorm-261016-1942-drive-gallery-refactor.md
relates: [261011-1035-file-preview]
blockedBy: []
blocks: []
---

# UI tinh gọn — bàn phím, ảnh đại diện & ảnh nhóm, Drive + tab Tệp

Yêu cầu người dùng (2026-10-16): refactor Drive dễ nhìn như Google Drive, có preview, refactor tab Tệp trong chat,
cho web / máy tính bảng / điện thoại; UI tinh gọn (vd chạm avatar để đổi ảnh, không có nút riêng); đổi ảnh nhóm;
ô soạn chat trên iPhone (app đã cài) đang cách bàn phím một khoảng cố định, muốn sát như Google Chat.

Hợp đồng đầy đủ (outcome, constraints, non-goals, acceptance) và mọi quyết định của người dùng: xem brainstorm
report ở frontmatter. Plan này không lặp lại, chỉ phân pha.

## Capability trong `docs/specs/`
- Sửa: `file-preview` (thumbnail), `drive-context-panel` (panel preview), `chat-attachments` (tab Tệp dùng chung),
  `contacts-documents-settings-screens` (AvatarEdit ở Hồ sơ).
- Thêm: `chat-channel-avatar` (ảnh nhóm: lưu, quyền `manage`, realtime).
- Không đổi mô hình NGAC: không node, không thao tác mới; quyền mới dùng lại `read` (thumbnail) và `manage`
  (ảnh nhóm) trên OA sẵn có.

## Phases

| # | Phase | PR | Status | Phụ thuộc |
|---|-------|----|--------|-----------|
| 01 | [Thiết kế: mockup + DESIGN.md (duyệt trước code)](phase-01-design.md) | 1 + 2 | done, production `0bf4455` (#20/#21/#22) | — |
| 02 | [Ô soạn sát bàn phím trên iPhone PWA](phase-02-composer-keyboard.md) | 1 | done, production `0bf4455` (#20/#21/#22) | 01 |
| 03 | [AvatarEdit dùng chung + chuẩn hoá ảnh thành `backend/pkg`](phase-03-avatar-edit.md) | 1 | done, production `0bf4455` (#20/#21/#22) | 01 |
| 04 | [Ảnh nhóm: backend messaging](phase-04-group-avatar-backend.md) | 1 | done, production `0bf4455` (#20/#21/#22) | 03 |
| 05 | [Ảnh nhóm: frontend](phase-05-group-avatar-frontend.md) | 1 | done, production `0bf4455` (#20/#21/#22) | 03, 04 |
| 06 | [Thumbnail: backend drive](phase-06-thumbnail-backend.md) | 2 | done, production `0bf4455` (#20/#21/#22) | 01 |
| 07 | [Bộ hiển thị tệp dùng chung (`components/files/`)](phase-07-file-grid.md) | 2 | done, production `0bf4455` (#20/#21/#22) | 01, 06 |
| 08 | [Màn Tài liệu: lưới / danh sách / panel / phím tắt](phase-08-drive-screen.md) | 2 | done, production `0bf4455` (#20/#21/#22) | 07 |
| 09 | [Tab Tệp trong chat](phase-09-space-files.md) | 2 | done, production `0bf4455` (#20/#21/#22) | 07 |
| 10 | [Kiểm chứng + review cho mỗi PR](phase-10-verify-review.md) | 1 + 2 | done, production `0bf4455` (#20/#21/#22) | 02–05 (PR 1), 06–09 (PR 2) |

PR 1 = 01 (phần ô soạn + avatar) + 02–05 + 10. PR 2 = 01 (phần tệp) + 06–09 + 10. Phase 01 được duyệt một lần cho cả hai.

## Acceptance (tóm tắt — chi tiết ở brainstorm report §Acceptance 1–9)
- Mockup + DESIGN.md được duyệt trước khi có code UI.
- iPhone PWA: ô soạn sát mép bàn phím khi gõ (đo bằng số trước/sau), không đè vạch home khi bàn phím đóng.
- Không còn nút "Đổi ảnh"/"Bỏ ảnh" bên ngoài; ảnh nhóm đổi được bởi người có `manage`, 403 với người khác, realtime.
- Drive có lưới thumbnail thật (ảnh, PDF, Office, MP4), panel preview, Space/Enter mở FileViewer; tab Tệp dùng chung.
- Ba khổ 390 / 820 / 1440 đúng bố cục; mọi cổng `make` + `npm` xanh; `make db-migrate` đã chạy.

## Quan hệ với plan khác
- `261011-1035-file-preview`: thumbnail (06) dùng lại pipeline Gotenberg + cache MinIO của nó; không chặn nhau.
  Pha 03 của plan đó ghi "pending" nhưng FileViewer đã ship (`ce64a74`) — trạng thái cũ, không sửa ở đây.

## Rủi ro chính
- Đo bàn phím iPhone cần máy thật (người dùng chạy). Không có số đo → không sửa phần ước đoán (xem phase 02).
- Tách chuẩn hoá ảnh khỏi auth (03): nếu dính chặt `store.AvatarImages`, tách kiểu kết quả ra pkg trước.
- Ảnh image drive +80–120 MB và CPU khi sinh thumbnail hàng loạt (06): giới hạn đồng thời + cache.

## Câu hỏi còn mở
- Không còn.
