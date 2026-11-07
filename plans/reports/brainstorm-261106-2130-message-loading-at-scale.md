# Brainstorm: tải tin nhắn khi có hàng nghìn công ty

Ngày: 2026-11-06 · Loại: nghiên cứu (chưa sửa code)

## Contract

- **Outcome:** biết chính xác hệ thống đang tải tin nhắn thế nào, điểm nào vỡ trước khi lên hàng nghìn tenant
  với lượng tin nhắn rất lớn, và chọn một hướng sửa có lộ trình.
- **Constraints:** mọi quyết định truy cập vẫn đi qua NGAC trên OA của channel. Thay đổi schema chỉ bằng
  migration mới (init.sql và các migration đã chạy được kiểm checksum trên prod). Giữ nguyên hợp đồng proto/REST
  hoặc đổi cả hai phía. Prod hiện là một VPS dùng chung.
- **Non-goals:** chưa đổi sang kho tin nhắn khác, chưa triển khai search engine, chưa đụng UI.
- **Acceptance (cho giai đoạn sửa đầu tiên):** một tin nhắn gửi vào channel N thành viên đang online không còn gây N
  truy vấn đếm chưa đọc nặng. Chi phí đếm chưa đọc không phụ thuộc tổng số tin nhắn của cả nền tảng hay độ dài
  lịch sử. Có test tải chứng minh bằng số liệu (mục tiêu số cụ thể: xem câu hỏi mở).

## Cơ chế hiện tại

**Lịch sử channel**
- `GET` tin nhắn → `domain.GetMessages` (`internal/domain/messages.go:170`): check NGAC `read` trên channel, tối đa
  50 tin/trang, cursor `before` là thời điểm RFC 3339.
- `store.ListMessages` (`internal/store/store.go:322`): `WHERE channel_id=$1 AND parent_message_id IS NULL AND
  (deleted_at IS NULL OR reply_count>0) AND created_at < $before ORDER BY created_at DESC LIMIT n+1`.
- Mỗi trang được làm giàu theo lô: reactions, attachments, ghim (toàn bộ ghim của channel), quote, trạng thái thread.
- Frontend giữ một `MessageList` duy nhất mỗi channel; `useLoadOlderMessages` (`hooks/useThreads.ts:78`) nối trang
  cũ vào cuối danh sách, không bao giờ bỏ bớt.
- Thread: `store.GetThread` trả gốc + **toàn bộ** reply, không phân trang.

**Realtime**
- Gửi tin → `Hub.BroadcastToChannel` publish Redis `channel:<id>`; mỗi instance `PSubscribe("channel:*", "user:*", …)`
  (`grpc/hub_broadcast.go:108`) rồi lọc client cục bộ.
- Client nhận `chatMessage` → chèn vào cache, rồi **invalidate `unreadCounts`** (`stores/websocket.store.ts:595`).
- Mất sequence realtime → `workspaceResyncKeys` invalidate mọi channel, mọi danh sách tin nhắn.

**Đếm chưa đọc** (`store.GetUnreadCounts`, `internal/store/reactions_pins_receipts.go:250`)
- Trước đó `readableChannelIDs`: liệt kê mọi channel của workspace + DM, rồi batch check NGAC.
- Một câu SQL lớn: `COUNT(m.id)` mọi tin chính sau `last_read_at` của từng channel; đếm reply chưa đọc; đếm mention.
- CTE `followed`, nhánh thứ hai: `messages WHERE parent_message_id IS NULL AND reply_count>0 AND (sender_id=$1 OR
  mentions @> ARRAY[$1])` — **không giới hạn theo channel hay tenant**.

**Schema** (`data/init.sql:100` và các migration 004, 039, 040, 050, 051)
- Một bảng `messages` chung cho mọi tenant, PK `TEXT`, không có cột tenant/workspace, không partition.
- Index: `(channel_id)`, `(created_at)`, `(channel_id, created_at DESC)`, `latest_main`/`latest_reply` partial
  `(channel_id, created_at DESC, id DESC)`, GIN `search_vector`, GIN `mentions`. **Không có index `sender_id`.**

## Điểm nghẽn, xếp theo thứ tự sẽ vỡ trước

