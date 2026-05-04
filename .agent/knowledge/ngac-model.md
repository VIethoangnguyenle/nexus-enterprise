# Mô Hình NGAC — Hệ Thống Kiểm Soát Quyền

## 1. Giới thiệu

NGAC (Next Generation Access Control) là "bộ não" kiểm soát quyền truy cập của toàn hệ thống. Thay vì dùng cách truyền thống "admin/user/viewer", NGAC dùng một đồ thị (graph) để mô tả: ai thuộc nhóm nào, nhóm nào được phép truy cập tài nguyên nào, và với quyền gì.

Hãy tưởng tượng NGAC như một sơ đồ tổ chức mở rộng — không chỉ mô tả "ai báo cáo cho ai" mà còn mô tả "ai được truy cập gì".

## 2. Permission Graph — Bức Tranh Toàn Cảnh

```
User (Người dùng)
├── belongs to → Department (Phòng ban)
│                 └── has → Role (Vai trò: Member, Chief)
│                            └── has → Permission (Quyền)
│                                       └── apply to → Resource (Tài nguyên)
└── context → Organization (Workspace)
               └── scope → Policy Class (Phạm vi chính sách)
```

**Ví dụ cụ thể:**

```
User A (Nguyễn Văn A)
├── belongs to → Department: Kế Toán
│                 └── has → Role: Trưởng phòng (Chief)
│                            └── has → Permission: read, write, delete, admin, manage
│                                       └── apply to → Resource: Tài liệu phòng Kế Toán
├── belongs to → Workspace Members
│                 └── has → Permission: read, write, create, upload
│                            └── apply to → Resource: Tài liệu chung, Kênh chat
└── context → Workspace "Công ty ABC"
               └── scope → PC_CongTyABC
```

→ User A có thể:
- ✅ Quản lý tài liệu phòng Kế Toán (vì là Chief)
- ✅ Đọc/ghi tài liệu chung (vì là Member workspace)
- ❌ Xem tài liệu phòng Nhân Sự (không thuộc phòng đó)

## 3. Các thành phần chính

### 5 loại node trong đồ thị

**User (U)** — Đại diện cho một người dùng cụ thể. Mỗi người khi đăng ký được tạo 1 node U.

**User Attribute (UA)** — Đại diện cho một nhóm người hoặc vai trò. Ví dụ:
- "Kế Toán_Dept" = tất cả nhân viên phòng Kế Toán
- "Kế Toán_Chief" = trưởng phòng Kế Toán
- "Workspace_Members" = tất cả thành viên workspace
- "ch_members_general" = thành viên kênh #general

**Object Attribute (OA)** — Đại diện cho một nhóm tài nguyên. Ví dụ:
- "Kế Toán_Mgmt" = tài liệu quản lý phòng KT
- "ch_content_general" = nội dung kênh #general
- "Documents" = thư mục tài liệu workspace

**Object (O)** — Đại diện cho một tài nguyên cụ thể. Ví dụ: một file trong drive.

**Policy Class (PC)** — Phạm vi chính sách — node gốc cao nhất. Mỗi workspace có 1 PC. Để quyền có hiệu lực, cả phía người dùng VÀ tài nguyên đều phải nằm trong CÙNG một PC.

### Cách quyền được thiết lập

**Assignment (gán)** — Nối node vào nhóm cha:
- User A → gán vào → "Kế Toán_Dept" (A thuộc phòng KT)
- "Tài liệu KT" → gán vào → PC workspace (tài liệu thuộc workspace)

**Association (liên kết quyền)** — Nối nhóm người với nhóm tài nguyên kèm quyền:
- "Kế Toán_Dept" → "Tài liệu KT" [đọc, ghi] (phòng KT được đọc/ghi tài liệu KT)
- "Kế Toán_Chief" → "Tài liệu KT" [toàn quyền] (trưởng phòng toàn quyền)

### Các loại quyền

| Quyền | Ý nghĩa | Ai có |
|---|---|---|
| read | Đọc/xem | Tất cả thành viên |
| write | Sửa/ghi | Tất cả thành viên |
| create | Tạo mới | Thành viên workspace/phòng |
| upload | Upload file | Thành viên workspace/phòng |
| delete | Xóa | Chỉ Owner/Chief |
| admin | Quản trị | Chỉ Owner/Chief |
| manage | Cấu hình | Chỉ Owner/Chief |
| approve | Phê duyệt | Người có quyền approve trên scope |

