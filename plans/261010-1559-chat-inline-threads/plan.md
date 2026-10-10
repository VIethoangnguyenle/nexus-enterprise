---
title: "Chat: Google-Chat-style in-line threading"
description: "Bring Chat to Google Chat parity: in-line threading with thread follow and per-thread unread, staged multi-file attachments, rich-text (HTML) messages sanitized on the server, then message lifecycle and notification levels."
status: pending
priority: P1
effort: 12-15d
issue:
branch: claude/inspiring-cray-32y4n1
tags: [feature, frontend, backend, realtime, chat]
blockedBy: []
blocks: []
created: 2026-10-10
---

# Chat — in-line threading như Google Chat

Nguồn: [brainstorm](../reports/brainstorm-261010-1559-chat-inline-threads.md) (đã chấp nhận). Nền: `main` @ 61616dc.
Liên quan, không chặn: `project:261009-0449-project-wide-refactor` phase 06 (UI Tín hiệu — plan này thay
`TopicStream` của phase đó).

## Mục tiêu (contract)
- **Outcome:** Nhóm và DM dùng in-line threading. Luồng chính theo thời gian, gom tin liên tiếp; mỗi tin có tóm
  tắt thread do server trả (số trả lời, giờ cuối, người trả lời, số mới); theo dõi/bỏ theo dõi thread; chưa đọc
  theo thread; panel "Thread" của nhóm (lọc Theo dõi / Chưa đọc); vạch "Tin mới" + nút nhảy tới chưa đọc.
  Đợt 2: **đính kèm** như Google Chat (hàng chờ trong ô soạn, nhiều tệp + chữ trong một tin, kéo-thả, dán ảnh,
  lưới ảnh, trình xem) và **soạn tin định dạng HTML** (thanh định dạng, phím tắt, cú pháp gõ nhanh, link; server
  lọc allowlist). Đợt 3: sửa/xoá/trích dẫn/đánh dấu chưa đọc. Đợt 4: mức thông báo + Trang chủ ưu tiên thread.
- **Non-goals:** huddle/call, bot, chuyển tiếp vào email, soạn chung realtime, E2E, đại tu tìm kiếm, migrate dữ
  liệu tin (reply đã có `parent_message_id`).

## Phases

| # | Phase | Đợt | Priority | Effort | Depends on | Status |
|---|---|---|---|---|---|---|
| 01 | [Thiết kế Chat: thread, đính kèm, định dạng](phase-01-design-inline-threads.md) | 1 | P1 | 1-1.5d | — | pending |
| 02 | [Mô hình dữ liệu và API thread](phase-02-thread-data-model-and-api.md) | 1 | P1 | 2d | 01 (chốt hành vi) | pending |
| 03 | [Sự kiện realtime cho thread](phase-03-realtime-thread-events.md) | 1 | P1 | 1d | 02 | pending |
| 04 | [UI luồng in-line, panel Thread](phase-04-inline-stream-ui.md) | 1 | P1 | 2-3d | 01, 02, 03 | pending |
| 05 | [Kiểm chứng đợt 1](phase-05-wave1-verification.md) | 1 | P1 | 0.5d | 04 | pending |
| 06 | [Đính kèm như Google Chat](phase-06-attachments.md) | 2 | P1 | 2-2.5d | 05 | pending |
| 07 | [Soạn tin định dạng HTML](phase-07-rich-text-messages.md) | 2 | P1 | 1.5d | 05 | pending |
| 08 | [Vòng đời tin nhắn](phase-08-message-lifecycle.md) | 3 | P2 | 2d | 05, 07 | pending |
| 09 | [Mức thông báo, Trang chủ theo thread](phase-09-notification-levels.md) | 4 | P3 | 1.5d | 08 | pending |

```text
01 design (cả 4 đợt) ──► 02 data+API ──► 03 realtime ──► 04 UI ──► 05 verify   │ đợt 1 (merge riêng)
                                                                  ├──► 06 attachments ┐ đợt 2 (song song)
                                                                  └──► 07 rich text ──┴─► 08 lifecycle ──► 09 notifications
```

## Capabilities in `docs/specs/`
| Capability | Action | Phase |
|---|---|---|
| `chat-threads` | **new** — follow, per-thread unread, thread summary, threads list | 02, 03, 04 |
| `realtime-event-delivery` | modify — thread reply fan-out to followers, thread-state event | 03 |
| `lark-messaging-layout` | modify or mark superseded by in-line threading | 04 |
| `chat-attachments` | **new** — staged multi-file attachments, channel-drive ownership rule | 06 |
| `chat-threads` (or `chat-rich-text`) | modify/new — HTML allowlist, plain-text copy, mentions as nodes | 07 |
| `chat-message-lifecycle` | **new** — edit/delete/quote/mark unread | 08 |
| `notifications` | modify — per-space notification level | 09 |

## Ground rules (CLAUDE.md §5)
- Thiết kế trước: DESIGN.md + `design/mockups/spaces.html` được duyệt rồi mới code UI.
- `ak:cook --tdd`: test đỏ trước; mọi nhánh quyền mới có test **deny**.
- Tin nhắn không là node NGAC → mọi check trên OA kênh (`read`/`write`/`manage`), op từ `backend/ngac`.
- Đổi `messaging.proto`/`ws.proto` → `make proto` **và** `cd frontend && npm run proto:gen`.
- Migration mới → backup DB, `make db-migrate`, kiểm chứng trên DB chạy thật.
- Route REST mới → có trong `frontend/vite.config.js` (khối regex nếu dưới `/api/workspaces/:id/`).
- Không UUID nào lên màn hình.

## Acceptance (đợt 1)
1. B trả lời thread A theo dõi → tóm tắt thread + panel Thread của A hiện "N mới" ngay (WS); thread A không theo
   dõi không tăng; A tự theo dõi thread khi tạo, trả lời hoặc được nhắc tên.
2. Mở nhóm có tin chưa đọc → vạch "Tin mới" đúng vị trí + nút "Nhảy tới tin chưa đọc"; đọc xong → số về 0.
3. Thread: mở panel → đánh dấu đã đọc thread; bỏ theo dõi → không còn đếm.
4. Không UUID trên màn hình; Go tests (0 skip), vitest, lint, typecheck:diff, bundle production xanh; CI xanh.

## Acceptance (đợt 2)
1. Chữ + nhiều tệp gửi thành **một** tin; ảnh hiện lưới, bấm mở trình xem; tệp đính kèm không thuộc drive của
   kênh bị server từ chối (test deny).
2. Đủ định dạng của thanh công cụ + phím tắt + cú pháp gõ nhanh; HTML độc gửi thẳng API bị server lọc (test deny);
   tìm kiếm dùng bản văn bản thuần.

## Risks
- Hiệu năng đếm chưa đọc theo thread trên kênh lớn → index `(parent_message_id, created_at)`; đo trên dữ liệu thật.
- Đổi hành vi gửi tin (Enter = tin chính, không mở chủ đề mới) có thể gây bỡ ngỡ → copy rõ trong composer.

## Unresolved questions
- "Đồng thời gửi vào nhóm" khi trả lời thread? Mặc định **không** (Google Chat không có).