1. **Bão đếm chưa đọc.** *(Đính chính khi lập plan: client chỉ subscribe channel đang mở, nên N là số người đang mở
   channel; badge của channel khác hiện không cập nhật realtime. Xem `plans/261106-2127-message-unread-seq-at-scale/plan.md`.)*
   Một tin vào channel có N người online → N client cùng refetch `unreadCounts` → N lần
   (liệt kê channel + batch NGAC + SQL đếm). Channel toàn công ty 5.000 người: một tin = 5.000 truy vấn nặng.
   Đây là chi phí O(thành viên × tốc độ tin nhắn), tăng theo cấp số nhân với tenant lớn.
2. **Truy vấn xuyên tenant trong đếm chưa đọc.** Nhánh `sender_id = $1 OR mentions @> …` không có điều kiện channel,
   và không có index `sender_id` → chi phí tăng theo **tổng tin nhắn của cả nền tảng**, không phải của tenant.
   Một tenant mới ít dữ liệu vẫn chậm khi nền tảng lớn. (Suy luận từ schema; cần `EXPLAIN ANALYZE` trên dữ liệu lớn.)
3. **Đếm bằng `COUNT(*)`.** Số chưa đọc tỉ lệ với số dòng chưa đọc. Người mới vào channel chưa từng đọc
   (`last_read_at` NULL) → đếm toàn bộ lịch sử channel, mỗi lần có tin mới.
4. **Tìm kiếm trên GIN toàn cục.** `search_vector @@ … AND channel_id = ANY(...)`: GIN trả mọi tin khớp từ khoá của
   mọi tenant rồi mới lọc channel. Từ khoá phổ biến trên hàng tỉ dòng sẽ rất chậm.
5. **Pub/sub phát toàn cục.** Mọi instance nhận mọi tin nhắn của mọi tenant qua `channel:*`. Ổn với vài instance;
   khi scale ngang, băng thông Redis = instance × tổng tốc độ tin.
6. **Resync và reconnect đồng loạt.** Deploy hoặc Redis chập chờn → hàng nghìn client reconnect, mỗi client
   invalidate mọi khoá chat → thundering herd vào DB.
7. **Lỗi nhỏ của phân trang.**
   - Cursor chỉ có `created_at`, không có `id` phân xử: hai tin cùng micro giây ở ranh giới trang thì một tin bị
     bỏ qua.
   - Điều kiện `(deleted_at IS NULL OR reply_count > 0)` không khớp vị từ của partial index `latest_main`, nên
     planner dùng `(channel_id, created_at)` rồi lọc reply. Channel nhiều reply thì phải quét thừa (cần EXPLAIN xác nhận).
   - `GetThread` không phân trang. `PinnedMessageIDsIn` tải mọi ghim của channel cho mỗi trang.
   - Frontend nối trang vô hạn vào một mảng, không ảo hoá hay bỏ trang (chưa kiểm phần render).

## Các hướng

### A. Sửa tại chỗ trên Postgres: số thứ tự theo channel + đẩy delta (khuyến nghị)

- Thêm `messages.seq` (tăng dần theo channel, cấp từ `channels.last_seq` trong cùng transaction insert),
  `read_receipts.last_read_seq`, `messages.workspace_id`.
- Số chưa đọc chính = `channels.last_seq − last_read_seq` (trừ điều chỉnh tin bị xoá nếu cần chính xác tuyệt đối),
  O(1) mỗi channel. Mention và reply chưa đọc giữ bảng đếm riêng, cập nhật lúc ghi.
- Server đẩy `unread delta` qua websocket theo người nhận (đã có kênh `user:<id>`), client cộng dồn; bỏ invalidate
  sau mỗi tin. Refetch đầy đủ chỉ khi mở app hoặc resync.
- Cursor phân trang theo `(channel_id, seq)`; index `(channel_id, seq DESC) WHERE parent_message_id IS NULL`.
  Phân trang thread theo seq.
- Sửa CTE `followed` để giới hạn theo channel đọc được.
- **Giả định quan trọng nhất:** một Postgres primary chịu được tổng lượng ghi và kích thước bảng.
  **Vỡ trước tiên khi:** bảng `messages` lên nhiều TB hoặc tốc độ ghi vượt một primary (thường ở mức hàng chục
  nghìn tin/giây kéo dài), hoặc vacuum/index rebuild không theo kịp.

