# Luồng Realtime — WebSocket và Event

## 1. Giới thiệu

File này mô tả cách hệ thống gửi dữ liệu tức thì tới trình duyệt người dùng: tin nhắn mới, thông báo, trạng thái online/offline, cập nhật phê duyệt. Có 2 cơ chế realtime: WebSocket (tới client) và Kafka event (giữa các dịch vụ backend).

## 2. Các thành phần chính

### WebSocket Hub

Hub là "trung tâm điều phối" WebSocket, sống trong Messaging service. Hub quản lý:

- **Danh sách kênh** — Mỗi kênh chat có danh sách client đang "lắng nghe". Khi có tin nhắn mới, Hub gửi cho tất cả client trong kênh đó.
- **Danh sách user** — Mỗi user có danh sách kết nối (một user có thể mở nhiều tab). Dùng để gửi thông báo, sự kiện cá nhân.

### Giao thức WebSocket

Client và server giao tiếp qua protobuf binary (không phải JSON text). Có 2 loại "phong bì":

**ClientEnvelope** — Client gửi lên server:
- Xác thực (gửi JWT token)
- Subscribe kênh (bắt đầu nhận tin nhắn kênh X)
- Unsubscribe kênh (ngừng nhận)
- Đang gõ (typing indicator)

**ServerEnvelope** — Server gửi xuống client:
- Tin nhắn mới
- Trả lời thread
- Thông báo (notification)
- Sự kiện phê duyệt
- Sự kiện tài sản
- Trạng thái online/offline (presence)
- Số thông báo chưa đọc
- Lỗi

### Redis Pub/Sub

Khi hệ thống chạy nhiều server instance (để chịu tải), Redis giúp đồng bộ WebSocket giữa các server:

- Server A nhận tin nhắn mới → publish lên Redis channel "channel:{id}"
- Server B nhận từ Redis → gửi tới client đang kết nối ở server B

3 loại Redis channel:
- `channel:{channelID}` — Tin nhắn, typing trong kênh chat
- `user:{userID}` — Thông báo, sự kiện cá nhân
- `presence` — Online/offline broadcast cho tất cả user

### Kafka Event Bus

Các dịch vụ backend giao tiếp bất đồng bộ qua Kafka (Redpanda):

- **Asset service** phát sự kiện khi tài sản thay đổi trạng thái, có yêu cầu mới, hoặc được assign
- **Approval service** phát sự kiện khi yêu cầu được tạo, duyệt, hoặc từ chối
- **Messaging service** nhận tất cả sự kiện → tạo thông báo → gửi qua WebSocket

## 3. Luồng hoạt động

### Tin nhắn realtime

Khi user A gửi tin nhắn trong kênh #general:

1. Frontend gửi REST request tới Messaging service: "Gửi tin nhắn"
2. Server kiểm tra quyền NGAC, lưu tin nhắn
3. Server gọi Hub.BroadcastToChannel("general-channel-id", message)
4. Nếu có Redis:
   - Hub publish tin nhắn lên Redis channel "channel:general-channel-id"
   - Tất cả server instance nhận từ Redis
   - Mỗi server gửi tới client đang subscribe kênh đó
5. Nếu không có Redis (dev mode):
   - Hub gửi trực tiếp tới client đang subscribe trên cùng server

### Typing indicator

Khi user đang gõ:

1. Client gửi ClientEnvelope{typing: {channelId}} qua WebSocket
2. Hub broadcast tới tất cả client KHÁC trong kênh (không gửi lại cho người đang gõ)
3. Client nhận → hiển thị "A đang gõ..."

### Thông báo từ sự kiện Kafka

Khi tài sản được phê duyệt:

1. Approval service duyệt yêu cầu → publish ApprovalEvent lên topic "approval.events"
2. Messaging consumer nhận sự kiện
3. Consumer tạo notification trong database: "Yêu cầu XYZ đã được duyệt"
4. Consumer gọi Hub.SendNotification(userID, notification)
5. Hub gửi notification qua WebSocket tới tất cả tab của user đó
6. Consumer gọi Hub.BroadcastApprovalEvent(...) 
7. Hub broadcast sự kiện tới TẤT CẢ user → frontend invalidate cache → UI tự cập nhật

### Online/Offline (Presence)

Khi user kết nối WebSocket thành công:

1. Hub đăng ký client vào danh sách users[userID]
2. Hub broadcast PresenceEvent{userId, username, "online"} cho tất cả
3. Nếu có Redis → publish lên channel "presence" → tất cả server instance đều broadcast

Khi user ngắt kết nối:

1. Hub kiểm tra: user này còn kết nối nào khác không? (có thể mở nhiều tab)
2. Nếu đây là kết nối cuối cùng → broadcast PresenceEvent{..., "offline"}
3. Hub xóa client khỏi tất cả channel subscriptions

### Thread reply

Khi user trả lời trong thread:

1. Server lưu tin nhắn với parent_message_id
2. Server tăng reply_count trên tin nhắn gốc
3. Server thêm user vào danh sách thread participants
4. Hub.BroadcastThreadReply() gửi sự kiện ThreadReplyEvent cho kênh
5. Client nhận → cập nhật UI thread

## 4. Ví dụ thực tế

**Tình huống**: Lê Văn C tạo yêu cầu mua máy in → Nguyễn Văn A duyệt

1. C tạo yêu cầu trên UI → Approval service lưu vào DB
2. Approval service publish event: `{request_id: "req-001", action: "created", ...}` lên topic "approval.events"
3. Messaging consumer nhận event → tạo notification cho C: "Yêu cầu mua máy in đã được gửi"
4. Hub.SendNotification(C.userID, notif) → C nhận thông báo trên UI ngay lập tức
5. A thấy yêu cầu trong tab "Chờ duyệt" → nhấn "Duyệt"
6. Approval service cập nhật trạng thái → publish event: `{action: "approved", actor: A, ...}`
7. Messaging consumer nhận → tạo notification cho C: "Yêu cầu đã được duyệt"
8. Hub.BroadcastApprovalEvent(...) → TẤT CẢ client nhận sự kiện → TanStack Query invalidate → UI tự refresh danh sách

## 5. Điều cần nhớ

- WebSocket dùng protobuf binary, không phải JSON — tiết kiệm bandwidth
- Client phải xác thực trong 5 giây đầu tiên, nếu không bị ngắt
- Redis pub/sub cho phép scale horizontal — thêm server mới không cần đổi code
- Kafka events là fire-and-forget — nếu tạo notification thất bại, không ảnh hưởng flow chính
- Approval events broadcast cho TẤT CẢ user (không chỉ người liên quan) vì UI cần cập nhật số lượng pending, trạng thái request...
- Một user có thể có nhiều kết nối WebSocket (nhiều tab) — Hub quản lý tất cả và chỉ broadcast offline khi kết nối cuối cùng bị đóng
