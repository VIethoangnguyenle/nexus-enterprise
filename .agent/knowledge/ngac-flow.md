# Luồng NGAC — Cách Quyền Được Xây Dựng và Sử Dụng

## 1. Giới thiệu

File ngac-model.md giải thích NGAC là gì. File này giải thích NGAC hoạt động như thế nào trong thực tế: khi nào đồ thị quyền được xây dựng, khi nào được kiểm tra, và các dịch vụ sử dụng nó ra sao.

## 2. Khi nào đồ thị quyền được xây dựng?

Đồ thị NGAC không được tạo một lần rồi dùng mãi — nó được mở rộng liên tục mỗi khi có thực thể mới:

### Khi tạo user

Mỗi user mới → thêm 1 node U vào đồ thị → gán vào nhóm PublicUsers. Lúc này user chỉ có quyền đọc tài liệu công khai.

### Khi tạo workspace

Đây là lúc đồ thị mở rộng nhiều nhất. Tạo workspace = tạo cả một "cây con" mới:

1. Tạo PC (gốc workspace) — tất cả node của workspace đều treo dưới PC này
2. Tạo 3 OA (nhóm tài nguyên): Quản lý, Tài liệu, Kênh chat
3. Tạo 2 UA (nhóm người): Owners, Members
4. Gán 3 OA vào PC, gán 2 UA vào PC
5. Tạo 5 association (liên kết quyền): Owners được toàn quyền, Members được quyền cơ bản
6. Gán user tạo workspace vào cả 2 nhóm

### Khi tạo phòng ban

Tạo phòng ban = tạo nhánh mới trong cây workspace:

1. Tạo UA cho phòng ban (nơi chứa nhân viên)
2. Tạo UA cho trưởng phòng (nằm trong UA phòng ban → kế thừa quyền)
3. Tạo OA cho tài liệu phòng
4. Gán phòng ban vào cây (workspace PC hoặc phòng ban cha)
5. Tạo association: trưởng phòng toàn quyền, nhân viên đọc/ghi

### Khi tạo kênh chat

Mỗi kênh = 1 cặp node mới:
1. Content OA (nội dung kênh) → gán vào nhóm Channels của workspace
2. Members UA (thành viên kênh) → gán vào workspace PC
3. Association: Members UA → Content OA [đọc, ghi]
4. Người tạo kênh được gán vào Members UA

### Khi tạo file/thư mục trong Drive

Mỗi file hoặc thư mục = 1 node mới:
- Thư mục → tạo node OA → gán vào thư mục cha
- File → tạo node O → gán vào thư mục chứa nó

Nhờ vậy, quyền trên thư mục tự động áp dụng cho tất cả file bên trong.

## 3. Khi nào đồ thị quyền được kiểm tra?

Kiểm tra quyền xảy ra ở MỌI thao tác đọc/ghi dữ liệu. Dưới đây là các dịch vụ sử dụng NGAC:

### Messaging — Gửi/đọc tin nhắn

Trước khi gửi tin nhắn, server kiểm tra: "User A có quyền write trên Content OA của kênh X không?" Nếu không → từ chối.

Khi hiển thị danh sách kênh, server kiểm tra TỪNG kênh: "User A có quyền read không?" Chỉ trả về kênh mà user có quyền.

### Drive — Upload/xem file

Trước khi upload file vào thư mục, kiểm tra: "User A có quyền write trên OA của thư mục cha không?"

Trước khi xem file, kiểm tra quyền read.

### Approval — Duyệt yêu cầu

Khi tìm người duyệt cho một bước: hỏi NGAC "ai có quyền approve trên OA phạm vi này?" → tìm ra tất cả user có quyền.

Khi người duyệt bấm "Duyệt": kiểm tra lại quyền NGAC ngay lúc đó. Tại sao kiểm tra lại? Vì có thể từ lúc phân công đến lúc duyệt, người đó đã bị chuyển phòng ban → mất quyền.

### Asset — Quản lý tài sản

