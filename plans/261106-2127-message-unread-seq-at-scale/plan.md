---
title: "Tin nhắn ở quy mô lớn: đếm chưa đọc O(1) theo seq, badge realtime, phân trang theo seq"
description: "Số chưa đọc tính bằng số thứ tự theo channel thay cho COUNT(*), thanh bên cập nhật realtime bằng sự kiện nhỏ chỉ gửi cho thành viên, và phân trang lịch sử/thread theo seq."
status: pending
priority: P2
effort: 45h
branch: feat/message-unread-seq
base: origin/main (bbfeeac)
tags: [backend, frontend, database, messaging, realtime, performance]
created: 2026-11-06
blockedBy: []
blocks: []
report: ../reports/brainstorm-261106-2130-message-loading-at-scale.md
specs: [chat-unread-counts (mới), chat-threads, realtime-event-delivery, chat-message-lifecycle]
---

# Tin nhắn ở quy mô lớn — hướng A

Nguồn: `plans/reports/brainstorm-261106-2130-message-loading-at-scale.md` (hướng A). Các quyết định đã chốt với người dùng
ngày 2026-11-06:

- Badge thanh bên của channel **không mở** phải cập nhật realtime (hiện không: sự kiện `unreadCount` có trong proto
  nhưng server không gửi ở đâu).
- Số chưa đọc phải **chính xác** khi có tin bị xoá.
- Quy mô cần chứng minh (**Mid**): 2.000 tenant, tenant lớn nhất 5.000 người, một channel có 1.000 người đang mở,
  50 triệu tin nhắn, 200 tin/giây lúc cao điểm.
- Benchmark chạy trên **Postgres Docker local**, database riêng `ngac_bench` (không dùng `ngac_ci`, không đụng prod).

## Hiện trạng đã kiểm (sửa lại một điểm của brainstorm)

- Client chỉ subscribe realtime channel **đang mở** (`SpaceView.tsx:175`, `CallChat.tsx:45`). Mỗi `chatMessage` đến làm
  client gọi lại `unreadCounts` (`websocket.store.ts:595`). Bão truy vấn vì thế là **N người đang mở channel**, không
  phải N thành viên online.
- Badge của channel khác chỉ làm mới khi mở app, reconnect (`revalidateChatState`) hoặc sau một số thao tác.
  `refetchOnWindowFocus: false` (`lib/query-client.ts:41`).
- `GetUnreadCounts` (`store/reactions_pins_receipts.go:250`): `COUNT(*)` tin sau `last_read_at`; CTE `followed` có
  nhánh `sender_id = $1 OR mentions @> …` không giới hạn channel/tenant; bảng `messages` không có index `sender_id`.
- `ListMessages` (`store/store.go:322`): cursor chỉ có `created_at`; điều kiện xoá không khớp partial index `latest_main`.
  `GetThread` trả mọi reply.
- Hub: mỗi instance `PSubscribe("channel:*", …)` (`grpc/hub_broadcast.go:108`), tức nhận tin của mọi tenant.

## Thiết kế

- **`messages.seq`**: số thứ tự tăng dần của tin **chính** trong một channel, cấp từ `channels.last_seq` bằng trigger
  `BEFORE INSERT` trong cùng transaction insert (`UPDATE channels SET last_seq = last_seq + 1 … RETURNING`). Reply có
  `messages.reply_seq` cấp từ `messages.last_reply_seq` của gốc. Trigger thay cho sửa từng đường ghi: mọi đường chèn
  tin, kể cả code bản cũ còn chạy trong lúc deploy, đều có số; insert bị rollback thì số rollback theo, không có lỗ.
- *(Bỏ khi làm phase 2)* `messages.workspace_id`: bản sao của `channels.workspace_id` sẽ lệch khi channel đổi workspace
  (migration 066 từng chuyển DM), và khoá ngoại của nó chặn xoá workspace. Không acceptance nào cần; để lại cho hướng B.
- **Vị trí đọc**: `read_receipts.last_read_seq`, `thread_follows.last_read_reply_seq`. Giữ cột thời gian cũ cho tới khi
  không còn chỗ đọc.
- **Số chưa đọc chính xác**: `last_seq − last_read_seq − (số tin chính đã xoá có seq > last_read_seq)`. Tin xoá hiếm nên
  phần trừ dùng partial index nhỏ. Tin của chính người dùng: khi gửi, vị trí đọc của người gửi tiến tới seq mới
  (giữ đúng hành vi hiện tại là tin mình gửi không thành chưa đọc — kiểm ở phase 3).
