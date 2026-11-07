---
phase: 4
title: "Sự kiện activity realtime cho badge thanh bên"
status: pending
priority: P2
effort: "10h"
dependencies: [3]
---
# Phase 4: Sự kiện activity realtime

## Mục tiêu
Badge, preview và pill mention của mọi channel cập nhật trong < 1 s mà client không gọi lại API; mỗi tin tốn đúng một
publish Redis; chỉ thành viên channel nhận sự kiện.

## Thiết kế
- `ws.proto`: `ConversationActivityEvent { channel_id, workspace_id, seq, root_id, reply_seq, sender_id,
  repeated mentions, kind (sent|deleted|read) }`. Không mang nội dung tin. Thêm vào `ServerEnvelope` oneof. `kind=read`
  gửi cho chính người đọc (các thiết bị khác) với `last_read_seq` mới. Xoá `UnreadCountEvent` không dùng nếu không còn
  chỗ tham chiếu (giữ số field, đánh `reserved`).
- Server: sau khi gửi/xoá tin (cạnh `BroadcastToChannel` trong `grpc/server.go:SendMessage`, và đường xoá trong
  `rest/lifecycle.go`/`grpc` DeleteMessage) gọi `hub.PublishActivity(tenantID, ev)` → một `PUBLISH activity:<tenant>`.
- Mỗi instance: khi nhận `activity:<tenant>`, lấy tập thành viên channel từ cache `channelMembers` trong bộ nhớ
  (nạp lười từ `channel_members` qua store, TTL 60 s), rồi gửi cho client cục bộ của tenant đó có `userID` thuộc tập.
  Cache bị xoá khi nhận sự kiện domain `channel` kiểu `member_added`/`member_removed` (đã có trong
  `backend/pkg/realtime/event.go`) và khi workspace revoke (`revokeWorkspaceLocal`).
- Client (`stores/websocket.store.ts`, `lib/chat-cache.ts`): reducer `applyActivity(ev)`:
  - bỏ qua nếu `sender_id` là mình (trừ `kind=read`);
  - channel đang mở và tab đang hiện: không cộng, gọi mark-read như hiện nay;
  - ngược lại: cộng `unread_count` (tin chính, theo mức thông báo đã có trong cache), `thread_unread_count` (reply của
    thread mình theo dõi — dựa vào cache thread state), `mention_count` (mình có trong `mentions`); cập nhật seq
    cuối của preview và invalidate riêng `conversationActivity` của channel đó (preview cần nội dung, một request nhỏ
    đã có NGAC check);
  - `kind=deleted`: trừ nếu `seq > last_read_seq` đã biết;
  - lệch seq (nhảy cóc) → refetch `unreadCounts` một lần (phục hồi khi mất frame).
  - Bỏ `invalidateQueries(unreadCounts)` ở `websocket.store.ts:595`.

## Files
- `backend/proto/messaging/ws.proto`; `make proto` **và** `cd frontend && npm run proto:gen`.
- `backend/services/messaging/internal/grpc/hub_activity.go` (mới), `hub_broadcast.go` (subscribe `activity:*`),
  `server.go`, đường xoá tin.
- `backend/services/messaging/internal/store/channel_members.go` (hoặc file store hiện có chứa `channel_members`):
  `ChannelMemberUserIDs(ctx, channelID)`.
- `frontend/src/stores/websocket.store.ts`, `frontend/src/lib/chat-cache.ts` (`applyActivity`), test tương ứng.

## Test (viết trước)
- Go: `hub_activity_test.go` — một publish → mỗi thành viên online nhận 1 frame; **deny**: client không phải thành viên,
  client tenant khác cùng `userID`, thành viên vừa bị gỡ (sau sự kiện `member_removed`) đều không nhận.
- Go: gửi 1 tin → đúng 1 lệnh `PUBLISH activity:<tenant>` (miniredis hoặc Redis test env theo memory dự án).
- Vitest: `chat-cache.test.ts` — bảng ca cho `applyActivity` (mình gửi, channel đang mở, mức `off`, mention, reply theo
  dõi/không theo dõi, xoá tin đã/chưa đọc, nhảy seq → refetch).
- E2E hai người (`frontend/e2e/`, theo mẫu `wave1-two-people.mjs`): B không mở channel, A gửi tin → badge B tăng trong < 1 s;
  A xoá → badge B giảm.

## Validation
`make test s=messaging`, `cd frontend && npm test && npm run typecheck:diff && npm run lint`, e2e hai người qua.

## Rủi ro / rollback
Channel 5.000 thành viên: lần nạp cache đầu là một query có index; sau đó lọc trong bộ nhớ. Cache lệch tối đa 60 s, chỉ
ảnh hưởng badge (không lộ nội dung: nội dung luôn qua endpoint có NGAC check). Rollback: tắt publish activity bằng cờ
env `MESSAGING_ACTIVITY_EVENTS=0`, client vẫn chạy nhờ refetch khi lệch seq / reconnect.
