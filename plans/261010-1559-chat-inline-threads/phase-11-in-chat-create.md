---
phase: 11
title: "Tạo trong chat: bình chọn, công việc, tài liệu, phê duyệt"
status: in-progress
priority: P2
effort: "2.5-3d"
dependencies: [1, 5]
---

# Phase 11: Tạo trong chat

## Overview
Nút "+" trong ô soạn tạo bình chọn, công việc, tài liệu mới và đề nghị phê duyệt; mỗi thứ là một thẻ trong tin,
cập nhật realtime. Thiết kế: DESIGN.md "Chat: tạo trong chat" + mockup `spaces.html` §6. Làm sau đợt 1 (người dùng
chọn 2026-10-10).

## Hiện trạng (scout)
- Bình chọn: đã có bảng `polls`, `poll_options`, `poll_votes` (`data/migrations/002_chat_polls.sql`) và REST
  `POST /channels/:chId/polls`, `POST|DELETE /polls/:pollId/vote`, `GET /polls/:pollId`
  (`backend/services/messaging/internal/rest/polls_tasks.go`); frontend chưa có UI.
- Công việc: đã có `chat_tasks` (`003_chat_tasks.sql`) và REST tạo/sửa/liệt kê; tab Công việc (`SpaceTasks.tsx`).
- Tin liên kết thực thể qua `linked_entity_type`/`linked_entity_id` (proto đã ghi "poll", "task", "document").

## Requirements
- Bình chọn: 2–10 lựa chọn, chọn nhiều, ẩn danh, hạn đóng, đóng sớm (người tạo/quản lý nhóm), rút phiếu; chưa bỏ
  phiếu không thấy kết quả; ẩn danh thì API **không trả** danh tính người bỏ phiếu. Sự kiện realtime khi có phiếu.
- Công việc: tạo từ chat với người làm (thành viên nhóm) + hạn; đánh dấu xong từ thẻ (người được giao/người tạo).
- Tài liệu mới: tạo trong thư mục của nhóm qua dịch vụ documents/drive; thẻ trỏ tới tài liệu.
- Đề nghị phê duyệt: tạo qua dịch vụ approvals, gắn kênh; thẻ đọc trạng thái; ẩn ở workspace cá nhân.
- Quyền: tạo cần `write` trên OA kênh; bỏ phiếu cần `read` trên OA kênh; đóng bình chọn = người tạo hoặc `manage`;
  thẻ tài liệu/phê duyệt chỉ hiện chi tiết khi người xem có `read` trên OA của đối tượng đó. Test **deny** cho
  từng nhánh (bỏ phiếu khi không là thành viên, đóng bình chọn của người khác, xem thẻ phê duyệt không có quyền).

## Related Code Files
- Modify: `backend/services/messaging/internal/{domain,store,rest}/polls_tasks.go` (đóng sớm, ẩn danh, realtime)
- Modify: `backend/proto/messaging/ws.proto` (sự kiện phiếu) → `make proto` + `npm run proto:gen`
- Create: `frontend/src/components/chat/{CreateMenu,CreateDialog,PollCard,TaskCard,DocCard,ApprovalCard}.tsx` (+ test)
- Create: `docs/specs/chat-cards/spec.md`

## Success Criteria
- [ ] Bốn loại thẻ khớp mockup ở light/dark/390px; realtime phiếu và trạng thái duyệt.
- [ ] Test deny đủ các nhánh quyền; mọi gate xanh; không ID nào lên màn hình.

## Tiến độ
- Backend (bình chọn, công việc) xong 2026-10-11: 2–10 lựa chọn, `ends_at` được đọc, bỏ phiếu cần `read`, chọn một thay phiếu cũ, đóng sớm (người tạo/manage, migration 042), ẩn kết quả tới khi bỏ phiếu/đóng, ẩn danh không lộ người bỏ phiếu, sự kiện chỉ mang id; người được giao phải đọc được kênh; đổi trạng thái công việc chỉ người được giao/người tạo/manage. Spec `chat-cards`. **Còn:** UI menu +, các thẻ; thẻ tài liệu/phê duyệt lấy chi tiết qua dịch vụ gốc theo quyền người xem.
- Review: `plans/reports/code-reviewer-261011-0830-chat-wave2-backend.md` (C1, H1, H2, M1–M5, M8 đã sửa; M6 bằng dòng hệ thống; M7 — sửa tin không đổi danh sách nhắc tên — giữ nguyên, ghi vào câu hỏi mở).
