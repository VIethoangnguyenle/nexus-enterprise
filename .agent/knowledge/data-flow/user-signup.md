# Data Flow Trace — User Signup (Onboarding)

## Mô tả

User đăng ký tài khoản mới. Hệ thống tạo NGAC node, lưu user, tạo/join workspace, provision NGAC graph, và auto-tạo channel #general.

---

## Step 1 — API

**Endpoint:** `POST /api/auth/signup`

**File:** `services/auth/internal/rest/handler.go`
**Function:** `Handler.Signup()` (line 47)

**Input body:**
```json
{
  "email": "nguyen.a@company.vn",
  "password": "securepassword",
  "display_name": "Nguyễn Văn A",
  "tenant_name": "Company XYZ"
}
```

**Không có middleware auth** — endpoint public.

---

## Step 2 — Service (Domain)

**File:** `services/auth/internal/domain/service.go`
**Function:** `Service.Signup()` (line 142)

### 2a. Kiểm tra email tồn tại
- `store.GetUserByEmail(ctx, email)` → nếu có → `ErrUserExists`

### 2b. Hash password
- `auth.HashPassword(password)` → bcrypt hash

### 2c. Tạo NGAC user node

**Function:** `Service.createUserNGACNode()` (line ~330)

```go
s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
    Name:     username,
    NodeType: "U",   // User type
})
```

- Policy Write service tạo node trong `ngac_nodes`
- Trả về `ngacNodeID` — ID duy nhất của user trong NGAC graph

### 2d. Lưu user vào database
- `store.CreateUser(ctx, userID, username, hash, ngacNodeID, email, unionID, displayName, "")`

### 2e. Resolve tenant (3 cases)

**Function:** `Service.resolveOrCreateTenant()` (line 196)

| Case | Điều kiện | Kết quả |
|---|---|---|
| 1 | `tenantName != ""` | Tạo workspace mới |
| 2 | Email domain match tenant | Join workspace hiện có |
| 3 | Không match | Tạo workspace mới "{displayName}'s Workspace" |

### 2f. Tạo workspace (Case 1 & 3)

**Function:** `Service.createTenantForUser()` (line 224)

```go
s.wsClient.CreateWorkspace(ctx, &workspacepb.CreateWorkspaceRequest{
    Name: name, UserId: userID, UserNgacNodeId: ngacNodeID,
})
```

- Workspace service tạo workspace
- Tạo NGAC PC node cho workspace
- Tạo Owners UA, Members UA
- Trả về `ws.Id`, `ws.PcNodeId`, `ws.OwnersUaId`, `ws.MembersUaId`

### 2g. Init tenant NGAC

**Function:** `Service.initTenantNGAC()` (line ~337)

- Tạo các Association giữa Owners UA / Members UA và workspace OAs
- Gán quyền cơ bản cho members

### 2h. Join tenant

**Function:** `Service.joinTenant()` (line 249)

- `store.InsertTenantUser(ctx, tenantID, userID, role, "active", ngacNodeID)`
- `assignUserToTenantNGAC(ctx, tenantID, ngacNodeID, isOwner)`:
  - `CreateAssignment(user → Owners UA)` nếu owner
  - `CreateAssignment(user → Members UA)` luôn luôn

### 2i. Auto-provision channel #general

**Function:** `Service.autoProvisionChannel()` (line ~243)

- Tạo channel #general cho workspace
- Add user vào channel

### 2j. Generate JWT
- `auth.GenerateToken(userID, username, ngacNodeID, tenantID)`
- JWT chứa: UserID, Username, NGACNodeID, TenantID

---

## Step 3 — Repository (Store)

### 3a. Auth Store: `GetUserByEmail()` (line 73)

```sql
SELECT id, username, COALESCE(password,''), COALESCE(ngac_node,''),
       COALESCE(email,''), COALESCE(union_id,''), COALESCE(display_name,''),
       COALESCE(phone,'')
FROM users WHERE email = $1
```

### 3b. Policy Store: `CreateNode()` — INSERT ngac_nodes

```sql
INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $2, $3, $4)
```

**Bảng:** `ngac_nodes` — INSERT
| Field | Giá trị |
|---|---|
| id | UUID mới |
| name | "nguyen.a" |
| node_type | `U` |

### 3c. Auth Store: `CreateUser()` (line 57)

```sql
INSERT INTO users (id, username, password, ngac_node, email, union_id, display_name, phone)
VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7, NULLIF($8,''))
```

**Bảng:** `users` — INSERT
| Field | Giá trị |
|---|---|
| id | UUID mới |
| username | "nguyen.a" (derived from email) |
| password | bcrypt hash |
| ngac_node | NGAC node UUID |
| email | "nguyen.a@company.vn" |
| union_id | UUID mới (cross-tenant identity) |
| display_name | "Nguyễn Văn A" |
| phone | NULL |

### 3d. Workspace Store: `CreateWorkspace()` — qua gRPC

**Bảng:** `workspaces` — INSERT
| Field | Giá trị |
|---|---|
| id | UUID mới |
| name | "Company XYZ" |
| ngac_pc_id | PC node UUID |

**Bảng:** `ngac_nodes` — INSERT (3 nodes)
| node_type | Vai trò |
|---|---|
| PC | Workspace Policy Class |
| UA | Owners user attribute |
| UA | Members user attribute |

### 3e. Auth Store: `InsertTenantUser()` (line 121)

```sql
INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (tenant_id, user_id) DO NOTHING
```

**Bảng:** `tenant_users` — INSERT
| Field | Giá trị |
|---|---|
| tenant_id | workspace UUID |
| user_id | user UUID |
| role | "owner" |
| status | "active" |
| ngac_node_id | NGAC node UUID |

### 3f. Policy Store: NGAC assignments — INSERT (2-3 records)

```sql
INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3)
```

| child | parent | Nghĩa |
|---|---|---|
| User node | Owners UA | User là owner |
| User node | Members UA | User là member |
| Members UA | Workspace PC | Members thuộc workspace |

---

## Database Impact Tổng Kết

| Bảng | Thao tác | Số records |
|---|---|---|
| `users` | SELECT (check existing) | 0 |
| `ngac_nodes` | INSERT | 1 (user U node) |
| `users` | INSERT | 1 |
| `workspaces` | INSERT | 1 (nếu tạo mới) |
| `ngac_nodes` | INSERT | 3 (PC, Owners UA, Members UA) — qua workspace svc |
| `ngac_assignments` | INSERT | 2-3 |
| `ngac_associations` | INSERT | 2+ (owner/member permissions) |
| `tenant_users` | INSERT | 1 |
| `channels` | INSERT | 1 (#general — qua messaging svc) |
| `channel_members` | INSERT | 1 |

---

## NGAC Check

| Điểm kiểm tra | Logic |
|---|---|
| Signup | **Không có NGAC check** — endpoint public |
| Sau signup | User tự động có quyền qua assignment vào Owners/Members UA |
