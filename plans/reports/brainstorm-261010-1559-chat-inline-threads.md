---
title: "Chat: Google-Chat-style message and thread management"
type: brainstorm
status: accepted
created: 2026-10-10
---

# Brainstorm — Chat theo cơ chế Google Chat

## Summary
Chuyển Chat từ topic model (mỗi tin gốc = thẻ chủ đề, `components/spaces/TopicStream.tsx`) sang
**in-line threading** như Google Chat hiện nay, kèm theo dõi thread, chưa đọc theo thread, danh sách
thread, vòng đời tin nhắn. Chia 3 đợt. Ưu tiên số 1 của người dùng.

## Evidence (main @ 61616dc)
- `Message` proto chỉ có `reply_count`; không `last_reply_at`, người trả lời, `edited_at`, `deleted_at`.
- Không có `PATCH`/`DELETE /messages/:id`; không thread follow; không thread read state.
- `read_receipts` theo kênh; đếm chưa đọc lọc `parent_message_id IS NULL` → tin trả lời không bao giờ tính chưa đọc.
- Có sẵn: `GET /messages/:msgId/thread`, `GET /channels/:chId/messages?before=`, `POST /channels/:chId/read`,
  search, pins, reactions, tasks.

## Contract
- **Outcome:** Nhóm và DM dùng in-line threading. Luồng chính theo thời gian, gom tin liên tiếp; mỗi tin có
  tóm tắt thread từ server; theo dõi/bỏ theo dõi thread; chưa đọc theo thread; panel "Thread" của nhóm;
  vạch "Tin mới" + nhảy tới chưa đọc; sửa/xoá tin có trạng thái rõ.
- **Constraints:** thiết kế trước (`design/mockups/spaces.html`, `DESIGN.md`); sửa/xoá kiểm trên OA kênh
  (sửa tin mình: `write`; xoá tin người khác: `manage`) — tin nhắn không là node; proto đổi → `make proto` +
  `npm run proto:gen`; migration áp + kiểm chứng; sự kiện WS cho sửa/xoá/trả lời thread; giữ wash realtime;
  TDD, test deny; spec mới `chat-threads`.
- **Non-goals:** huddle/call, bot, chuyển tiếp vào email, soạn chung realtime, E2E, đại tu tìm kiếm. Không
  migrate dữ liệu (reply đã có `parent_message_id`).
- **Acceptance:**
  1. B trả lời thread A theo dõi → số "mới" tăng trên tóm tắt + panel Thread của A; thread không theo dõi không tăng.
  2. Mở nhóm có tin chưa đọc → vạch "Tin mới" + nút nhảy.
  3. Sửa/xoá: người đang xem thấy ngay qua WS; người không quyền 403 (test deny).
  4. Không UUID trên màn hình; test, lint, typecheck, production bundle đều xanh.

## Options
- A. Chỉ đổi UI — rẻ, không đạt "cơ chế" (thiếu follow/unread/sửa/xoá).
- B. Làm đủ một lần — PR quá lớn, rủi ro dây chuyền.
- **C. Chia đợt (chọn):**
  1. Lõi thread: luồng in-line, tóm tắt thread server-side (`last_reply_at`, người trả lời), follow tự động
     (tạo/trả lời/được nhắc) + thủ công, chưa đọc theo thread, panel Thread (lọc Theo dõi/Chưa đọc), vạch "Tin mới".
  2. Vòng đời tin: sửa ("Đã chỉnh sửa"), xoá (dòng "Tin nhắn đã bị xoá"), trích dẫn khi trả lời, đánh dấu chưa đọc.
  3. Thông báo: mức theo nhóm (Tất cả / Tin chính + thread theo dõi / Tắt); Trang chủ ưu tiên thread theo dõi có tin mới.

## Decisions
- In-line threading thay topic model; chia 3 đợt; DM cũng có thread — theo đề xuất, người dùng chưa phản đối
  và yêu cầu làm Chat trước.

## Unresolved questions
- Trả lời thread có tuỳ chọn "Đồng thời gửi vào nhóm" như Slack? (Google Chat không có — mặc định không.)
