# [USE CASE] — User Đăng Ký (Onboarding)

## 1. Mô tả ngắn

Người dùng mới đăng ký tài khoản. Hệ thống tạo user, node quyền NGAC, workspace đầu tiên, và kênh chat mặc định.

## 2. Actor

- User mới (chưa có tài khoản)
- Không thuộc workspace hay department nào

## 3. Input

- `username` — Tên đăng nhập (duy nhất)
- `password` — Mật khẩu

## 4. Luồng xử lý

### Bước 1 — Frontend

- Gọi `POST /api/auth/signup` với body `{username, password}`

### Bước 2 — Auth Service

- Hash password bằng bcrypt
- Tạo user ID (UUID)
- Gọi Policy service: `CreateNode(name="user_{id}", type=U)` → nhận về NGAC node ID
- Gọi Policy service: `CreateAssignment(user_node → PublicUsers UA)` → user thuộc nhóm công khai
- Gọi Workspace service: `CreateDefaultWorkspace(userID)` → tạo workspace đầu tiên

### Bước 3 — Workspace Service (trong cùng flow)

- Tạo workspace record
- Gọi Policy service tạo cây NGAC cho workspace (PC, OA, UA, associations)
- Gọi Drive service: `CreateDriveForWorkspace()` → tạo thư mục gốc

### Bước 4 — Auth Service (tiếp tục)

- Gọi Messaging service: `CreateDefaultChannel(workspaceID, userNodeID)` → tạo kênh #general
- Tạo tenant record (liên kết user ↔ workspace)
- Tạo JWT token

### Bước 5 — Frontend update

- Nhận JWT → lưu vào localStorage
- Redirect tới workspace dashboard
- Thiết lập kết nối WebSocket

## 5. Database Impact

### Bảng: `users`
| Field | Giá trị |
|---|---|
| id | UUID mới |
| username | từ input |
| password | bcrypt hash |
| ngac_node | ID node NGAC vừa tạo |
| created_at | NOW() |

### Bảng: `ngac_nodes` (6+ records mới)
| Record | name | node_type |
|---|---|---|
| User node | `user_{id}` | U |
| Workspace PC | `PC_{wsID}` | PC |
| Owners UA | `{wsID}_Owners` | UA |
| Members UA | `{wsID}_Members` | UA |
| Mgmt OA | `{wsID}_Mgmt` | OA |
| Documents OA | `{wsID}_Documents` | OA |
| Channels OA | `{wsID}_Channels` | OA |

### Bảng: `ngac_assignments` (7+ records mới)
- user_node → PublicUsers
- user_node → Owners UA
- user_node → Members UA
- Owners UA → PC
- Members UA → PC
- Mgmt/Documents/Channels OA → PC

### Bảng: `ngac_associations` (5+ records mới)
- Owners → Mgmt [toàn quyền]
- Owners → Documents [toàn quyền]
- Owners → Channels [toàn quyền]
- Members → Documents [read, write, create, upload]
- Members → Channels [read, write, create, upload]

### Bảng: `workspaces`
| Field | Giá trị |
|---|---|
| id | UUID mới |
| name | "My Workspace" (mặc định) |
| owner_id | user.id |
| ngac_pc_id | PC node ID |

### Bảng: `channels`
| Field | Giá trị |
|---|---|
| id | UUID mới |
| name | "general" |
| channel_type | "workspace" |
| workspace_id | workspace.id |
| ngac_oa_id | Content OA ID |
| ngac_ua_id | Members UA ID |
| created_by | user.id |

### Bảng: `drive_items`
| Field | Giá trị |
|---|---|
| id | UUID mới |
| workspace_id | workspace.id |
| item_type | "folder" |
| name | "Root" |
| ngac_node_id | Documents OA ID |
| status | "active" |

## 6. NGAC Check

- Không có kiểm tra quyền — đây là thao tác tạo mới (public endpoint)
- Sau khi hoàn thành, user có quyền trên workspace qua cây NGAC vừa tạo

## 7. Ví dụ thực tế

Nguyễn Văn A đăng ký tài khoản "nguyen.van.a":

1. `users` → thêm record {id: "u-001", username: "nguyen.van.a", ngac_node: "n-001"}
2. `ngac_nodes` → thêm "user_u-001" (type U)
3. `ngac_assignments` → gán "user_u-001" → "PublicUsers"
4. `workspaces` → thêm {id: "ws-001", name: "My Workspace", owner: "u-001", ngac_pc: "pc-001"}
5. NGAC graph mở rộng: PC_ws-001 → Owners/Members/Mgmt/Documents/Channels
6. `channels` → thêm kênh #general cho ws-001
7. A nhận JWT: `{user_id: "u-001", ngac_node_id: "n-001", workspace_id: "ws-001"}`
