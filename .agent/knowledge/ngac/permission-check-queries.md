# NGAC — SQL Queries Kiểm Tra Quyền

## 1. Giới thiệu

File này chứa các SQL query thực tế để kiểm tra và debug quyền truy cập trong hệ thống NGAC. Mỗi query dựa trên schema thật, dùng đúng tên bảng và cột. Bạn có thể chạy trực tiếp trên PostgreSQL.

**Lưu ý quan trọng:** Trong runtime, hệ thống KHÔNG query DB để kiểm tra quyền — nó dùng in-memory graph. Các query dưới đây dùng để **debug, verify, và audit** quyền.

---

## 2. Query Cơ Bản — Xem Thông Tin Node

### Tìm NGAC node của một user

```sql
-- Tìm node quyền của user theo username
SELECT u.id AS user_id, u.username, u.ngac_node, n.name AS ngac_name, n.node_type
FROM users u
JOIN ngac_nodes n ON u.ngac_node = n.id
WHERE u.username = 'nguyen.van.a';
```

### Xem tất cả nhóm (UA) mà user thuộc về

```sql
-- Tìm tất cả UA mà user được gán trực tiếp
SELECT n_parent.id, n_parent.name, n_parent.node_type
FROM ngac_assignments a
JOIN ngac_nodes n_parent ON a.parent_id = n_parent.id
WHERE a.child_id = (SELECT ngac_node FROM users WHERE username = 'nguyen.van.a')
  AND n_parent.node_type = 'UA';
```

### Xem tất cả node trong một workspace

```sql
-- Liệt kê tất cả node thuộc workspace (đi từ PC xuống)
WITH RECURSIVE tree AS (
    SELECT id, name, node_type, 0 AS depth
    FROM ngac_nodes
    WHERE id = (SELECT ngac_pc_id FROM workspaces WHERE name = 'Công ty ABC')
    
    UNION ALL
    
    SELECT n.id, n.name, n.node_type, t.depth + 1
    FROM tree t
    JOIN ngac_assignments a ON a.parent_id = t.id
    JOIN ngac_nodes n ON a.child_id = n.id
    WHERE t.depth < 10
)
SELECT REPEAT('  ', depth) || name AS tree_view, node_type, id
FROM tree
ORDER BY depth, node_type, name;
```

---

## 3. Case 1 — User Có Quyền Approve Request Không?

**Bối cảnh:** Kiểm tra xem user A có quyền duyệt yêu cầu trong phạm vi phòng Kế Toán.

```sql
-- Bước 1: Tìm NGAC node của user
-- Bước 2: Tìm scope OA của phòng ban (từ bảng departments hoặc approval_requests)
-- Bước 3: Kiểm tra có Association UA→OA với operation 'approve' không

WITH user_uas AS (
    -- Tìm tất cả UA mà user thuộc về (đệ quy lên trên)
    WITH RECURSIVE ancestors AS (
        SELECT parent_id AS ua_id
        FROM ngac_assignments
        WHERE child_id = (SELECT ngac_node FROM users WHERE username = 'nguyen.van.a')
        
        UNION
        
        SELECT a.parent_id
        FROM ancestors anc
        JOIN ngac_assignments a ON a.child_id = anc.ua_id
        JOIN ngac_nodes n ON a.parent_id = n.id
        WHERE n.node_type IN ('UA', 'PC')
    )
    SELECT ua_id FROM ancestors
)
SELECT 
    assoc.ua_id,
    ua_node.name AS role_name,
    assoc.oa_id,
    oa_node.name AS scope_name,
    assoc.operations
FROM ngac_associations assoc
JOIN ngac_nodes ua_node ON assoc.ua_id = ua_node.id
JOIN ngac_nodes oa_node ON assoc.oa_id = oa_node.id
WHERE assoc.ua_id IN (SELECT ua_id FROM user_uas)
  AND 'approve' = ANY(assoc.operations);
```

**Giải thích:**
- Query đệ quy tìm tất cả UA mà user thuộc về (trực tiếp và gián tiếp)
- Sau đó tìm Association có operation "approve"
- Nếu có kết quả → user có quyền approve. Nếu rỗng → không có quyền.

### Kiểm tra approval assignment cụ thể

```sql
-- Xem ai được phân công duyệt cho request cụ thể
-- (approval tables nằm trong tenant schema)
SELECT 
    aa.user_node_id,
    n.name AS approver_name,
    aa.step_order,
    aa.grant_source,
    aa.status,
    aa.acted_at,
    aa.comment
FROM tenant_XXXXXXXX.approval_assignments aa
JOIN ngac_nodes n ON aa.user_node_id = n.id
WHERE aa.request_id = 'REQUEST_UUID_HERE'
ORDER BY aa.step_order, aa.status;
```

---

## 4. Case 2 — User Có Trong Channel Không? (Send Message)

**Bối cảnh:** Kiểm tra xem user B có quyền gửi tin nhắn trong kênh #ke-toan.

