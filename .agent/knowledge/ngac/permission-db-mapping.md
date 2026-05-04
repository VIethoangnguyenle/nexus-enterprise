# NGAC — Database Mapping (Mapping Đồ Thị → Cơ Sở Dữ Liệu)

## 1. Giới thiệu

File này map chính xác từng node và liên kết trong đồ thị quyền NGAC tới bảng và cột cụ thể trong database. Đọc file này để biết "dữ liệu quyền nằm ở đâu" và "cách truy vấn".

## 2. Bảng NGAC Core — 3 bảng nền tảng

### Bảng `ngac_nodes` — Tất cả node trong đồ thị

| Cột | Kiểu | Ý nghĩa |
|---|---|---|
| `id` | TEXT PK | ID duy nhất của node |
| `name` | TEXT | Tên node (theo convention: `user_{id}`, `PC_{wsID}`, `{deptID}_Dept`...) |
| `node_type` | TEXT | Loại: `U`, `UA`, `OA`, `O`, `PC` |
| `properties` | JSONB | Metadata tùy chỉnh (ví dụ: `{"scope": "global"}`) |

**Node type mapping:**

| Loại node | Ý nghĩa | Ví dụ name | Tạo bởi service nào |
|---|---|---|---|
| U | Người dùng | `user_abc123` | Auth |
| UA | Nhóm người / vai trò | `ws_Owners`, `dept_Chief`, `ch_members_x` | Workspace, Messaging |
| OA | Nhóm tài nguyên | `ws_Documents`, `dept_Mgmt`, `ch_content_x` | Workspace, Messaging |
| O | Tài nguyên cụ thể | tên file | Drive |
| PC | Policy Class | `PC_ws123`, `PC_Global` | Workspace, seed data |

### Bảng `ngac_assignments` — Liên kết cha-con (cạnh đồ thị)

| Cột | Kiểu | Ý nghĩa |
|---|---|---|
| `id` | TEXT PK | ID của assignment |
| `child_id` | TEXT FK → ngac_nodes | Node con |
| `parent_id` | TEXT FK → ngac_nodes | Node cha |
| UNIQUE | (child_id, parent_id) | Không cho phép gán trùng |

**Assignment mapping:**

| child_type → parent_type | Ý nghĩa | Ví dụ |
|---|---|---|
| U → UA | User thuộc nhóm | user_A → KeToan_Dept |
| UA → UA | Nhóm con thuộc nhóm cha | KeToan_Chief → KeToan_Dept |
| UA → PC | Nhóm người thuộc phạm vi | Members → PC_workspace |
| O → OA | File thuộc thư mục | File1 → DriveRoot |
| OA → OA | Nhóm con thuộc nhóm cha | ch_content → Channels |
| OA → PC | Nhóm tài nguyên thuộc phạm vi | Documents → PC_workspace |

### Bảng `ngac_associations` — Liên kết quyền

| Cột | Kiểu | Ý nghĩa |
|---|---|---|
| `id` | TEXT PK | ID của association |
| `ua_id` | TEXT FK → ngac_nodes | Nhóm người |
| `oa_id` | TEXT FK → ngac_nodes | Nhóm tài nguyên |
| `operations` | TEXT[] | Mảng quyền: `{read,write,create,...}` |
| UNIQUE | (ua_id, oa_id) | Mỗi cặp UA-OA chỉ có 1 association |

## 3. Bảng Business — Cầu nối sang NGAC

Mỗi bảng business lưu ID node NGAC tương ứng. Đây là cách "thế giới business" nối với "thế giới quyền":

### `users` → NGAC User node

| Cột liên quan | Nối tới | Ý nghĩa |
|---|---|---|
| `users.ngac_node` | `ngac_nodes.id` (type=U) | Node quyền của user |

### `workspaces` → NGAC Policy Class

| Cột liên quan | Nối tới | Ý nghĩa |
|---|---|---|
| `workspaces.ngac_pc_id` | `ngac_nodes.id` (type=PC) | Phạm vi quyền workspace |

### `channels` → NGAC OA + UA cặp

| Cột liên quan | Nối tới | Ý nghĩa |
|---|---|---|
| `channels.ngac_oa_id` | `ngac_nodes.id` (type=OA) | Nhóm nội dung kênh |
| `channels.ngac_ua_id` | `ngac_nodes.id` (type=UA) | Nhóm thành viên kênh |

### `departments` → NGAC UA

| Cột liên quan | Nối tới | Ý nghĩa |
|---|---|---|
| `departments.ngac_ua_id` | `ngac_nodes.id` (type=UA) | Nhóm thành viên phòng ban |

### `drive_items` → NGAC O hoặc OA

