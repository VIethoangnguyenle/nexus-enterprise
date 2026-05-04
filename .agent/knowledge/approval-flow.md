# Luồng Phê Duyệt (Approval)

## 1. Giới thiệu

File này mô tả hệ thống quy trình phê duyệt — nơi nhân viên tạo yêu cầu (mua hàng, nghỉ phép, thanh toán...) và yêu cầu đó được chuyển qua nhiều bước duyệt bởi những người có thẩm quyền. Hệ thống tự động tìm mẫu phê duyệt phù hợp, phân công người duyệt, và theo dõi tiến trình.

## 2. Các thành phần chính

### Mẫu phê duyệt (Template)

Mẫu phê duyệt định nghĩa "luồng duyệt" cho một loại yêu cầu. Ví dụ:

- Mẫu "Mua hàng trên 10 triệu" → bước 1: Trưởng phòng duyệt, bước 2: Giám đốc duyệt
- Mẫu "Nghỉ phép" → bước 1: Trưởng phòng duyệt

Mỗi mẫu có:
- **Điều kiện kích hoạt** — Ví dụ: "khi số tiền > 10.000.000" hoặc "loại = mua hàng"
- **Các bước duyệt** — Mỗi bước chỉ định ai duyệt và cần bao nhiêu người đồng ý
- **Form nhập liệu** — Các trường dữ liệu người tạo yêu cầu cần điền
- **Độ ưu tiên** — Khi nhiều mẫu cùng khớp, mẫu có độ ưu tiên cao nhất được chọn

### Yêu cầu phê duyệt (Request)

Khi nhân viên tạo yêu cầu, hệ thống tạo một "request" chứa:
- Loại yêu cầu (mua hàng, nghỉ phép...)
- Dữ liệu form đã điền
- Mẫu đang sử dụng (lưu bản sao "đông lạnh" để thay đổi mẫu sau này không ảnh hưởng)
- Trạng thái hiện tại: đang chờ (pending), đã duyệt (approved), bị từ chối (rejected)
- Bước hiện tại (đang ở bước mấy)

### Phân công duyệt (Assignment)

Mỗi bước duyệt tạo ra các "assignment" — phân công cụ thể cho từng người. Ví dụ bước 1 có 3 trưởng phòng được phân công, cần 2 người đồng ý. Mỗi assignment có trạng thái riêng: pending, approved, rejected, skipped.

### Nhật ký kiểm toán (Audit Log)

Mọi hành động đều được ghi lại: ai tạo yêu cầu, ai duyệt, ai từ chối, lúc nào, comment gì. Đây là bản ghi không thể sửa, dùng cho mục đích kiểm toán.

## 3. Luồng hoạt động

### Tạo yêu cầu phê duyệt

Khi nhân viên tạo yêu cầu:

1. Nhân viên chọn loại yêu cầu và điền form
2. Hệ thống tìm mẫu phù hợp:
   - Lấy tất cả mẫu active cho loại yêu cầu này
   - Kiểm tra điều kiện: số tiền có > ngưỡng không? Loại hàng có thuộc danh sách không?
   - Chọn mẫu có độ ưu tiên cao nhất mà tất cả điều kiện đều thỏa mãn
3. "Đông lạnh" mẫu — lưu bản sao hiện tại vào yêu cầu (để thay đổi mẫu sau không ảnh hưởng)
4. Tìm người duyệt cho bước 1:
   - Nếu loại "specific_user" → gán trực tiếp cho người đó
   - Nếu loại "role_in_dept" → hỏi NGAC tìm ai có quyền "approve" → gán cho tất cả
   - Nếu loại "department" → gán cho tất cả thành viên phòng ban
5. Tạo audit log: "Yêu cầu được tạo bởi Nguyễn Văn A"
6. Phát sự kiện qua Kafka → Messaging service tạo thông báo

### Duyệt yêu cầu

Khi người có thẩm quyền nhấn "Duyệt":

1. Hệ thống kiểm tra: yêu cầu còn ở trạng thái "pending" không?
2. Kiểm tra: người duyệt có assignment cho bước hiện tại không?
3. **Kiểm tra quyền realtime** — Đây là bước quan trọng: nếu người duyệt được phân công qua "vai trò" (ví dụ: trưởng phòng), hệ thống kiểm tra lại quyền NGAC ngay lúc duyệt. Tại sao? Vì có thể người đó đã bị chuyển phòng ban kể từ lúc được phân công!
4. Cập nhật assignment thành "approved"
5. Ghi audit log
6. Kiểm tra bước hoàn thành:
   - Đếm số người đã duyệt ở bước hiện tại
   - Nếu đủ số lượng yêu cầu → bước hoàn thành
   - Nếu còn bước tiếp theo → chuyển sang bước tiếp, tìm người duyệt mới
   - Nếu không còn bước nào → yêu cầu được duyệt hoàn toàn
7. Phát sự kiện "approved" qua Kafka → thông báo tới người tạo yêu cầu

### Từ chối yêu cầu

Khi người duyệt nhấn "Từ chối":

1. Kiểm tra tương tự bước duyệt (trạng thái, assignment, quyền)
2. Cập nhật assignment thành "rejected"
3. **Ngay lập tức** skip tất cả assignment còn lại (ở mọi bước)
4. Đánh dấu yêu cầu là "rejected" — kết thúc luồng
5. Ghi audit log
6. Phát sự kiện → thông báo tới người tạo yêu cầu (kèm comment từ chối)

### Duyệt hàng loạt (Batch Approve)

Người duyệt có thể chọn nhiều yêu cầu và duyệt cùng lúc. Hệ thống xử lý từng yêu cầu một nhưng trong cùng một thao tác.

## 4. Ví dụ thực tế

**Tình huống**: Lê Văn C (phòng Kế Toán) tạo yêu cầu mua máy in 15 triệu.

1. C chọn loại "Mua hàng" và điền: tên hàng = "Máy in Canon", số tiền = 15.000.000
2. Hệ thống tìm mẫu:
   - Mẫu "Mua hàng dưới 5 triệu" → điều kiện "amount < 5000000" → KHÔNG khớp
   - Mẫu "Mua hàng 5-20 triệu" → điều kiện "amount between [5000000, 20000000]" → KHỚP
   - Mẫu này có 2 bước: (1) Trưởng phòng Kế Toán, (2) Giám đốc
3. Bước 1: Hệ thống tìm ai là Chief của phòng Kế Toán → Nguyễn Văn A → tạo assignment
4. A nhận thông báo: "Có yêu cầu mua hàng cần duyệt"
5. A xem xét và nhấn "Duyệt" với comment "OK, phòng cần máy in"
6. Bước 1 hoàn thành → chuyển sang bước 2
7. Hệ thống tìm Giám đốc → Phạm Văn X → tạo assignment
8. X duyệt → yêu cầu hoàn thành → C nhận thông báo "Yêu cầu đã được duyệt"

## 5. Điều cần nhớ

- Mẫu phê duyệt dùng hệ thống điều kiện linh hoạt: so sánh số (gt, lt, between), so sánh text (eq, in)
- Khi yêu cầu được tạo, mẫu được "đông lạnh" → thay đổi mẫu sau không ảnh hưởng yêu cầu đang chạy
- Từ chối ở BẤT KỲ bước nào sẽ kết thúc toàn bộ luồng ngay lập tức
- Hệ thống kiểm tra quyền NGAC lại tại thời điểm duyệt — chống trường hợp người bị chuyển phòng nhưng vẫn duyệt được
- Mọi hành động đều có audit log — không thể xóa hay sửa