```sql
-- Cách 1: Kiểm tra qua NGAC assignments (source of truth)
SELECT EXISTS (
    SELECT 1
    FROM ngac_assignments a
    WHERE a.child_id = (SELECT ngac_node FROM users WHERE username = 'tran.thi.b')
      AND a.parent_id = (SELECT ngac_ua_id FROM channels WHERE name = 'ke-toan')
) AS is_member;
```

```sql
-- Cách 2: Kiểm tra qua denormalized table (nhanh hơn nhưng có thể stale)
SELECT EXISTS (
    SELECT 1
    FROM channel_members cm
    WHERE cm.channel_id = (SELECT id FROM channels WHERE name = 'ke-toan')
      AND cm.ngac_node_id = (SELECT ngac_node FROM users WHERE username = 'tran.thi.b')
) AS is_member;
```

```sql
-- Kiểm tra quyền write cụ thể (Association)
SELECT assoc.operations
FROM ngac_associations assoc
WHERE assoc.ua_id = (SELECT ngac_ua_id FROM channels WHERE name = 'ke-toan')
  AND assoc.oa_id = (SELECT ngac_oa_id FROM channels WHERE name = 'ke-toan');

-- Nếu kết quả chứa 'write' VÀ user là member → user có quyền gửi tin nhắn
```

**Giải thích:**
- Cách 1 kiểm tra trực tiếp trong NGAC graph — đây là source of truth
- Cách 2 dùng bảng cache `channel_members` — nhanh hơn cho DM lookup
- Cần kiểm tra CẢ membership (assignment) VÀ quyền (association) để kết luận

---

## 5. Case 3 — User Có Quyền Add Member Không?

**Bối cảnh:** Kiểm tra xem user A có quyền thêm thành viên vào workspace.

```sql
-- Kiểm tra user có quyền 'admin' hoặc 'manage' trên workspace
WITH user_uas AS (
    WITH RECURSIVE ancestors AS (
        SELECT parent_id AS ua_id
        FROM ngac_assignments
        WHERE child_id = (SELECT ngac_node FROM users WHERE username = 'nguyen.van.a')
        
        UNION
        
        SELECT a.parent_id
        FROM ancestors anc
        JOIN ngac_assignments a ON a.child_id = anc.ua_id
        JOIN ngac_nodes n ON a.parent_id = n.id
        WHERE n.node_type IN ('UA', 'PC')
    )
    SELECT ua_id FROM ancestors
),
workspace_oas AS (
    -- Tìm OA quản lý của workspace
    SELECT n.id AS oa_id, n.name
    FROM ngac_nodes n
    WHERE n.name LIKE '%_Mgmt' 
      AND n.node_type = 'OA'
      AND n.id IN (
          SELECT child_id FROM ngac_assignments 
          WHERE parent_id = (SELECT ngac_pc_id FROM workspaces WHERE name = 'Công ty ABC')
      )
)
SELECT 
    ua_node.name AS role_name,
    oa_node.name AS resource_name,
    assoc.operations
FROM ngac_associations assoc
JOIN ngac_nodes ua_node ON assoc.ua_id = ua_node.id
JOIN ngac_nodes oa_node ON assoc.oa_id = oa_node.id
WHERE assoc.ua_id IN (SELECT ua_id FROM user_uas)
  AND assoc.oa_id IN (SELECT oa_id FROM workspace_oas)
  AND ('admin' = ANY(assoc.operations) OR 'manage' = ANY(assoc.operations));

-- Có kết quả → user có quyền quản lý → có thể add member
```

```sql
-- Hoặc đơn giản hơn: kiểm tra user có phải Owner không
SELECT EXISTS (
    SELECT 1
    FROM ngac_assignments a
    JOIN ngac_nodes n ON a.parent_id = n.id
    WHERE a.child_id = (SELECT ngac_node FROM users WHERE username = 'nguyen.van.a')
      AND n.name LIKE '%_Owners'
      AND n.node_type = 'UA'
) AS is_owner;
```

---

## 6. Case 4 — User Có Quyền Quản Lý Department Không?

**Bối cảnh:** Kiểm tra xem user có quyền quản lý (admin) phòng Kế Toán.

```sql
-- Kiểm tra user có phải Chief của phòng ban không
SELECT EXISTS (
    SELECT 1
    FROM ngac_assignments a
    WHERE a.child_id = (SELECT ngac_node FROM users WHERE username = 'nguyen.van.a')
      AND a.parent_id IN (
          -- Tìm Chief UA của phòng Kế Toán
          SELECT n.id
          FROM ngac_nodes n
          WHERE n.name LIKE '%_Chief'
            AND n.node_type = 'UA'
            AND n.id IN (
                SELECT child_id FROM ngac_assignments
                WHERE parent_id = (SELECT ngac_ua_id FROM departments WHERE name = 'Kế Toán')
            )
      )
) AS is_department_chief;
```