Kiểm tra quyền trước khi cho phép thay đổi trạng thái tài sản (ví dụ: chuyển từ "đang dùng" sang "bảo trì").

## 4. Các kiểu kiểm tra quyền

Hệ thống dùng 4 kiểu kiểm tra khác nhau:

### Kiểu 1: Kiểm tra trực tiếp (phổ biến nhất)

"User A có quyền X trên object Y không?" → Có/Không

Dùng khi: gửi tin nhắn, upload file, sửa tài liệu. Đơn giản và nhanh.

### Kiểu 2: Lọc theo quyền (cho danh sách)

"Trong 20 kênh, user A có quyền đọc kênh nào?" → Trả về 15 kênh

Dùng khi: hiển thị danh sách kênh, danh sách file. Server kiểm tra TỪNG item rồi lọc.

### Kiểu 3: Tìm phạm vi truy cập (cho approval)

"User A có quyền approve trên những phạm vi nào?" → Trả về danh sách OA IDs

Dùng khi: hiển thị các yêu cầu phê duyệt mà user có thể xem/duyệt.

### Kiểu 4: Kiểm tra lại tại thời điểm hành động

"Lúc phân công, A có quyền. Nhưng BÂY GIỜ, A còn quyền không?"

Dùng khi: duyệt/từ chối yêu cầu phê duyệt. Chống trường hợp quyền bị thu hồi giữa chừng.

## 5. Ví dụ thực tế — Cây quyền workspace

Khi workspace "Công ty ABC" được tạo với phòng Kế Toán và kênh #general, đồ thị trông như sau:

```
PC_CongTyABC (gốc)
├── Owners (UA) ←── user_owner
│   ├── → Mgmt (OA) [toàn quyền]
│   ├── → Documents (OA) [toàn quyền]
│   └── → Channels (OA) [toàn quyền]
├── Members (UA) ←── user_A, user_B
│   ├── → Documents (OA) [đọc, ghi, tạo, upload]
│   └── → Channels (OA) [đọc, ghi, tạo, upload]
├── KeToan_Dept (UA) ←── user_A, user_B
│   ├── KeToan_Chief (UA) ←── user_A
│   │   └── → KeToan_Mgmt (OA) [toàn quyền]
│   └── → KeToan_Mgmt (OA) [đọc, ghi, tạo, upload]
├── ch_members_general (UA) ←── user_A, user_B
│   └── → ch_content_general (OA) [đọc, ghi]
└── Documents (OA)
    └── DriveRoot (OA)
        └── File_BaoCaoQ1 (O)
```

Từ cây này, hệ thống có thể trả lời mọi câu hỏi quyền:
- "A đọc được file BáoCáoQ1 không?" → A ∈ Members → Members → Documents [read] → Documents → DriveRoot → File → **CÓ**
- "B xóa được tài liệu KT không?" → B ∈ KeToan_Dept → KeToan_Mgmt nhưng chỉ có [read, write, create, upload], không có delete → **KHÔNG**

## 6. Điều cần nhớ

- Đồ thị NGAC được load **per-workspace** qua `ShardManager` với O(1) LRU eviction (doubly-linked list + index map). Shard được lazy-load khi có request và evict khi vượt `maxShards` (default 1000)
- Global graph vẫn load vào bộ nhớ khi Policy service khởi động → fallback nếu shard chưa sẵn sàng
- Mọi thay đổi (tạo node, gán, liên kết) đều lưu vào database VÀ cập nhật trong bộ nhớ
- Thêm thành viên vào phòng ban = tạo 1 assignment mới → quyền có hiệu lực ngay
- Xóa thành viên khỏi phòng ban = xóa 1 assignment → mất quyền ngay
- Policy service có 2 phiên bản: Write (1 instance, cho thao tác sửa đổi) và Read (2 instances, cho kiểm tra quyền — vì đọc nhiều hơn ghi)
- Kết quả quyết định sử dụng typed constants: `DecisionAllow` / `DecisionDeny` (không hardcode string)
