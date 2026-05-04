# Kiến Trúc Hệ Thống

## 1. Giới thiệu

File này mô tả cách các dịch vụ trong hệ thống được tổ chức, kết nối, và giao tiếp với nhau. Nếu overview.md trả lời "hệ thống có gì?", thì file này trả lời "các phần đó nối với nhau như thế nào?".

## 2. Các thành phần chính

### Cách các dịch vụ giao tiếp

Hệ thống có 3 cách giao tiếp:

**gRPC (hỏi-đáp trực tiếp)** — Khi một dịch vụ cần câu trả lời ngay. Ví dụ: Messaging hỏi Policy "user A có quyền gửi tin nhắn không?" và chờ "Có" hoặc "Không".

**Kafka/Redpanda (phát sự kiện)** — Khi một dịch vụ muốn thông báo "có chuyện xảy ra" mà không cần biết ai đang nghe. Ví dụ: Approval service phát "yêu cầu vừa được duyệt" → Messaging service tự động nghe và tạo thông báo.

**WebSocket (gửi tới trình duyệt)** — Khi cần gửi dữ liệu tức thì tới người dùng. Ví dụ: tin nhắn mới, thông báo, trạng thái online.

### Ai gọi ai?

Dưới đây là bản đồ "ai gọi ai" giữa các dịch vụ:

- **Auth** gọi → Policy (tạo node quyền cho user mới), Workspace (tạo workspace đầu tiên), Messaging (tạo kênh #general)
- **Workspace** gọi → Policy (xây cây quyền workspace/phòng ban), Drive (tạo thư mục gốc)
- **Messaging** gọi → Policy (kiểm tra quyền đọc/ghi kênh), Auth (tra cứu tên user), Drive (tạo thư mục cho kênh)
- **Drive** gọi → Policy (kiểm tra quyền file/thư mục, tạo node NGAC)
- **Asset** gọi → Policy (kiểm tra quyền quản lý tài sản)
- **Approval** gọi → Policy (kiểm tra quyền phê duyệt, tìm người có quyền)

Tất cả dịch vụ đều gọi Policy — đây là dịch vụ quan trọng nhất, nếu Policy chết thì toàn hệ thống không thể kiểm tra quyền.

### Cách request từ trình duyệt tới dịch vụ

Trình duyệt không gọi trực tiếp tới từng dịch vụ. Thay vào đó, tất cả request đi qua một "cổng" gọi là Traefik. Traefik nhìn vào URL để quyết định chuyển request tới dịch vụ nào:

- URL bắt đầu bằng `/api/auth` → chuyển tới Auth service
- URL bắt đầu bằng `/api/workspaces` → chuyển tới Workspace service
- URL bắt đầu bằng `/api/channels` hoặc `/api/messages` → chuyển tới Messaging service
- URL bắt đầu bằng `/api/ws` → chuyển tới WebSocket server (Messaging service, cổng khác)
- URL bắt đầu bằng `/api/drive` → chuyển tới Drive service
- URL bắt đầu bằng `/api/approval` → chuyển tới Approval service
- Còn lại → chuyển tới Frontend (trang web React)

### Sự kiện Kafka (ai phát, ai nghe)

Có 4 luồng sự kiện:

| Sự kiện | Ai phát | Ai nghe | Khi nào |
|---|---|---|---|
| Tài sản thay đổi trạng thái | Asset service | Messaging | Laptop chuyển từ "mới" sang "đang dùng" |
| Yêu cầu tài sản | Asset service | Messaging | Nhân viên gửi yêu cầu mua laptop |
| Gán/trả tài sản | Asset service | Messaging | Admin giao laptop cho nhân viên |
| Phê duyệt | Approval service | Messaging | Yêu cầu được duyệt/từ chối |

Messaging service là "người nghe" duy nhất — nó nhận sự kiện và tạo thông báo + gửi qua WebSocket.

## 3. Cơ sở dữ liệu

Tất cả dịch vụ dùng chung một database PostgreSQL, nhưng mỗi dịch vụ "sở hữu" các bảng riêng:

**Policy sở hữu:**
- `ngac_nodes` — Tất cả node quyền
- `ngac_assignments` — Liên kết cha-con giữa các node
- `ngac_associations` — Liên kết quyền (nhóm người → nhóm tài nguyên + quyền)

**Auth sở hữu:**
- `users` — Tài khoản người dùng

**Workspace sở hữu:**
- `workspaces` — Danh sách workspace

**Messaging sở hữu:**
- `channels` — Kênh chat
- `messages` — Tin nhắn
- `channel_members` — Thành viên kênh (bản cache, quyền thật nằm ở NGAC)
- `thread_participants` — Người tham gia thread
- `notifications` — Thông báo

**Asset sở hữu:**
- `asset_types` — Loại tài sản (laptop, bàn ghế...)
- `assets` — Tài sản cụ thể
- `asset_transitions` — Lịch sử thay đổi trạng thái
- `asset_requests` — Yêu cầu tài sản

**Drive sở hữu:**
- `drive_items` — File và thư mục
- `drive_shares` — Chia sẻ file
- `drive_quotas` — Giới hạn dung lượng workspace

**Document sở hữu:**
- `documents` — Thông tin tài liệu

Điều quan trọng: mỗi bảng business (users, channels, assets, drive_items) đều có cột lưu NGAC node ID — đây là "cầu nối" giữa dữ liệu business và hệ thống quyền.

## 4. Redis — Phân chia theo dịch vụ

Redis được chia thành các database riêng để tránh xung đột:

- DB 0: Policy service (cache quyền)
- DB 1: Auth service (session, rate limiting)
- DB 2: Messaging service (WebSocket pub/sub)
- DB 3: Asset service
- DB 4: Approval service

## 5. Dữ liệu khởi tạo

Khi hệ thống chạy lần đầu, database tự động tạo 3 node NGAC cơ bản:

- **PC_Global** — Policy Class mặc định, dùng cho DM (tin nhắn riêng không thuộc workspace nào)
- **PublicUsers** — Nhóm tất cả user, gán vào PC_Global
- **PublicDocs** — Nhóm tài liệu công khai, gán vào PC_Global
- Liên kết quyền: PublicUsers → PublicDocs [đọc]

Nghĩa là: mọi user mặc định có thể đọc tài liệu công khai.

## 6. Điều cần nhớ

- Policy service là trung tâm — mọi dịch vụ đều phụ thuộc vào nó
- Policy service có 2 phiên bản: Write (1 instance) và Read (2 instances) — tối ưu vì đọc nhiều hơn ghi
- Shared database nhưng separated ownership — mỗi dịch vụ chỉ đọc/ghi bảng của mình
- Mọi entity business đều có NGAC node ID — đây là thiết kế cốt lõi