```sql
-- Xem tất cả quyền của user trên tài liệu phòng Kế Toán
WITH user_uas AS (
    WITH RECURSIVE ancestors AS (
        SELECT parent_id AS ua_id
        FROM ngac_assignments
        WHERE child_id = (SELECT ngac_node FROM users WHERE username = 'nguyen.van.a')
        
        UNION
        
        SELECT a.parent_id
        FROM ancestors anc
        JOIN ngac_assignments a ON a.child_id = anc.ua_id
        JOIN ngac_nodes n ON a.parent_id = n.id
        WHERE n.node_type IN ('UA', 'PC')
    )
    SELECT ua_id FROM ancestors
)
SELECT 
    ua_node.name AS via_role,
    assoc.operations
FROM ngac_associations assoc
JOIN ngac_nodes ua_node ON assoc.ua_id = ua_node.id
WHERE assoc.ua_id IN (SELECT ua_id FROM user_uas)
  AND assoc.oa_id IN (
      -- Mgmt OA của phòng Kế Toán
      SELECT n.id
      FROM ngac_nodes n
      WHERE n.name LIKE '%_Mgmt' AND n.node_type = 'OA'
        AND n.id IN (
            SELECT a.oa_id FROM ngac_associations a
            WHERE a.ua_id IN (
                SELECT ngac_ua_id FROM departments WHERE name = 'Kế Toán'
            )
        )
  );

-- Kết quả cho thấy user có quyền gì trên phòng ban:
-- Chief → [read,write,create,delete,admin,upload,manage]
-- Member → [read,write,create,upload]
```

---

## 7. Query Debug — Tìm Nguyên Nhân "Access Denied"

```sql
-- Khi user bị "access denied", chạy query này để tìm nguyên nhân:

-- Bước 1: User đạt tới PC nào?
WITH RECURSIVE user_path AS (
    SELECT parent_id, 1 AS depth
    FROM ngac_assignments
    WHERE child_id = 'USER_NGAC_NODE_ID'
    
    UNION ALL
    
    SELECT a.parent_id, p.depth + 1
    FROM user_path p
    JOIN ngac_assignments a ON a.child_id = p.parent_id
    WHERE p.depth < 10
)
SELECT n.id, n.name, n.node_type
FROM user_path up
JOIN ngac_nodes n ON up.parent_id = n.id
WHERE n.node_type = 'PC';

-- Bước 2: Resource đạt tới PC nào?
WITH RECURSIVE resource_path AS (
    SELECT parent_id, 1 AS depth
    FROM ngac_assignments
    WHERE child_id = 'RESOURCE_NGAC_NODE_ID'
    
    UNION ALL
    
    SELECT a.parent_id, p.depth + 1
    FROM resource_path p
    JOIN ngac_assignments a ON a.child_id = p.parent_id
    WHERE p.depth < 10
)
SELECT n.id, n.name, n.node_type
FROM resource_path rp
JOIN ngac_nodes n ON rp.parent_id = n.id
WHERE n.node_type = 'PC';

-- Bước 3: So sánh → nếu không có PC chung → đó là nguyên nhân
-- Nếu có PC chung → kiểm tra Association:

-- Bước 4: Tìm Association kết nối UA của user với OA của resource
WITH RECURSIVE user_uas AS (
    SELECT 'USER_NGAC_NODE_ID' AS ua_id
    UNION ALL
    SELECT a.parent_id
    FROM user_uas u
    JOIN ngac_assignments a ON a.child_id = u.ua_id
    JOIN ngac_nodes n ON a.parent_id = n.id
    WHERE n.node_type = 'UA'
),
resource_oas AS (
    SELECT 'RESOURCE_NGAC_NODE_ID' AS oa_id
    UNION ALL
    SELECT a.parent_id
    FROM resource_oas r
    JOIN ngac_assignments a ON a.child_id = r.oa_id
    JOIN ngac_nodes n ON a.parent_id = n.id
    WHERE n.node_type = 'OA'
)
SELECT assoc.*, ua_n.name AS ua_name, oa_n.name AS oa_name
FROM ngac_associations assoc
JOIN ngac_nodes ua_n ON assoc.ua_id = ua_n.id
JOIN ngac_nodes oa_n ON assoc.oa_id = oa_n.id
WHERE assoc.ua_id IN (SELECT ua_id FROM user_uas)
  AND assoc.oa_id IN (SELECT oa_id FROM resource_oas);

-- Nếu không có kết quả → thiếu Association → cần tạo
-- Nếu có nhưng thiếu operation → cần update operations
```

---

## 8. Điều cần nhớ

- Thay `'nguyen.van.a'`, `'Kế Toán'`, `'Công ty ABC'` bằng giá trị thực tế
- Approval tables ở tenant schema: `tenant_XXXXXXXX.approval_requests` (8 ký tự đầu workspace ID, bỏ dấu gạch)
- Query recursive CTE (`WITH RECURSIVE`) giả lập BFS traversal mà in-memory graph làm
- DB chỉ dùng để debug — runtime luôn dùng in-memory graph (nhanh hơn nhiều)
- Nếu DB và runtime khác kết quả → graph cần reload (`LoadFullGraph()` khi restart Policy service)
