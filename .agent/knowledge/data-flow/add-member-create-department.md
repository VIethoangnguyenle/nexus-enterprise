# Data Flow Trace — Add Channel Member

## Mô tả

User thêm một thành viên vào kênh chat. Hệ thống tạo NGAC assignment (gán user vào Members UA) và lưu denormalized record.

---

## Step 1 — API

**Endpoint:** `POST /api/channels/:chId/members`

**File:** `services/messaging/internal/rest/handler.go`
**Function:** `Handler.AddChannelMember()` (line 248)

**Input body:**
```json
{
  "ngac_node_id": "target-user-ngac-node-id"
}
```

**Handler logic:**
- Parse body → lấy `body.NGACNodeID`
- Gọi `svc.AddMember(ctx, channelID, claims.NGACNodeID, body.NGACNodeID)`

---

## Step 2 — Service (Domain)

**File:** `services/messaging/internal/domain/service.go`
**Function:** `Service.AddMember()` (line 426)

### 2a. Load channel
- Gọi `store.GetChannel(ctx, channelID)` → lấy `ch.NGACUaID` (Members UA)

### 2b. Create NGAC assignment (gRPC tới Policy service)

```go
s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
    ChildId:  targetNodeID,    // user node
    ParentId: ch.NGACUaID,     // channel Members UA
})
```

**Policy Write service:**
- File: `services/policy/internal/grpc/write_server.go`
- Store: `services/policy/internal/ngac/store.go`

### 2c. Insert denormalized member
- Gọi `store.InsertChannelMember(ctx, channelID, targetNodeID)`

---

## Step 3 — Repository (Store)

### 3a. Messaging Store: `GetChannel()` (line 70)

```sql
SELECT c.id, c.name, c.channel_type, COALESCE(c.workspace_id,''),
       COALESCE(c.ngac_oa_id,''), COALESCE(c.ngac_ua_id,''),
       COALESCE(c.created_by,''), c.created_at,
       COALESCE((SELECT COUNT(*)::int FROM channel_members cm WHERE cm.channel_id = c.id), 0)
FROM channels c WHERE c.id = $1
```

### 3b. Policy Store: `CreateAssignment()` — INSERT ngac_assignments

**File:** `services/policy/internal/ngac/store.go`

```sql
INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3)
```

**Bảng:** `ngac_assignments` — INSERT
| Field | Giá trị |
|---|---|
| id | UUID mới |
| child_id | target user NGAC node ID |
| parent_id | channel Members UA ID (`ch.NGACUaID`) |

**Hiệu ứng:** Từ lúc này, user có đường đi: `user → ch_members_X (UA) → Association → ch_content_X (OA) [read, write]`

### 3c. Messaging Store: `InsertChannelMember()` (line 125)

```sql
INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING
```

**Bảng:** `channel_members` — INSERT
| Field | Giá trị |
|---|---|
| channel_id | channelID |
| ngac_node_id | target user NGAC node ID |
| joined_at | NOW() (default) |

---

## Step 4 — Không có Event

AddMember không publish event. Không có Kafka message.

---

## Step 5 — Frontend Update

- Frontend re-fetch member list (nếu panel mở)
- Target user sẽ thấy kênh trong danh sách lần tải tiếp theo (do `filterAccessible` cho phép)

---

## Database Impact Tổng Kết

| Bảng | Thao tác | Service |
|---|---|---|
| `channels` | SELECT | Messaging |
| `ngac_assignments` | INSERT | Policy (gRPC) |
| `channel_members` | INSERT | Messaging |

**Không thay đổi:** `messages`, `users`, `ngac_nodes`, `ngac_associations`

---

## NGAC Check

| Điểm kiểm tra | Logic |
|---|---|
| AddMember | **Không có NGAC check** — mọi thành viên kênh có thể mời người khác |
| Sau khi thêm | User tự động có read/write qua assignment vào Members UA |

---

---

# Data Flow Trace — Remove Channel Member

## Mô tả

Xóa thành viên khỏi kênh chat. Hệ thống xóa NGAC assignment → thu hồi quyền ngay lập tức.

---

## Step 1 — API

**Endpoint:** `DELETE /api/channels/:chId/members/:nodeId`

**File:** `services/messaging/internal/rest/handler.go`
**Function:** `Handler.RemoveChannelMember()` (line 276)

---

## Step 2 — Service

**File:** `services/messaging/internal/domain/service.go`
**Function:** `Service.RemoveMember()` (line 442)

```go
s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
    ChildId:  targetNodeID,
    ParentId: ch.NGACUaID,
})
```

---

## Step 3 — Repository

### Policy Store: DELETE ngac_assignments

```sql
DELETE FROM ngac_assignments WHERE child_id = $1 AND parent_id = $2
```