| Cột liên quan | Nối tới | Ý nghĩa |
|---|---|---|
| `drive_items.ngac_node_id` | `ngac_nodes.id` (type=O/OA) | Node quyền file/thư mục |

### `tenant_users` → NGAC User node (trong workspace)

| Cột liên quan | Nối tới | Ý nghĩa |
|---|---|---|
| `tenant_users.ngac_node_id` | `ngac_nodes.id` (type=U) | Node quyền user trong workspace |
| `tenant_users.department_id` | `departments.id` | Phòng ban user thuộc |

### `assets` → NGAC node

| Cột liên quan | Nối tới | Ý nghĩa |
|---|---|---|
| `assets.ngac_node` | `ngac_nodes.id` | Node quyền tài sản |

## 4. Bảng Approval — Schema-per-Tenant

**Quan trọng:** Bảng approval nằm trong schema riêng cho mỗi workspace (tenant). Schema name = `tenant_{8_chars_đầu_workspace_id}`.

### `{schema}.approval_requests`

| Cột liên quan | Nối tới | Ý nghĩa |
|---|---|---|
| `scope_oa_id` | `ngac_nodes.id` (type=OA) | Phạm vi phòng ban → xác định ai thấy yêu cầu |
| `department_id` | `departments.id` | Phòng ban tạo yêu cầu |
| `created_by` | NGAC node ID | Node quyền người tạo |

### `{schema}.approval_assignments`

| Cột liên quan | Nối tới | Ý nghĩa |
|---|---|---|
| `user_node_id` | `ngac_nodes.id` (type=U) | Node quyền người được phân công duyệt |
| `grant_source` | Text | Nguồn gốc quyền: "direct", "role:KeToan_Chief", "department:KeToan_Dept" |

## 5. Bảng Denormalized — Cache truy vấn nhanh

### `channel_members`

| Cột | Ý nghĩa |
|---|---|
| `channel_id` | FK → channels |
| `ngac_node_id` | NGAC node ID user → dùng lookup DM nhanh |

Đây là bản **copy** (denormalized) — quyền thật nằm ở `ngac_assignments`. Bảng này chỉ dùng để tìm DM channel giữa 2 user mà không cần query NGAC graph.

## 6. Ví dụ — Trace Dữ Liệu

Giả sử user_A (ID: "u-001") thuộc phòng Kế Toán (ID: "dept-kt") trong workspace "Công ty ABC" (ID: "ws-abc"):

```
users:             id="u-001", ngac_node="n-001"
tenant_users:      tenant_id="ws-abc", user_id="u-001", ngac_node_id="n-001", department_id="dept-kt"
departments:       id="dept-kt", ngac_ua_id="ua-kt-dept"
workspaces:        id="ws-abc", ngac_pc_id="pc-ws-abc"

ngac_nodes:        id="n-001",        name="user_u-001",     type=U
                   id="ua-kt-dept",   name="dept-kt_Dept",   type=UA
                   id="ua-kt-chief",  name="dept-kt_Chief",  type=UA
                   id="oa-kt-mgmt",   name="dept-kt_Mgmt",   type=OA
                   id="pc-ws-abc",    name="PC_ws-abc",       type=PC

ngac_assignments:  child="n-001"       → parent="ua-kt-chief"  (A là Chief KT)
                   child="ua-kt-chief" → parent="ua-kt-dept"   (Chief thuộc Dept)
                   child="ua-kt-dept"  → parent="pc-ws-abc"    (Dept thuộc workspace)
                   child="oa-kt-mgmt"  → parent="pc-ws-abc"    (Tài liệu KT thuộc workspace)

ngac_associations: ua="ua-kt-chief"  → oa="oa-kt-mgmt" ops=[read,write,create,delete,admin,upload,manage]
                   ua="ua-kt-dept"   → oa="oa-kt-mgmt" ops=[read,write,create,upload]
```

→ User A đi theo đường: `n-001 → ua-kt-chief → ua-kt-dept → pc-ws-abc`
→ Tài liệu KT: `oa-kt-mgmt → pc-ws-abc`
→ PC chung: `pc-ws-abc` ✓
→ Association: `ua-kt-chief → oa-kt-mgmt [full]` ✓

## 7. Điều cần nhớ

- 3 bảng NGAC core (`ngac_nodes`, `ngac_assignments`, `ngac_associations`) là source of truth cho quyền
- Bảng business chỉ lưu FK tới `ngac_nodes.id` — không lưu quyền trực tiếp
- Approval tables nằm trong tenant schema riêng, KHÔNG ở public schema
- `channel_members` là denormalized cache — quyền thật ở `ngac_assignments`
- Mọi kiểm tra quyền runtime đều dựa trên in-memory graph (không query DB) — DB chỉ dùng khi khởi động và khi thay đổi graph
