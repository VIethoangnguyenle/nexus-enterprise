# Tổng Quan Hệ Thống

## 1. Giới thiệu

NGAC Platform (tên thương hiệu: Zump Biz) là một nền tảng làm việc nhóm dành cho doanh nghiệp. Hãy tưởng tượng nó giống như Slack kết hợp với Google Drive và hệ thống phê duyệt — nhưng tất cả được bảo vệ bởi một hệ thống kiểm soát quyền truy cập rất chặt chẽ gọi là NGAC.

Người dùng đăng ký → được tạo workspace (không gian làm việc) → mời thành viên → chat, chia sẻ tài liệu, quản lý tài sản, tạo yêu cầu phê duyệt. Mọi thao tác đều được hệ thống kiểm tra quyền trước khi cho phép.

## 2. Các thành phần chính

Hệ thống được chia thành nhiều dịch vụ nhỏ (microservices), mỗi dịch vụ lo một nhiệm vụ riêng:

**Policy (Chính sách quyền)** — Đây là "bộ não" kiểm soát quyền truy cập. Mọi dịch vụ khác khi cần kiểm tra "user A có quyền làm X không?" đều hỏi dịch vụ này. Nó lưu trữ một đồ thị (graph) mô tả ai thuộc nhóm nào, nhóm nào được phép truy cập tài nguyên nào.

**Auth (Xác thực)** — Lo việc đăng ký, đăng nhập, và tạo "căn cước" cho người dùng. Khi bạn đăng nhập, dịch vụ này trả về một "vé" (JWT token) chứa thông tin của bạn để các dịch vụ khác biết bạn là ai.

**Workspace (Không gian làm việc)** — Quản lý workspace, phòng ban, và thành viên. Khi tạo workspace, dịch vụ này tự động xây dựng cấu trúc quyền cho workspace đó.

**Messaging (Nhắn tin)** — Quản lý kênh chat, tin nhắn, tin nhắn riêng (DM), thread trả lời, và thông báo. Dịch vụ này cũng quản lý kết nối WebSocket để gửi tin nhắn realtime.

**Drive (Lưu trữ file)** — Quản lý file và thư mục. Mỗi file/thư mục đều có node quyền riêng trong hệ thống NGAC, nghĩa là bạn có thể kiểm soát ai được xem file nào ở mức từng file.

**Document (Tài liệu)** — Lưu trữ nội dung tài liệu thực tế trên MinIO (một hệ thống lưu file giống Amazon S3).

**Asset (Quản lý tài sản)** — Quản lý tài sản doanh nghiệp (laptop, bàn ghế, thiết bị...) với hệ thống trạng thái (mới → đang dùng → bảo trì → thanh lý).

**Approval (Phê duyệt)** — Hệ thống quy trình phê duyệt nhiều bước. Ví dụ: nhân viên tạo yêu cầu mua hàng → trưởng phòng duyệt → giám đốc duyệt.

**Frontend** — Giao diện web sử dụng React, chạy trên Vite.

## 3. Cách các dịch vụ liên lạc với nhau

Các dịch vụ nói chuyện với nhau theo 3 cách:

**gRPC (đồng bộ)** — Khi một dịch vụ cần kết quả ngay lập tức. Ví dụ: Messaging hỏi Policy "user A có quyền đọc kênh X không?" và chờ câu trả lời.

**Kafka/Redpanda (bất đồng bộ)** — Khi một dịch vụ muốn thông báo sự kiện mà không cần chờ phản hồi. Ví dụ: Approval service phát sự kiện "yêu cầu vừa được duyệt" → Messaging service nhận và tạo thông báo.

**WebSocket (realtime tới client)** — Khi cần gửi dữ liệu tức thì tới trình duyệt của người dùng. Ví dụ: tin nhắn mới, thông báo, trạng thái online/offline.

## 4. Hạ tầng kỹ thuật

Hệ thống sử dụng:

- **PostgreSQL** — Cơ sở dữ liệu chính, tất cả dịch vụ dùng chung một database nhưng mỗi dịch vụ quản lý bảng riêng
- **Redis** — Bộ nhớ đệm và hệ thống pub/sub để scale WebSocket ra nhiều server
- **Redpanda** — Hệ thống event bus tương thích Kafka, dùng để truyền sự kiện giữa các dịch vụ
- **MinIO** — Lưu trữ file (tương thích Amazon S3)
- **Traefik** — Proxy đầu vào, điều hướng request tới đúng dịch vụ dựa trên URL

## 5. Ví dụ thực tế

Khi Nguyễn Văn A đăng ký tài khoản, đây là những gì xảy ra:

1. A nhập tên và mật khẩu trên giao diện web
2. Auth service tạo tài khoản, mã hóa mật khẩu, và tạo "node quyền" cho A trong hệ thống NGAC
3. Auth service tự động tạo workspace đầu tiên cho A (giống như Slack tự tạo workspace khi bạn đăng ký)
4. Workspace service xây dựng toàn bộ cấu trúc quyền cho workspace: ai là chủ, ai là thành viên, nhóm nào được truy cập gì
5. Messaging service tạo kênh #general mặc định
6. A nhận được "vé" (JWT) và được chuyển vào workspace của mình
7. Từ giờ, mọi thao tác của A đều được kiểm tra quyền qua hệ thống NGAC

## 6. Nguyên tắc quan trọng

- **Mọi truy cập đều qua NGAC** — Không có ngoại lệ. Dù là đọc tin nhắn hay upload file, hệ thống đều kiểm tra quyền
- **Dịch vụ độc lập** — Mỗi dịch vụ tự lo logic riêng, chỉ giao tiếp qua gRPC hoặc event
- **Event-driven** — Thay vì gọi trực tiếp, các dịch vụ phát sự kiện để các dịch vụ khác phản ứng (ví dụ: tài sản thay đổi trạng thái → tự động tạo thông báo)
