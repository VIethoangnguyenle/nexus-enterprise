# Data Flow Trace — Send Message

## Mô tả

User gửi tin nhắn trong kênh chat. Hệ thống kiểm tra quyền NGAC (write trên channel OA), lưu tin nhắn, và broadcast qua WebSocket.

---

## Step 1 — API

**Endpoint:** `POST /api/channels/:chId/messages`

**File:** `services/messaging/internal/rest/handler.go`
**Function:** `Handler.SendMessage()` (line 167)

**Input body:**
```json
{
  "content": "Xin chào mọi người",
  "content_format": "markdown",
  "mentions": ["user-id-1"],
  "parent_message_id": "",
  "linked_entity_type": "",
  "linked_entity_id": ""
}
```

**Middleware:** `httputil.JWTMiddleware` → extract JWT claims (UserID, NGACNodeID)

**Handler logic:**
- Parse body → tạo `domain.SendMessageInput{ChannelID, SenderID, SenderNodeID, Content, ...}`
- Gọi `svc.SendMessage(ctx, input)`
- Sau khi thành công → `hub.BroadcastToChannel(channelID, msg)` (WebSocket)

---

## Step 2 — Service (Domain)

**File:** `services/messaging/internal/domain/service.go`
**Function:** `Service.SendMessage()` (line 306)

### 2a. Load channel
- Gọi `store.GetChannel(ctx, channelID)`

```sql
SELECT c.id, c.name, c.channel_type, COALESCE(c.workspace_id,''),
       COALESCE(c.ngac_oa_id,''), COALESCE(c.ngac_ua_id,''),
       COALESCE(c.created_by,''), c.created_at,
       COALESCE((SELECT COUNT(*)::int FROM channel_members cm WHERE cm.channel_id = c.id), 0)
FROM channels c WHERE c.id = $1
```

- Lấy `ch.NGACOaID` → OA node của channel content

### 2b. NGAC check — write access (CRITICAL)

**File:** `services/messaging/internal/domain/service.go`
**Function:** `Service.checkAccess()` (line 488)

```go
s.policyRead.CheckAccess(ctx, &policypb.CheckAccessRequest{
    UserNodeId:   in.SenderNodeID,
    ObjectNodeId: ch.NGACOaID,   // channel content OA
    Operation:    "write",       // ngac.OpWrite
})
```

**Chỉ kiểm tra khi** `msgType == "user"` (line 317-321). System messages bypass NGAC.

**Policy service flow:**
- `CheckAccess(userNodeID, channelOAID, "write")`
- BFS từ user → tìm tất cả UA → tìm Association UA→OA có "write"
- Nếu user thuộc `ch_members_{channelID}` (UA) VÀ có Association `ch_members → ch_content [read, write]` → **ALLOW**
- Nếu `resp.Decision == ngac.DecisionDeny` → return `ErrAccessDenied`

### 2c. Build message
- Tạo `store.Message{ID: uuid.New(), ChannelID, SenderID, Content, CreatedAt: now}`

### 2d. Insert message
- Gọi `store.InsertMessage(ctx, msg)`

### 2e. Thread handling (nếu là reply)
- Nếu `in.ParentMessageID != ""`:
  - `store.IncrementReplyCount(ctx, parentID)` — UPDATE parent
  - `store.TrackThreadParticipant(ctx, parentID, senderID)` — INSERT participant

### 2f. Lookup sender name
- Gọi Auth service: `authClient.GetUserByID()` → lấy username cho response

---

## Step 3 — Repository (Store)

**File:** `services/messaging/internal/store/store.go`

### 3a. `GetChannel()` (line 70)

```sql
SELECT c.id, c.name, c.channel_type, COALESCE(c.workspace_id,''),
       COALESCE(c.ngac_oa_id,''), COALESCE(c.ngac_ua_id,''),
       COALESCE(c.created_by,''), c.created_at,
       COALESCE((SELECT COUNT(*)::int FROM channel_members cm WHERE cm.channel_id = c.id), 0)
FROM channels c WHERE c.id = $1
```

**Bảng:** `channels` — SELECT (read only)

### 3b. `InsertMessage()` (line 177)

```sql
INSERT INTO messages (id, channel_id, sender_id, content, message_type,
  parent_message_id, linked_entity_type, linked_entity_id,
  content_format, mentions, created_at)
VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9, $10, $11)
```

**Bảng:** `messages` — INSERT
| Field | Giá trị |
|---|---|
| id | UUID mới |
| channel_id | channelID từ URL |
| sender_id | claims.UserID |
| content | "Xin chào mọi người" |
| message_type | "user" |
| content_format | "markdown" |
| mentions | `{user-id-1}` |
| parent_message_id | NULL (hoặc parent ID nếu reply) |
| created_at | NOW() |

### 3c. `IncrementReplyCount()` (line 197) — chỉ khi reply

```sql
UPDATE messages SET reply_count = reply_count + 1 WHERE id = $1
```

**Bảng:** `messages` — UPDATE (parent message)
| Field | Trước | Sau |
|---|---|---|
| reply_count | N | N+1 |

### 3d. `TrackThreadParticipant()` (line 203) — chỉ khi reply

```sql
INSERT INTO thread_participants (message_id, user_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING
```

**Bảng:** `thread_participants` — INSERT
| Field | Giá trị |
|---|---|
| message_id | parent message ID |
| user_id | sender ID |

---

## Step 4 — WebSocket Broadcast

**File:** `services/messaging/internal/rest/handler.go` (line 200)
**File:** `services/messaging/internal/grpc/hub.go`

```go
h.hub.BroadcastToChannel(channelID, msg)
```

- Hub tìm tất cả WebSocket connections subscribed to `channelID`
- Gửi protobuf message tới mỗi connection
- Nếu multi-instance: publish qua Redis pub/sub → hub khác nhận và broadcast

---

## Step 5 — Frontend Update

- WebSocket nhận `MESSAGE_NEW` event
- TanStack Query cache update: prepend message vào danh sách
- UI scroll xuống tin nhắn mới

---

## Database Impact Tổng Kết

| Bảng | Thao tác | Điều kiện |
|---|---|---|
| `channels` | SELECT | Luôn luôn (đọc channel info + NGAC OA ID) |
| `messages` | INSERT | Luôn luôn |
| `messages` | UPDATE reply_count | Chỉ khi reply (parent_message_id != "") |
| `thread_participants` | INSERT | Chỉ khi reply |

**Không thay đổi:** `users`, `ngac_nodes`, `ngac_assignments`, `channel_members`

---

## NGAC Check

| Điểm kiểm tra | File | Function (line) | Logic |
|---|---|---|---|
| Write access | `service.go:318` | `checkAccess()` | `CheckAccess(senderNodeID, ch.NGACOaID, "write")` |
| Bypass | `service.go:317` | `SendMessage()` | Skip nếu `msgType != "user"` (system messages) |
| Graph traversal | `ngac/access.go` | `Graph.CheckAccess()` | BFS: user → ch_members UA → Association → ch_content OA [write] |
