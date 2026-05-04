# Luồng Chat (Messaging)

## 1. Giới thiệu

File này mô tả cách hệ thống nhắn tin hoạt động: kênh chat, tin nhắn riêng (DM), thread trả lời, và thông báo. Messaging là tính năng được sử dụng nhiều nhất và cũng là nơi kết nối WebSocket realtime hoạt động.

## 2. Các thành phần chính

### Kênh chat (Channel)

Kênh chat là nơi một nhóm người trao đổi. Có 3 loại:

- **Workspace channel** — Kênh thuộc workspace, tất cả thành viên workspace có thể thấy (ví dụ: #general, #engineering)
- **Private channel** — Kênh riêng, chỉ người được mời mới thấy
- **DM (Direct Message)** — Tin nhắn riêng giữa 2 người

Mỗi kênh chat có 2 node quyền trong NGAC:
- **Content OA** — đại diện cho nội dung kênh (tin nhắn, file đính kèm)
- **Members UA** — đại diện cho nhóm thành viên kênh

Khi kiểm tra "user A có quyền gửi tin nhắn trong kênh X không?", hệ thống kiểm tra xem node quyền của A có đường đi tới Content OA của kênh X hay không.

### Tin nhắn (Message)

Mỗi tin nhắn chứa: nội dung (hỗ trợ markdown), người gửi, thời gian, và có thể có:
- **Thread** — trả lời một tin nhắn cụ thể (giống Slack thread)
- **Reaction** — emoji phản ứng (mỗi user chỉ được 1 reaction/emoji)
- **Mention** — tag người dùng
- **Linked entity** — liên kết tới tài sản hoặc yêu cầu phê duyệt
- **Pin** — ghim tin nhắn quan trọng

### Thông báo (Notification)

Hệ thống tự động tạo thông báo khi:
- Tài sản thay đổi trạng thái (ví dụ: laptop được phê duyệt)
- Yêu cầu phê duyệt được duyệt/từ chối
- Có người assign tài sản cho bạn
- Yêu cầu mua hàng được gửi

## 3. Luồng hoạt động

### Gửi tin nhắn

Khi user gửi tin nhắn trong kênh:

1. User nhập nội dung và nhấn gửi
2. Frontend gửi request tới Messaging service
3. Server tìm kênh trong database → lấy Content OA ID
4. Server hỏi Policy service: "User A có quyền 'write' trên Content OA của kênh X không?"
5. Nếu KHÔNG có quyền → trả về lỗi "access denied"
6. Nếu CÓ quyền → lưu tin nhắn vào database
7. Server tìm tên hiển thị của người gửi (hỏi Auth service)
8. Server gửi tin nhắn qua WebSocket tới tất cả client đang subscribe kênh X
9. Nếu có Redis → publish qua Redis pub/sub để các server instance khác cũng nhận được

### Xem danh sách kênh

Khi user mở workspace và cần thấy danh sách kênh:

1. Server lấy tất cả kênh thuộc workspace từ database
2. Với MỖI kênh, server kiểm tra: "User A có quyền 'read' trên kênh này không?"
3. Chỉ trả về các kênh mà user có quyền đọc

### Tạo kênh mới

1. User nhập tên kênh
2. Server tạo 2 node NGAC: Content OA và Members UA
3. Content OA được gán vào nhóm "Channels" của workspace → kế thừa quyền
4. Members UA được gán vào workspace PC
5. Tạo liên kết quyền: Members UA → Content OA với quyền [read, write]
6. Gán node quyền của người tạo vào Members UA → người tạo tự động là thành viên
7. Drive service tạo thư mục riêng cho kênh (để chia sẻ file trong kênh)

### Tin nhắn riêng (DM)

1. User A muốn nhắn tin riêng cho User B
2. Server kiểm tra: đã có DM channel giữa A và B chưa? (tìm trong bảng channel_members)
3. Nếu đã có → trả về kênh cũ
4. Nếu chưa có → tạo kênh DM mới:
   - Tạo Content OA và Members UA (giống kênh thường)
   - Nhưng KHÔNG gán vào workspace → gán vào PC_Global
   - Gán cả A và B vào Members UA
5. Từ giờ, A và B có thể nhắn tin riêng qua kênh này

### Thread (trả lời tin nhắn)

1. User nhấn "Reply" trên một tin nhắn
2. Tin nhắn trả lời được lưu với tham chiếu tới tin nhắn gốc (parent_message_id)
3. Đếm reply tự động tăng trên tin nhắn gốc
4. User được thêm vào danh sách "thread participants"
5. Server gửi sự kiện "thread_reply" qua WebSocket

## 4. Ví dụ thực tế

**Tình huống**: Trần Thị B (phòng Kế Toán) gửi tin nhắn trong kênh #ke-toan

1. B nhập: "Báo cáo Q1 đã hoàn thành, mọi người review giúp nhé!"
2. Frontend gửi request với: channel_id = "ke-toan-channel", sender = B
3. Server kiểm tra: B có node quyền → thuộc nhóm Members Kế Toán → Members UA liên kết tới Content OA kênh → có quyền "write" → **CHO PHÉP**
4. Tin nhắn được lưu và gửi qua WebSocket
5. Nguyễn Văn A (trưởng phòng) và Lê Văn C (nhân viên cùng phòng) nhận được tin nhắn ngay lập tức
6. Hoàng Văn E (phòng Nhân Sự) KHÔNG thấy tin nhắn này vì không thuộc kênh #ke-toan

## 5. Điều cần nhớ

- Quyền chat = quyền NGAC. Không có logic "if admin then allow" — tất cả đều qua graph traversal
- DM channel không thuộc workspace nào, được gán vào PC_Global
- WebSocket dùng protobuf binary (không phải JSON) để tiết kiệm băng thông
- Redis pub/sub cho phép nhiều server instance cùng phục vụ WebSocket — tin nhắn gửi ở server A sẽ được chuyển tới client ở server B