**Bảng:** `ngac_assignments` — DELETE
| Điều kiện |
|---|
| child_id = target user node |
| parent_id = channel Members UA |

**Hiệu ứng:** User mất đường đi → `CheckAccess` sẽ trả `DENY` ngay lập tức.

---

## Database Impact

| Bảng | Thao tác | Service |
|---|---|---|
| `channels` | SELECT | Messaging |
| `ngac_assignments` | DELETE | Policy (gRPC) |

**Lưu ý:** `channel_members` KHÔNG được xóa (denormalized record tồn tại). Quyền thật ở `ngac_assignments`.

---

---

# Data Flow Trace — Create Department

## Mô tả

Admin tạo phòng ban mới. Hệ thống xây NGAC sub-tree: Dept UA, Chief UA, Mgmt OA + assignments + associations.

---

## Step 1 — API

**Endpoint:** `POST /api/workspaces/:id/departments`

**File:** `services/workspace/internal/rest/admin_handler.go`

---

## Step 2 — Service

**File:** `services/workspace/internal/domain/department.go`
**Function:** `Service.CreateDepartment()` (line tùy theo version)

### 2a. Tạo NGAC nodes (3 nodes)

```go
// Policy Write gRPC calls:
CreateNode({Name: deptID+"_Dept",  Type: "UA"})  // Nhóm thành viên
CreateNode({Name: deptID+"_Chief", Type: "UA"})  // Nhóm trưởng phòng
CreateNode({Name: deptID+"_Mgmt",  Type: "OA"})  // Nhóm tài liệu
```

### 2b. Tạo NGAC assignments (3-4 assignments)

```go
// Chief UA → Dept UA (chief cũng là thành viên)
CreateAssignment({ChildId: chiefUAID, ParentId: deptUAID})

// Dept UA → workspace PC hoặc parent Dept UA
CreateAssignment({ChildId: deptUAID, ParentId: parentNodeID})

// Mgmt OA → workspace PC
CreateAssignment({ChildId: mgmtOAID, ParentId: workspacePCID})
```

### 2c. Tạo NGAC associations (2 associations)

```go
// Chief toàn quyền trên tài liệu phòng
CreateAssociation({UaId: chiefUAID, OaId: mgmtOAID,
    Operations: [read,write,create,delete,admin,upload,manage]})

// Member quyền cơ bản
CreateAssociation({UaId: deptUAID, OaId: mgmtOAID,
    Operations: [read,write,create,upload]})
```

### 2d. Lưu department record

---

## Step 3 — Repository

### Policy Store (gRPC → `services/policy/internal/ngac/store.go`)

**INSERT `ngac_nodes`** (3 records):

```sql
INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $2, $3, $4)
```

| id | name | node_type |
|---|---|---|
| UUID-1 | `{deptID}_Dept` | UA |
| UUID-2 | `{deptID}_Chief` | UA |
| UUID-3 | `{deptID}_Mgmt` | OA |

**INSERT `ngac_assignments`** (3-4 records):

```sql
INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3)
```

| child | parent | Nghĩa |
|---|---|---|
| Chief UA | Dept UA | Chief thuộc phòng |
| Dept UA | workspace PC / parent Dept UA | Phòng thuộc workspace/phòng cha |
| Mgmt OA | workspace PC | Tài liệu thuộc workspace |

**INSERT `ngac_associations`** (2 records):

```sql
INSERT INTO ngac_associations (id, ua_id, oa_id, operations) VALUES ($1, $2, $3, $4)
```

| ua_id | oa_id | operations |
|---|---|---|
| Chief UA | Mgmt OA | `{read,write,create,delete,admin,upload,manage}` |
| Dept UA | Mgmt OA | `{read,write,create,upload}` |

### Workspace Store (`services/workspace/internal/store/department_store.go`)

```sql
INSERT INTO departments (id, workspace_id, name, parent_id, ngac_ua_id, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
```

**Bảng:** `departments` — INSERT
| Field | Giá trị |
|---|---|
| id | UUID mới |
| workspace_id | workspace.id |
| name | "Kế Toán" |
| parent_id | NULL hoặc parent dept ID |
| ngac_ua_id | Dept UA ID |
| sort_order | 0 (default) |

---

## Database Impact Tổng Kết

| Bảng | Thao tác | Số records |
|---|---|---|
| `ngac_nodes` | INSERT | 3 (Dept UA, Chief UA, Mgmt OA) |
| `ngac_assignments` | INSERT | 3-4 |
| `ngac_associations` | INSERT | 2 |
| `departments` | INSERT | 1 |

---

## NGAC Check

| Điểm kiểm tra | Logic |
|---|---|
| Tạo phòng ban | Middleware hoặc domain check quyền admin trên workspace |
| Sau khi tạo | Phòng ban sẵn sàng — gán user vào Dept UA = user có quyền |
