# NGAC — Permission Graph (Đồ Thị Quyền)

## 1. Giới thiệu

File này mô tả đồ thị quyền NGAC — cách hệ thống xác định "ai được phép làm gì trên tài nguyên nào". Đây là bản đồ toàn cảnh giúp bạn hiểu mối quan hệ giữa User, Department, Role, Permission, và Resource.

## 2. Permission Graph — Tổng Quan

```
┌─────────────────────────────────────────────────────────┐
│                  POLICY CLASS (PC)                       │
│              Phạm vi = Workspace                        │
│                                                         │
│   ┌─── PHÍA NGƯỜI DÙNG ───┐   ┌─── PHÍA TÀI NGUYÊN ──┐│
│   │                        │   │                        ││
│   │  User (U)              │   │  Resource (O)          ││
│   │   ↓ gán vào            │   │   ↓ gán vào            ││
│   │  Role / Group (UA)     │   │  Resource Group (OA)   ││
│   │   ↓ gán vào            │   │   ↓ gán vào            ││
│   │  Policy Class (PC)     │   │  Policy Class (PC)     ││
│   └────────────────────────┘   └────────────────────────┘│
│                                                         │
│   ← Association (liên kết quyền) →                      │
│   UA ──[read, write, ...]──→ OA                         │
└─────────────────────────────────────────────────────────┘
```

**Nguyên tắc cốt lõi:** Quyền chỉ có hiệu lực khi:
1. User (U) có đường đi lên tới một Policy Class (PC)
2. Resource (O/OA) cũng có đường đi lên tới **cùng** PC đó
3. Trên đường đi, có Association nối UA của user với OA của resource, kèm quyền cần kiểm tra

## 3. Graph Theo Vai Trò — Workspace Level

```
User
├── belongs to → Workspace Owners (UA)
│                 ├── → Workspace Mgmt (OA) [toàn quyền]
│                 ├── → Workspace Documents (OA) [toàn quyền]
│                 └── → Workspace Channels (OA) [toàn quyền]
│
├── belongs to → Workspace Members (UA)
│                 ├── → Workspace Documents (OA) [read, write, create, upload]
│                 └── → Workspace Channels (OA) [read, write, create, upload]
│
└── context → Workspace (PC)
```

**Giải thích:**
- Owner có toàn quyền vì thuộc nhóm Owners — Association cấp 7 quyền
- Member có quyền cơ bản vì thuộc nhóm Members — Association cấp 4 quyền
- Cả hai đều nằm trong cùng PC (workspace) nên quyền có hiệu lực

## 4. Graph Theo Vai Trò — Department Level

```
User
├── belongs to → Department Members (UA)
│                 └── → Department Mgmt (OA) [read, write, create, upload]
│
├── optionally → Department Chief (UA) ⊂ Department Members
│                 └── → Department Mgmt (OA) [toàn quyền]
│
└── context → Workspace (PC)
```

**Giải thích:**
- Chief UA nằm BÊN TRONG Dept UA → Chief cũng là thành viên (kế thừa)
- Chief có thêm Association riêng với toàn quyền
- Phòng ban lồng nhau: Dept A thuộc Dept B → thành viên Dept B có thể truy cập tài nguyên Dept A nếu có Association

## 5. Graph Theo Vai Trò — Channel Level

```
User
├── belongs to → Channel Members (UA)
│                 └── → Channel Content (OA) [read, write]
│
└── context → Workspace (PC) hoặc PC_Global (cho DM)
```

**Giải thích:**
- Kênh workspace: Members UA gán vào workspace PC
- Kênh DM: Members UA gán vào PC_Global (không thuộc workspace nào)
- Quyền channel đơn giản: chỉ read và write

## 6. Graph Theo Vai Trò — Drive Level

```
User (quyền từ workspace/department)
│
Folder (OA) ← gán vào → Parent Folder (OA) ← gán vào → Workspace Documents (OA)
│
File (O) ← gán vào → Folder (OA)
```

**Giải thích:**
- File kế thừa quyền từ thư mục chứa nó
- Thư mục kế thừa quyền từ thư mục cha
- Cuối cùng tất cả dẫn về Workspace Documents OA → user có quyền gì trên Documents sẽ có quyền tương tự trên file

## 7. Graph Theo Vai Trò — Approval Level

```
User
├── belongs to → Department Chief (UA)
│                 └── → Scope OA [approve]
│                        └── Dùng để tìm: "Ai có quyền duyệt trong phạm vi này?"
│
└── context → Workspace (PC)
```

**Giải thích:**
- Quyền "approve" được gán qua Association: Chief UA → Scope OA [approve]
- Khi tạo yêu cầu phê duyệt, hệ thống hỏi: "Trong phạm vi OA này, ai có quyền approve?"
- NGAC traverse ngược: từ OA → tìm Association → tìm UA → tìm User

## 8. Ví dụ Thực Tế — Full Graph

Workspace "Công ty ABC", phòng Kế Toán, kênh #general:

```
PC_CongTyABC
│
├── [NHÓM NGƯỜI]
│   ├── Owners (UA) ←── user_owner
│   ├── Members (UA) ←── user_A, user_B, user_C, user_E
│   ├── KeToan_Dept (UA) ←── user_A, user_B
│   │   └── KeToan_Chief (UA) ←── user_A
│   ├── NhanSu_Dept (UA) ←── user_E
│   └── ch_members_general (UA) ←── user_A, user_B, user_C, user_E
│
├── [NHÓM TÀI NGUYÊN]
│   ├── Mgmt (OA)
│   ├── Documents (OA)
│   │   └── DriveRoot (OA) → File1 (O), File2 (O)
│   ├── Channels (OA)
│   │   └── ch_content_general (OA)
│   ├── KeToan_Mgmt (OA)
│   └── NhanSu_Mgmt (OA)
│
└── [LIÊN KẾT QUYỀN]
    ├── Owners → Mgmt [full]
    ├── Owners → Documents [full]
    ├── Owners → Channels [full]
    ├── Members → Documents [read, write, create, upload]
    ├── Members → Channels [read, write, create, upload]
    ├── KeToan_Chief → KeToan_Mgmt [full]
    ├── KeToan_Dept → KeToan_Mgmt [read, write, create, upload]
    ├── NhanSu_Dept → NhanSu_Mgmt [read, write, create, upload]
    └── ch_members_general → ch_content_general [read, write]
```

**Kiểm tra nhanh:**

| Câu hỏi | User A (Chief KT) | User B (NV KT) | User E (NV NS) |
|---|---|---|---|
| Đọc tài liệu chung? | ✅ Member | ✅ Member | ✅ Member |
| Xóa tài liệu KT? | ✅ Chief [delete] | ❌ Dept chỉ [read,write] | ❌ Không thuộc KT |
| Chat #general? | ✅ ch_members [write] | ✅ ch_members [write] | ✅ ch_members [write] |
| Duyệt yêu cầu KT? | ✅ Chief [approve] | ❌ Dept không có [approve] | ❌ Không thuộc KT |
| Xem tài liệu NS? | ❌ Không thuộc NS | ❌ Không thuộc NS | ✅ NhanSu_Dept [read] |

## 9. Điều cần nhớ

- Graph có 2 phía đối xứng: phía người dùng (U→UA→PC) và phía tài nguyên (O→OA→PC)
- Quyền = giao điểm: tìm PC chung → tìm Association → kiểm tra operation
- Thêm quyền = thêm Assignment (gán user vào nhóm)
- Thu hồi quyền = xóa Assignment
- Tạo quyền mới = tạo Association (nối nhóm người ↔ nhóm tài nguyên + operations)