### B. A + partition và shard theo tenant (Postgres partitioning → Citus)

- `messages` partition theo `workspace_id` (hash) hoặc theo thời gian; về sau chuyển Citus, distribution key
  `workspace_id`. Tìm kiếm tách ra OpenSearch/Meilisearch, đồng bộ qua Redpanda (đã có).
- **Giả định:** mỗi tenant vừa một shard và truy vấn hầu như không xuyên tenant (đúng với mô hình hiện tại:
  `readableChannelIDs` đã giới hạn theo workspace).
  **Vỡ trước tiên khi:** một tenant khổng lồ thành hot shard. Ngoài ra chi phí vận hành Citus trên hạ tầng một VPS
  là không thực tế.

### C. Kho tin nhắn chuyên dụng (ScyllaDB/Cassandra, khoá `(channel_id, bucket)` + `seq`)

- Mô hình kiểu các app chat lớn: ghi tuyến tính, đọc theo partition channel, gần như không giới hạn.
- **Giả định:** đội vận hành được thêm một hệ phân tán, và chấp nhận mất join/transaction với Postgres.
  **Vỡ trước tiên khi:** cần nhất quán giữa tin nhắn và dữ liệu Postgres (thread, ghim, task, poll, xoá theo
  channel). Viết lại lớn, rủi ro cao, chi phí bỏ dở đắt nhất.

## Trade-offs

- A rẻ nhất, xử lý điểm nghẽn 1–3 và 7. Đây là những điểm vỡ đầu tiên, và không phụ thuộc vào việc dữ liệu nằm ở
  đâu. Cột `seq` và `workspace_id` cũng chính là thứ B cần, nên A không bị bỏ đi khi lên B.
- B chỉ đáng làm khi số liệu chứng minh một primary không đủ. Làm sớm thì tốn vận hành mà chưa có lợi.
- C chỉ nên làm khi B cũng hết đường. Với quy mô hiện tại (một VPS), đây là over-engineering.

## Better approaches

Đề bài ngầm là "tải tin nhắn" (đọc lịch sử). Bằng chứng cho thấy thứ vỡ trước **không phải đọc lịch sử** — trang
50 tin có index và cursor, chi phí cố định. Thứ vỡ trước là **đếm chưa đọc và realtime fan-out**: mỗi tin gây ra
O(thành viên) truy vấn tổng hợp, có nhánh quét xuyên tenant. Ưu tiên sửa phần này trước phân trang/lưu trữ:
chi phí chuyển hướng gần như bằng 0 vì chưa có gì được làm.

## Khuyến nghị

Đi hướng A, làm theo thứ tự:

1. Đếm chưa đọc bằng seq + đẩy delta, bỏ invalidate mỗi tin. Sửa nhánh `followed` xuyên tenant.
2. Cursor `(seq)`, phân trang thread, sửa index cho khớp vị từ.
3. Thêm `workspace_id` vào `messages`. Pub/sub chỉ subscribe channel có client cục bộ (hoặc Redis sharded pub/sub).
   Giãn reconnect bằng jitter.
4. Trước bước 1: dựng bộ seed dữ liệu lớn + `EXPLAIN ANALYZE` + test tải, để có số "trước" và xác nhận các suy luận
   về planner ở trên.

Thay đổi đụng schema và proto (event unread delta) → cần migration mới, `make proto` + `npm run proto:gen`, và
cập nhật spec messaging trong `docs/specs/`. Không đổi mô hình NGAC (vẫn check trên OA của channel).

Handoff: `ak:plan` với contract này → `ak:cook --tdd`.

## Câu hỏi mở

1. Quy mô mục tiêu cụ thể: số tenant, số người dùng của tenant lớn nhất, channel lớn nhất, số tin/giây lúc cao
   điểm, thời gian lưu tin nhắn. Các con số này quyết định ngưỡng chuyển từ A sang B.
2. Số chưa đọc có phải chính xác tuyệt đối khi có tin bị xoá không, hay chấp nhận lệch nhỏ (kiểu "99+")?
3. Có yêu cầu dữ liệu từng tenant nằm tách biệt (data residency / tenant riêng DB) không? Nếu có thì nghiêng sang B sớm hơn.