- **Badge realtime**: mỗi tin chính hoặc reply sinh **một** sự kiện `ConversationActivityEvent` (channel, seq,
  reply_seq/root, người gửi, danh sách mention) publish một lần lên Redis theo tenant. Mỗi instance chỉ chuyển cho client
  cục bộ là thành viên channel, dựa vào cache thành viên trong bộ nhớ (nạp từ `channel_members`, xoá theo các sự kiện
  membership đã có). Client tự cộng vào cache `unreadCounts`, `conversationActivity`, pill mention. Không còn gọi lại
  API sau mỗi tin.
- **Phân trang**: cursor `before_seq` cho lịch sử, `after_reply_seq` + `limit` cho thread.
- **NGAC**: không đổi mô hình. Nội dung vẫn chỉ đọc qua endpoint đã check `read` trên OA của channel. Sự kiện activity
  không mang nội dung tin, và chỉ đến người có hàng `channel_members` (cache membership). Phase 4 phải chứng minh người bị
  gỡ khỏi channel không còn nhận sự kiện (test deny).

## Phases

| # | Phase | Effort | Phụ thuộc | Trạng thái |
|---|-------|--------|-----------|------------|
| 1 | [Benchmark harness + số liệu nền](phase-01-benchmark-baseline.md) | 6h | — | pending |
| 2 | [Schema seq + backfill (migration 072, 073)](phase-02-schema-seq.md) | 5h | 1 | pending |
| 3 | [Đếm chưa đọc O(1), vị trí đọc theo seq, phân trang theo seq](phase-03-unread-and-cursors.md) | 10h | 2 | pending |
| 4 | [Sự kiện activity realtime cho badge thanh bên](phase-04-activity-event.md) | 10h | 3 | pending |
| 5 | [Hub: subscribe Redis theo nhu cầu, reconnect có jitter](phase-05-hub-scaling.md) | 6h | 4 | client xong; backend để lại (xem phase) |
| 6 | [Frontend: cursor seq, giới hạn cache lịch sử, phân trang thread](phase-06-frontend-history.md) | 4h | 3 | pending |
| 7 | [Đo lại, cập nhật spec, review](phase-07-verify-and-specs.md) | 4h | 1–6 | pending |

Phase 6 có thể chạy song song phase 4–5 (file tách biệt: phase 6 chỉ sửa `hooks/useThreads.ts`, `api/messaging.ts` phần
list/thread; phase 4 sửa `stores/websocket.store.ts` và `lib/chat-cache.ts`).

## Acceptance (toàn plan)

1. Trên `ngac_bench` (quy mô Mid): `GetUnreadCounts` cho một người có 200 channel đọc được chạy dưới **20 ms p95**, và
   thời gian **không đổi** khi tổng số tin của nền tảng tăng từ 5 triệu lên 50 triệu (chênh < 20%).
2. Một tin vào channel có 1.000 người đang mở: **0** request `unreadCounts` từ client. Server gửi đúng 1 publish Redis
   cho sự kiện activity, và mọi thành viên online nhận đúng 1 frame.
3. Badge của channel không mở tăng trong < 1 s sau khi tin được gửi (e2e hai người).
4. Số chưa đọc đúng tuyệt đối trong các ca: tin bị xoá trước/sau khi đọc, "đánh dấu chưa đọc", reply trong thread đang
   theo dõi, mức thông báo `off`/`main`/`all`, mention.
5. Trang lịch sử 50 tin: < 10 ms p95 trên channel 1 triệu tin, không mất/trùng tin ở ranh giới trang kể cả khi
   nhiều tin cùng `created_at`.
6. Người bị gỡ khỏi channel không nhận sự kiện activity của channel đó nữa (test deny).
7. `make test`, `make check-layering`, `make check-ngac`, `cd frontend && npm test && npm run typecheck:diff && npm run build` đều qua.

## Rủi ro chính

- `UPDATE channels … RETURNING last_seq` khoá dòng channel: ghi tuần tự trong một channel. Ở 200 tin/giây toàn nền tảng
  thì một channel nóng vẫn dưới vài chục tin/giây; phase 1 đo, phase 3 xác nhận.
- Backfill seq trên prod: bảng nhỏ, chạy trong migration. Migration phải chạy được nhiều lần (theo cách các migration
  hiện có làm), và **không sửa** migration/init.sql đã áp dụng.
- Cache membership trong bộ nhớ có thể lệch: dùng TTL ngắn + xoá theo sự kiện; lệch chỉ ảnh hưởng badge, không lộ
  nội dung.
- Thread unread ở mức `all` vẫn đếm theo số reply chưa đọc (bị chặn bởi index `(channel_id, reply_seq)`), không O(1).
  Chấp nhận: mức `all` là tuỳ chọn ít dùng; phase 1 đo.

## Ngoài phạm vi

Partition/Citus, search engine riêng, kho tin nhắn khác (hướng B, C). Thay đổi UI ngoài hành vi badge.
