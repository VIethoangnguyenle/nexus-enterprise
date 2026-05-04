# Vòng Đời Người Dùng

## 1. Giới thiệu

File này mô tả toàn bộ hành trình của một người dùng trong hệ thống: từ lúc đăng ký tài khoản, đăng nhập, vào workspace làm việc, cho đến khi tương tác với các tính năng. Hiểu file này sẽ giúp bạn nắm được cách hệ thống "biết" người dùng là ai và cho phép họ làm gì.

## 2. Các thành phần chính

**Tài khoản (User)** — Mỗi người dùng có một tài khoản gồm: tên đăng nhập, mật khẩu (đã mã hóa), và một "node quyền" trong hệ thống NGAC. Node quyền này giống như "thẻ nhân viên" — nó xác định người dùng thuộc nhóm nào, được phép truy cập gì.

**Vé đăng nhập (JWT Token)** — Sau khi đăng nhập thành công, người dùng nhận được một "vé" chứa thông tin: ID người dùng, tên, ID node quyền, và workspace hiện tại. Vé này được gửi kèm mọi request để hệ thống biết "ai đang yêu cầu".

**Tenant (Liên kết người dùng - workspace)** — Mỗi người dùng được liên kết với một workspace mặc định. Đây là nơi họ sẽ vào đầu tiên sau khi đăng nhập.

## 3. Luồng hoạt động

### Đăng ký tài khoản

Khi một người mới đăng ký:

1. Người dùng nhập tên đăng nhập và mật khẩu trên giao diện
2. Hệ thống mã hóa mật khẩu (không ai đọc được mật khẩu gốc)
3. Hệ thống tạo tài khoản mới trong database
4. Hệ thống tạo "node quyền" cho người dùng trong NGAC — đây là bước quan trọng nhất, vì từ giờ mọi kiểm tra quyền đều dựa trên node này
5. Node quyền của người dùng được gán vào nhóm "PublicUsers" (tất cả user đều thuộc nhóm này)
6. Hệ thống tự động tạo workspace đầu tiên cho người dùng:
   - Workspace service xây dựng cấu trúc quyền (xem thêm file organization-structure.md)
   - Messaging service tạo kênh chat #general mặc định
   - Drive service chuẩn bị thư mục gốc
7. Hệ thống trả về vé đăng nhập (JWT) và người dùng được chuyển vào workspace

### Đăng nhập

Khi người dùng đã có tài khoản:

1. Người dùng nhập tên và mật khẩu
2. Hệ thống kiểm tra: tên đăng nhập có tồn tại không? Mật khẩu có đúng không?
3. Nếu đúng, hệ thống tìm workspace mặc định của người dùng
4. Trả về vé đăng nhập (JWT) chứa đầy đủ thông tin

### Kết nối WebSocket (nhận tin nhắn realtime)

Sau khi đăng nhập, trình duyệt tự động thiết lập kết nối WebSocket:

1. Trình duyệt mở kết nối WebSocket tới server
2. Trong vòng 5 giây, trình duyệt phải gửi vé JWT để xác thực
3. Server kiểm tra vé — nếu hợp lệ, đăng ký client vào danh sách "đang online"
4. Server thông báo cho tất cả user khác: "Nguyễn Văn A vừa online"
5. Từ giờ, mọi tin nhắn mới, thông báo, sự kiện đều được gửi tức thì qua kết nối này

Khi người dùng đóng trình duyệt hoặc mất kết nối:

1. Server phát hiện kết nối bị đóng
2. Nếu đây là kết nối cuối cùng của người dùng (họ có thể mở nhiều tab), server thông báo: "Nguyễn Văn A vừa offline"

## 4. Ví dụ thực tế

**Tình huống**: Nguyễn Văn A mới vào công ty, được admin tạo tài khoản.

1. Admin tạo tài khoản "nguyen.van.a" với mật khẩu tạm
2. Hệ thống tạo node quyền cho A — lúc này A chưa thuộc nhóm nào đặc biệt, chỉ có quyền cơ bản
3. A đăng nhập → được tạo workspace riêng
4. Admin mời A vào workspace công ty → A được gán vào nhóm Members của workspace
5. Admin thêm A vào phòng Kế Toán → node quyền của A được gán thêm vào nhóm "KeToan_Dept"
6. Giờ A có thể:
   - Đọc/gửi tin nhắn trong các kênh của workspace (vì là Member)
   - Truy cập tài liệu phòng Kế Toán (vì thuộc KeToan_Dept)
   - Nhưng KHÔNG thể xem tài liệu phòng Nhân Sự (vì không thuộc nhóm đó)

## 5. Điều cần nhớ

- Mỗi user có đúng một node quyền NGAC — node này là "chìa khóa" cho mọi kiểm tra quyền
- JWT token chứa cả user ID và NGAC node ID — hai thứ này luôn đi cùng nhau
- WebSocket auth phải hoàn thành trong 5 giây, nếu không sẽ bị ngắt kết nối
- Một user có thể mở nhiều tab (nhiều kết nối WebSocket cùng lúc), hệ thống quản lý tất cả
