# Cấu Trúc Tổ Chức

## 1. Giới thiệu

File này mô tả cách hệ thống tổ chức người dùng thành các nhóm: workspace, phòng ban, vai trò. Đây là nền tảng cho hệ thống quyền — bạn thuộc nhóm nào sẽ quyết định bạn được làm gì.

## 2. Các thành phần chính

### Workspace (Không gian làm việc)

Workspace là đơn vị tổ chức lớn nhất — tương đương một "công ty" hoặc "tổ chức". Mỗi workspace có:

- **Chủ sở hữu (Owner)** — người tạo workspace, có toàn quyền
- **Thành viên (Members)** — người được mời vào, có quyền cơ bản
- **Phòng ban** — các nhóm nhỏ hơn bên trong workspace
- **Kênh chat** — nơi các thành viên trao đổi
- **Drive** — nơi lưu trữ file chung

### Phòng ban (Department)

Phòng ban là cách chia nhỏ workspace thành các nhóm. Mỗi phòng ban có:

- **Trưởng phòng (Chief)** — có quyền quản lý phòng ban, duyệt yêu cầu
- **Thành viên phòng ban** — có quyền đọc/ghi tài liệu của phòng
- **Tài liệu riêng** — chỉ thành viên phòng mới truy cập được

Phòng ban có thể lồng nhau: Phòng Kế Toán có thể nằm trong Khối Tài Chính. Khi đó, quyền của nhóm cha sẽ bao trùm nhóm con.

### Vai trò trong hệ thống quyền

Hệ thống không có khái niệm "role" truyền thống (admin, user, viewer). Thay vào đó, quyền được xác định bởi **vị trí của bạn trong cây tổ chức**:

- Bạn là Owner workspace → có toàn quyền trên workspace đó
- Bạn là Member → có quyền đọc/ghi cơ bản
- Bạn là Chief phòng ban → có quyền quản lý phòng ban
- Bạn là thành viên phòng → có quyền truy cập tài liệu phòng

## 3. Luồng hoạt động

### Tạo workspace mới

Khi người dùng tạo workspace:

1. Hệ thống tạo một "Policy Class" (PC) — đây là node gốc của toàn bộ cây quyền cho workspace
2. Hệ thống tạo 3 nhóm tài nguyên (OA):
   - Quản lý (Mgmt) — cho các thao tác quản trị
   - Tài liệu (Documents) — cho file và thư mục
   - Kênh chat (Channels) — cho các kênh tin nhắn
3. Hệ thống tạo 2 nhóm người dùng (UA):
   - Owners — nhóm chủ sở hữu
   - Members — nhóm thành viên
4. Hệ thống thiết lập quyền:
   - Owners được toàn quyền trên cả 3 nhóm tài nguyên
   - Members được quyền đọc/ghi trên Tài liệu và Kênh chat
5. Người tạo được gán vào cả 2 nhóm (vừa Owner vừa Member)
6. Drive tự động tạo thư mục gốc cho workspace
7. Messaging tự động tạo kênh #general

### Tạo phòng ban

Khi admin tạo phòng ban trong workspace:

1. Hệ thống tạo nhóm "Phòng ban" (UA) — đây là nơi chứa thành viên
2. Hệ thống tạo nhóm "Trưởng phòng" (UA) — nằm bên trong nhóm Phòng ban
3. Hệ thống tạo nhóm tài liệu riêng cho phòng (OA)
4. Thiết lập quyền:
   - Trưởng phòng có toàn quyền trên tài liệu phòng
   - Thành viên phòng có quyền đọc/ghi
5. Nếu phòng ban có parent (ví dụ: Kế Toán thuộc Khối Tài Chính), nhóm phòng ban sẽ được gán vào nhóm parent → kế thừa quyền

### Thêm thành viên vào workspace

1. Admin chọn user cần mời
2. Node quyền của user được gán vào nhóm Members của workspace
3. User ngay lập tức có thể truy cập các kênh chat và tài liệu chung
4. Nếu cần, admin tiếp tục gán user vào phòng ban cụ thể

### Thêm thành viên vào phòng ban

1. Admin chọn user và phòng ban
2. Node quyền của user được gán vào nhóm phòng ban
3. User ngay lập tức có thể truy cập tài liệu riêng của phòng
4. Nếu user được chỉ định làm trưởng phòng, node được gán vào nhóm Chief → có thêm quyền quản lý

## 4. Ví dụ thực tế

**Công ty ABC** tạo workspace với cấu trúc:

```
Workspace "Công ty ABC"
├── Phòng Kế Toán
│   ├── Trưởng phòng: Nguyễn Văn A
│   └── Thành viên: Trần Thị B, Lê Văn C
├── Phòng Nhân Sự
│   ├── Trưởng phòng: Phạm Thị D
│   └── Thành viên: Hoàng Văn E
└── Phòng IT
    ├── Trưởng phòng: Vũ Văn F
    └── Thành viên: Đỗ Thị G
```

Với cấu trúc này:

- **Nguyễn Văn A** (trưởng Kế Toán) có thể: đọc/sửa/xóa tài liệu phòng Kế Toán, quản lý thành viên phòng. Nhưng KHÔNG thể xem tài liệu phòng Nhân Sự hay IT.
- **Trần Thị B** (nhân viên Kế Toán) có thể: đọc/ghi tài liệu phòng Kế Toán. Nhưng KHÔNG có quyền quản lý hay xóa.
- **Owner workspace** có toàn quyền trên tất cả phòng ban.

## 5. Điều cần nhớ

- Workspace là ranh giới quyền lớn nhất — dữ liệu workspace A hoàn toàn tách biệt với workspace B
- Phòng ban tạo ra quyền truy cập chi tiết hơn bên trong workspace
- Quyền được kế thừa theo cây: nếu bạn thuộc nhóm Owners, bạn có quyền trên mọi tài nguyên trong workspace
- Không cần cấu hình quyền thủ công — hệ thống tự xây dựng cây quyền khi tạo workspace/phòng ban