## 4. Cách hệ thống kiểm tra quyền

Khi hệ thống cần kiểm tra "User A có quyền đọc tài liệu X không?":

### Bước 1: Đi ngược từ User A lên trên

Tìm tất cả nhóm (UA) mà A thuộc về, và ghi nhận các Policy Class (PC) đạt được.

```
A → Kế Toán Dept (UA) → PC_Workspace_ABC ✓
A → Workspace Members (UA) → PC_Workspace_ABC ✓
```

### Bước 2: Đi ngược từ Tài liệu X lên trên

Tìm tất cả nhóm tài nguyên (OA) và PC.

```
Tài liệu X → Tài liệu KT (OA) → PC_Workspace_ABC ✓
```

### Bước 3: Tìm PC chung

Cả A và X đều đạt tới PC_Workspace_ABC → ĐẠT

### Bước 4: Kiểm tra quyền cụ thể

Tìm association kết nối nhóm của A với nhóm của X có quyền "read":
→ "Kế Toán Dept" → "Tài liệu KT" [read, write] → CÓ quyền read → **CHO PHÉP**

### Bước 5: Ràng buộc bổ sung

Sau khi đồ thị cho phép, kiểm tra ràng buộc động. Hiện tại: không cho sửa/upload vào cuối tuần.

## 5. Permission Graph trong thực tế

Đây là đồ thị quyền đầy đủ cho workspace "Công ty ABC" với phòng Kế Toán:

```
PC_CongTyABC (Phạm vi workspace)
│
├── [Nhóm người]
│   ├── Owners (UA) ← user_owner
│   │   → Mgmt (OA) [toàn quyền]
│   │   → Documents (OA) [toàn quyền]
│   │   → Channels (OA) [toàn quyền]
│   │
│   ├── Members (UA) ← user_A, user_B, user_C
│   │   → Documents (OA) [read, write, create, upload]
│   │   → Channels (OA) [read, write, create, upload]
│   │
│   ├── KeToan_Dept (UA) ← user_A, user_B
│   │   ├── KeToan_Chief (UA) ← user_A
│   │   │   → KeToan_Mgmt (OA) [toàn quyền]
│   │   └── → KeToan_Mgmt (OA) [read, write, create, upload]
│   │
│   └── ch_members_general (UA) ← user_A, user_B, user_C
│       → ch_content_general (OA) [read, write]
│
└── [Nhóm tài nguyên]
    ├── Mgmt (OA)
    ├── Documents (OA)
    ├── Channels (OA)
    ├── KeToan_Mgmt (OA)
    └── ch_content_general (OA)
```

Từ graph này:
- **A** (Chief KT): toàn quyền tài liệu KT, đọc/ghi tài liệu chung, chat kênh #general
- **B** (nhân viên KT): đọc/ghi tài liệu KT, đọc/ghi tài liệu chung, chat kênh #general
- **C** (phòng khác): đọc/ghi tài liệu chung, chat kênh #general, KHÔNG thấy tài liệu KT

## 6. Tại sao dùng NGAC?

RBAC truyền thống: bạn là "admin" hoặc "user" → quyền cố định theo label.

NGAC linh hoạt hơn:
- Quyền dựa trên **vị trí trong tổ chức**, không phải label cố định
- Kiểm soát quyền ở mức **từng file, từng kênh chat, từng phòng ban**
- Kế thừa quyền tự động — thêm phòng ban mới thì quyền tự có
- Thêm ràng buộc động (giờ làm việc, IP...) mà không sửa code

## 7. Điều cần nhớ

- NGAC là đồ thị, không phải danh sách → quyền được XÁC ĐỊNH bởi cấu trúc graph
- Muốn cấp quyền → tạo assignment (gán node vào nhóm)
- Muốn tạo quyền mới → tạo association (liên kết nhóm người ↔ nhóm tài nguyên)
- Muốn thu hồi quyền → xóa assignment
- Policy Class (PC) là "rào chắn" — quyền chỉ có hiệu lực khi cả 2 bên cùng thuộc 1 PC
- Graph được load vào bộ nhớ khi khởi động → kiểm tra quyền rất nhanh (không query DB)
