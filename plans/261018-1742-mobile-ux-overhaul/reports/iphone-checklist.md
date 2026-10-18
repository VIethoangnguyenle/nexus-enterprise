# Danh sách kiểm tra xác thực trên thiết bị thật iPhone (Phase 12)

Tài liệu kiểm tra thực địa trên iPhone thật dành cho Phase 12 thuộc kế hoạch `plans/261018-1742-mobile-ux-overhaul` (PWA đã cài đặt ở chế độ standalone, độ rộng màn hình 390pt). Mỗi dòng kiểm tra tương ứng với một kịch bản nghiệm thu (acceptance scenario) được trích xuất đầy đủ từ 10 đặc tả: `mobile-shell`, `user-settings`, `conversation-settings`, `chat-group-management`, `chat-mention-all`, `unified-search`, `mobile-gestures`, `contacts-visibility`, `drive-tree-navigation` và `file-preview`.

Các mục cử chỉ đặc thù chỉ có trên màn hình cảm ứng được đánh dấu rõ:
- `[cảm ứng: edge back]`
- `[cảm ứng: row swipe]`
- `[cảm ứng: long-press slide]`
- `[cảm ứng: double-tap]`
- `[cảm ứng: keyboard drag]`
- `[cảm ứng: pinch]`

---

## 1. Thanh đáy & Thêm

- [ ] Trên iPhone (PWA cài đặt, 390pt), mở thanh đáy, sheet Thêm hoặc trang Sửa thanh đáy trong workspace cá nhân; kết quả là Phê duyệt, Tài sản và Quản trị hoàn toàn không xuất hiện ở bất kỳ đâu; Workspace: Cá nhân; Giao diện: sáng.
- [ ] Trên iPhone, cấu hình đưa phân hệ Tài sản lên thanh đáy ở workspace A rồi chuyển sang workspace B; kết quả là workspace B hiển thị lựa chọn thanh đáy riêng của B và lựa chọn của A vẫn được giữ nguyên khi quay lại; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, mở thanh đáy sau khi một đồng nghiệp cùng workspace vừa lưu cấu hình thanh đáy của họ; kết quả là không nhìn thấy cấu hình của đồng nghiệp và khi tự lưu thanh đáy của mình thì cấu hình của đồng nghiệp không bị ảnh hưởng; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, thử gửi yêu cầu lưu thanh đáy có 3 phân hệ, có phân hệ trùng lặp, có Quản trị hoặc có Phê duyệt trong workspace cá nhân; kết quả là hệ thống trả về lỗi 400 và không có thay đổi nào được ghi lại; Workspace: Cá nhân; Giao diện: tối.
- [ ] Trên iPhone, chạm vào biểu tượng ⚙ trên khối workspace trong sheet Thêm hoặc dấu ⋯ của workspace đang mở trong bộ chọn workspace; kết quả là trang Cài đặt workspace (/settings/workspace) mở ra và sheet Thêm không có ô Quản trị; Workspace: Tổ chức; Giao diện: sáng.

---

## 2. Tôi & Cài đặt

- [ ] Trên iPhone, tìm cách chuyển đổi workspace cá nhân thành tổ chức (hoặc ngược lại) trong giao diện Cài đặt hoặc truy cập trực tiếp đường dẫn chuyển đổi; kết quả là không có nút hành động chuyển đổi nào trong ứng dụng và đường dẫn bị từ chối; Workspace: Cá nhân; Giao diện: sáng.
- [ ] Trên iPhone, thay đổi tuỳ chọn thông báo tin nhắn trực tiếp trong Tôi → Thông báo → Chat rồi mở màn hình Cài đặt Chat; kết quả là cả hai màn hình đều hiển thị chính xác cùng một giá trị đã lưu; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, thành viên thông thường mở Cài đặt workspace trong workspace tổ chức; kết quả là chỉ nhìn thấy tên, mô tả, danh sách quản trị viên, các quy tắc ở chế độ chỉ đọc, hồ sơ của mình và nút Rời, không xuất hiện các mục quản lý thành viên, vai trò, phòng ban, mẫu hay dung lượng; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, cố gắng đọc hoặc lưu cài đặt phạm vi workspace của một workspace mà mình không phải là thành viên; kết quả là hệ thống trả về mã lỗi 403 và không có dữ liệu nào được ghi; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, bật tính năng Tạm dừng thông báo đến một thời điểm xác định trong tương lai từ Tôi → Thông báo và nhận tin nhắn có nhắc tên; kết quả là không có thông báo đẩy nào được gửi đến, sau khi hết mốc thời gian tạm dừng thì thông báo đẩy hoạt động trở lại bình thường; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, tắt thông báo đẩy cho tin nhắn trực tiếp trong Tôi → Thông báo rồi nhận một tin nhắn trực tiếp mới; kết quả là không có thông báo đẩy nào được gửi đến thiết bị; Workspace: Cá nhân; Giao diện: sáng.
- [ ] Trên iPhone, trong khung giờ yên lặng đã cài đặt, nhận tin nhắn không nhắc tên thì không có thông báo đẩy gửi đến; sau đó nhận tin nhắn có nhắc đích danh tên mình hoặc @Tất cả thì nhận được thông báo đẩy; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, khi tính năng tạo nhóm đang bị quản trị viên khoá ở cấp workspace, quản trị viên mở màn hình vai trò và quyền để gán quyền tạo kênh cho một vai trò tự tạo; kết quả là màn hình không cung cấp quyền này và yêu cầu gửi lên bị từ chối; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, quản trị viên thực hiện mở khoá quyền tạo nhóm trong Cài đặt workspace; kết quả là các thành viên thông thường tạo được nhóm trở lại, còn các vai trò tự tạo bị mất quyền trước đó vẫn không tự động có lại cho đến khi được gán thủ công; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, thành viên không có quyền quản trị cố gắng thiết lập khoá tạo nhóm trong Cài đặt workspace; kết quả là hệ thống trả về lỗi 403 và trạng thái đồ thị quyền không thay đổi; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, lưu cài đặt mang một giá trị không hợp lệ (như giờ kết thúc trước giờ bắt đầu làm việc); kết quả là hệ thống trả về lỗi 400 hoặc 422 và không có giá trị nào bị ghi đè; Workspace: Cá nhân; Giao diện: tối.
- [ ] Trên iPhone, chạm vào dấu ⋯ trên tiêu đề của Chat, Lịch, Tài liệu, Phê duyệt hoặc Tài sản; kết quả là dòng đầu tiên trong menu là "Cài đặt <phân hệ>" và chạm vào sẽ mở đúng trang /settings/<section> của phân hệ đó; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone trong workspace cá nhân, mở đường dẫn /settings/approval, /settings/assets hoặc /settings/org-profile; kết quả là ứng dụng tự động chuyển hướng về trang Tôi (/settings) thay vì báo lỗi; Workspace: Cá nhân; Giao diện: tối.
- [ ] Trên iPhone, vào Tôi → Tài khoản → Phiên đăng nhập; kết quả là danh sách chỉ hiển thị các phiên của chính mình với nhãn thiết bị dạng "Safari · iPhone", trong đó phiên đang hoạt động trên máy được xếp đầu tiên; Workspace: Cá nhân; Giao diện: sáng.
- [ ] Trên iPhone, gửi yêu cầu kết thúc một phiên đăng nhập thuộc về người dùng khác; kết quả là nhận mã lỗi 404 và phiên đăng nhập của người kia vẫn duy trì bình thường; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, gửi yêu cầu xóa chính phiên đăng nhập đang sử dụng; kết quả là nhận mã lỗi 400 và phiên không bị xóa (phải sử dụng nút Đăng xuất); Workspace: Tổ chức; Giao diện: sáng.

---

## 3. Chat

- [ ] Trên iPhone, mở liên kết cũ dẫn đến tab Chat (như /channels/mentions hoặc /channels/starred); kết quả là ứng dụng mở đúng chip tương ứng trên hàng chip của Chat; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone trong workspace tổ chức bị khoá tạo nhóm, thành viên thông thường chạm nút + để tạo nhóm mới; kết quả là thao tác bị từ chối quyền tạo kênh; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone khi khoá tạo nhóm đang bật, thành viên có vai trò tự tạo thuộc nhóm Channels thử tạo nhóm; kết quả là quyền tạo kênh bị từ chối; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone khi khoá tạo nhóm đang bật, chủ sở hữu workspace chạm tạo nhóm; kết quả là tạo nhóm thành công; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, chạm vào tên cuộc trò chuyện ở thanh trên cùng; kết quả là màn hình Cài đặt cuộc trò chuyện mở ra và thanh trên chỉ còn nút Gọi và nút Thread; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, thành viên không có quyền đổi tên mở Cài đặt cuộc trò chuyện của nhóm; kết quả là không có thao tác đổi tên và yêu cầu đổi tên gửi lên bị từ chối; Workspace: Tổ chức; Giao diện: tối.
- [ ] [cảm ứng: row swipe] Trên iPhone, chuyển cuộc trò chuyện vào mục từ thao tác nhấn giữ, từ vuốt sang trái chọn Thêm hoặc từ dòng Mục trong Cài đặt cuộc trò chuyện; kết quả là cả ba nơi đều mở cùng một bộ chọn mục và cuộc trò chuyện được chuyển đúng mục; Workspace: Cá nhân; Giao diện: sáng.
- [ ] Trên iPhone, người không phải thành viên của nhóm cố gắng gửi trả lời vào một thread của nhóm đó; kết quả là hệ thống từ chối quyền ghi và không lưu tin nhắn; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, người dùng gửi tin nhắn giả mạo kiểu tin hệ thống vào nhóm chat; kết quả là tin nhắn bị từ chối, trong khi thẻ phê duyệt tạo qua luồng phê duyệt hợp lệ vẫn được gửi theo đúng quyền đăng bài; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, thành viên gửi tin nhắn có nhắc @Tất cả trong nhóm có 7 người; kết quả là 6 thành viên còn lại đều nhận được thông báo như được nhắc tên trực tiếp; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, khi nhóm hạn chế quyền nhắc mọi người, thành viên sửa tin nhắn đã gửi để chèn thêm @Tất cả; kết quả là thao tác chỉnh sửa bị từ chối và không ai nhận thông báo; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, khi nhóm hạn chế quyền nhắc mọi người, quản trị nhóm gửi tin nhắn có @Tất cả; kết quả là tin nhắn được gửi thành công đến các thành viên; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, gõ và gửi @Tất cả trong một cuộc trò chuyện trực tiếp 1-1; kết quả là tin nhắn bị từ chối và không có ai nhận thông báo; Workspace: Cá nhân; Giao diện: tối.
- [ ] Trên iPhone, thành viên không phải quản trị viên workspace gửi, trả lời hoặc sửa tin để chèn @Tất cả trong kênh chung của workspace; kết quả là mọi thao tác đều bị từ chối; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, quản trị viên workspace gửi tin nhắn có @Tất cả trong kênh chung của workspace; kết quả là toàn bộ thành viên khác trong workspace nhận được thông báo như được nhắc tên; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, mở danh sách thành viên của một nhóm có người bị danh bạ ẩn; kết quả là người đó vẫn hiện tên và avatar qua bộ phân giải tên, chạm vào chỉ hiện thẻ người bị ẩn không có nút hành động, nhưng nút Nhắn tin trực tiếp trong danh sách vẫn bấm được; Workspace: Tổ chức; Giao diện: sáng.

---

## 4. Quản lý nhóm

- [ ] Trên iPhone, chủ sở hữu workspace không thuộc nhóm riêng tư cố gắng tự chỉ định mình làm quản trị nhóm, đổi cài đặt nhóm hoặc gửi @Tất cả; kết quả là mọi hành động đều bị từ chối và vẫn không đọc được nội dung nhóm; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, quản trị nhóm rời nhóm hoặc bị xoá khỏi nhóm; kết quả là lập tức mất quyền đọc, gửi tin và mời người vào nhóm; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, chủ sở hữu workspace không thuộc nhóm thực hiện đổi tên nhóm hoặc xoá một thành viên; kết quả là thao tác kiểm duyệt thành công mà chủ sở hữu vẫn không xem được nội dung tin nhắn trong nhóm; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, khi nhóm đặt "Ai được gửi tin" là chỉ chủ nhóm và quản trị nhóm, thành viên thường gửi tin nhắn chính; kết quả là bị từ chối, nhưng trả lời trong thread, thả cảm xúc và sửa câu trả lời của mình vẫn thành công; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, khi quyền gửi tin bị giới hạn, thành viên thường tạo bình chọn, công việc hoặc thẻ phê duyệt ở luồng chính; kết quả là bị từ chối, nhưng bỏ phiếu vào bình chọn có sẵn hoặc đóng bình chọn cũ của mình vẫn thành công; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, quản trị nhóm gửi tin nhắn chính khi quyền gửi bị giới hạn, hoặc thành viên gửi tin khi quyền gửi ở chế độ mọi thành viên; kết quả là tin nhắn được gửi thành công; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, khi nhóm đặt "Ai được thêm người" là chỉ chủ nhóm và quản trị nhóm, thành viên thường mời thêm người; kết quả là bị từ chối quyền mời; quản trị nhóm mời cùng người đó thì thành công; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, khi nhóm hạn chế quyền nhắc @Tất cả, thành viên thường gửi, trả lời hoặc sửa tin có @Tất cả; kết quả là bị từ chối; quản trị nhóm thực hiện gửi @Tất cả thì thành công; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, khi quyền thêm người bị giới hạn và một quản trị nhóm bị hạ xuống thành viên thường; kết quả là người đó lập tức bị từ chối quyền mời thêm người ngay khi quyền được cập nhật; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, chủ nhóm hoặc quản trị nhóm chỉ định một thành viên làm quản trị nhóm; kết quả là người được chỉ định nhận quyền quản trị nhóm ngay lập tức; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, thành viên thông thường cố gắng chỉ định một người khác làm quản trị nhóm; kết quả là thao tác bị từ chối và không ghi nhận thay đổi; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, chủ nhóm gỡ quyền của một quản trị nhóm; kết quả là thành công và người đó trở lại làm thành viên thường; thành viên thường cố gắng gỡ quyền quản trị thì bị từ chối; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, chủ nhóm chuyển quyền chủ nhóm cho một thành viên khác; kết quả là chuyển quyền thành công; người không phải chủ nhóm cố chuyển quyền thì bị từ chối; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, chủ nhóm chọn Giải tán nhóm; kết quả là nhóm và toàn bộ nút đồ thị liên quan bị xóa hoàn toàn khỏi hệ thống; quản trị nhóm cố giải tán nhóm thì bị từ chối; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, sau khi chuyển quyền chủ nhóm từ A sang B, người cũ A cố xoá chủ mới B hoặc cố giải tán nhóm; kết quả là đều bị từ chối, nhưng A vẫn thực hiện được các quyền của quản trị nhóm; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, chủ nhóm chọn Rời nhóm khi chưa chuyển quyền chủ nhóm; kết quả là bị từ chối kèm thông báo yêu cầu chuyển quyền chủ nhóm trước; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, chủ nhóm hoặc quản trị nhóm xoá một thành viên khỏi nhóm; kết quả là thành viên bị xoá và không còn quyền đọc nhóm đó nữa; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, thành viên thông thường cố gắng xoá một thành viên khác khỏi nhóm; kết quả là thao tác bị từ chối; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, chủ sở hữu workspace không thuộc nhóm thực hiện xoá một thành viên; kết quả là xoá thành công mà chủ sở hữu vẫn không đọc được nội dung nhóm; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, chủ nhóm cũ A (nay là quản trị nhóm) xoá một thành viên; kết quả là thành công với tư cách quản trị nhóm; khi A bị hạ xuống thành viên thường thì không còn xoá được ai nữa; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, đối với nhóm mà người tạo ban đầu đã rời nhóm trước đợt cập nhật vai trò; kết quả là một thành viên hiện tại trở thành chủ nhóm và người tạo cũ vẫn không đọc được nhóm; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, kiểm tra nhóm sau khi lệnh khởi tạo vai trò chạy lại; kết quả là nhóm vẫn giữ nguyên chủ nhóm và các quản trị viên mà không bị nhân bản dữ liệu; Workspace: Tổ chức; Giao diện: sáng.

---

## 5. Tìm

- [ ] Trên iPhone, gõ từ khoá "đối soát" trong ô tìm kiếm của Tài liệu rồi chuyển phạm vi sang "Tất cả"; kết quả là từ khoá đã gõ được giữ nguyên và kết quả hiển thị gom theo từng loại (tối đa 3 mục mỗi nhóm cùng nút "Xem tất cả · N"); Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, tìm kiếm đề nghị hoặc tài liệu với tham số trỏ sang workspace B mà mình không có quyền; kết quả là nhận kết quả 403 và không có dữ liệu nào của B bị lộ; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, tìm kiếm tệp tài liệu khớp từ khoá nhưng người dùng không có quyền đọc; kết quả là tệp không xuất hiện trong kết quả và không được cộng vào số lượng "Xem tất cả · N"; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, tìm kiếm với từ khoá "nghỉ phép" khi bản thân đã từng gửi đề nghị có tên mẫu khớp; kết quả là đề nghị đó xuất hiện trong kết quả tìm kiếm; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, người duyệt tìm kiếm từ khoá "mua sắm" cho đề nghị được phân công duyệt; kết quả là đề nghị đó xuất hiện trong kết quả, đề nghị không liên quan không xuất hiện; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, trưởng phòng tìm kiếm đề nghị thuộc phạm vi phòng ban mình quản lý; kết quả là các đề nghị trong phạm vi phòng ban xuất hiện dù trưởng phòng không phải người tạo hay người duyệt trực tiếp; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone trong workspace cá nhân, gõ từ khoá khớp với hành động "Tạo đề nghị"; kết quả là ô tìm kiếm không gợi ý hành động này; Workspace: Cá nhân; Giao diện: sáng.

---

## 6. Vuốt

- [ ] [cảm ứng: edge back] Trên iPhone (PWA cài đặt, 390pt), mở cuộc trò chuyện từ thông báo đẩy khi chưa có lịch sử màn hình trước đó rồi vuốt từ mép trái trong khoảng 20px; kết quả là ứng dụng chuyển về màn hình danh sách gốc của Chat; Workspace: Tổ chức; Giao diện: sáng.
- [ ] [cảm ứng: edge back] [cảm ứng: row swipe] Trên iPhone, vuốt bắt đầu từ vị trí cách mép trái 10px đè lên tin nhắn trong thread; kết quả là kích hoạt cử chỉ quay lại (back); nếu bắt đầu vuốt từ vị trí cách mép 40px sang phải thì kích hoạt trích dẫn trả lời tin nhắn; Workspace: Tổ chức; Giao diện: tối.
- [ ] [cảm ứng: edge back] Trên iPhone, mở một thread trong cuộc trò chuyện rồi vuốt từ mép trái để quay lại; kết quả là màn hình quay lại cuộc trò chuyện cha nhưng lớp peek phía dưới trống (thread peek blank do route chỉ đổi query parameter); Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, người dùng không vuốt màn hình kiểm tra các nút thay thế; kết quả là menu nhấn giữ tin nhắn có tuỳ chọn Trả lời / Trích dẫn và thanh trên cùng luôn có nút mũi tên ← để quay lại; Workspace: Cá nhân; Giao diện: sáng.
- [ ] [cảm ứng: row swipe] Trên iPhone, vuốt một tin nhắn sang phải quá 64px trong cuộc trò chuyện; kết quả là thanh soạn thảo hiển thị khung trích dẫn và bàn phím mở lên; Workspace: Tổ chức; Giao diện: tối.
- [ ] [cảm ứng: row swipe] Trên iPhone, vuốt một tin nhắn sang trái quá 64px; kết quả là mở màn hình thread của tin nhắn đó; Workspace: Tổ chức; Giao diện: sáng.
- [ ] [cảm ứng: long-press slide] Trên iPhone, nhấn giữ một tin nhắn rồi trượt ngón tay lên thanh 6 biểu tượng cảm xúc và thả tay; kết quả là thả đúng cảm xúc đã chọn vào tin nhắn; nếu nhấc tay trước khi trượt thì mở sheet thao tác; Workspace: Tổ chức; Giao diện: tối.
- [ ] [cảm ứng: double-tap] Trên iPhone, chạm đúp nhanh (trong vòng 300ms) vào một tin nhắn; kết quả là thả cảm xúc nhanh (mặc định 👍 hoặc cảm xúc đã chọn trong cài đặt), chạm đúp lần nữa để gỡ bỏ; Workspace: Cá nhân; Giao diện: sáng.
- [ ] [cảm ứng: keyboard drag] Trên iPhone, khi bàn phím đang mở trong cuộc trò chuyện, kéo luồng tin nhắn xuống dưới quá 48px; kết quả là bàn phím tự động đóng lại; Workspace: Tổ chức; Giao diện: tối.
- [ ] [cảm ứng: keyboard drag] Trên iPhone, kéo thanh nắm (grip) phía trên ô soạn thảo lên quá 25% chiều cao màn hình; kết quả là mở rộng ô soạn thảo toàn màn hình; kéo xuống quá 25% thì thu gọn về vị trí cũ; chạm vào thanh nắm thì chuyển đổi giữa hai trạng thái; Workspace: Tổ chức; Giao diện: sáng.
- [ ] [cảm ứng: row swipe] Trên iPhone, ở đầu danh sách bất kỳ, kéo xuống quá 56px rồi thả tay; kết quả là hiển thị vòng quay tải lại và danh sách được làm mới; Workspace: Tổ chức; Giao diện: tối.
- [ ] [cảm ứng: row swipe] Trên iPhone, trên bề mặt một bottom sheet, trang Tôi hoặc trình xem ảnh, kéo xuống quá 25% chiều cao hoặc vuốt nhanh xuống; kết quả là đóng bề mặt đang mở; Workspace: Cá nhân; Giao diện: sáng.
- [ ] Trên iPhone, khi bật chế độ giảm chuyển động trong cài đặt thiết bị hoặc ứng dụng; kết quả là các cử chỉ không trượt theo ngón tay mà chuyển trạng thái ngay khi vượt ngưỡng hoặc chỉ mờ dần; Workspace: Tổ chức; Giao diện: tối.
- [ ] [cảm ứng: row swipe] Trên iPhone, mở danh sách thông báo và vuốt hàng sang phải; kết quả là chỉ hiển thị nút Đã đọc, chưa có nút Xoá hay Chưa đọc (chưa xây dựng); Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, mở phân hệ Tài sản và thử vuốt các hàng tài sản; kết quả là hàng tài sản chưa hỗ trợ thao tác vuốt (chưa xây dựng); Workspace: Tổ chức; Giao diện: tối.

---

## 7. Tài liệu

- [ ] Trên iPhone (PWA cài đặt, 390pt), mở phân hệ Tài liệu; kết quả là chỉ nhìn thấy duy nhất một ô tìm kiếm dạng viên thuốc ở đầu trang, không có ô tìm kiếm thứ hai; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, tắt mục "Được chia sẻ tài liệu" trong Cài đặt Tài liệu; kết quả là chỉ giá trị notify_shared bị tắt, các thiết lập về kiểu xem, sắp xếp, đưa thư mục lên đầu và tải trước qua 4G giữ nguyên; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, mở Tài liệu; kết quả là danh sách hiển thị các mục ở cấp gốc (các thư mục con bên trong chưa được tải sẵn); Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, mở danh mục Thư mục và chạm vào thư mục cha; kết quả là thư mục con được tải về và hiển thị; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, quay lại thư mục cha sau khi mở thư mục con; kết quả là dữ liệu thư mục con được lưu trong bộ nhớ đệm, mở lại không cần tải lại từ đầu; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên thiết bị hoặc giao diện có nút "Mở cây thư mục", chạm nút; kết quả là ngăn kéo cây thư mục mở ra và chọn một thư mục sẽ cập nhật đường dẫn URL rồi đóng ngăn kéo; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, chạm vào vùng tối (scrim) bên ngoài ngăn kéo hoặc bấm phím thoát; kết quả là ngăn kéo đóng lại; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone khi dùng bàn phím gắn ngoài, dùng phím mũi tên và Enter để điều hướng cây thư mục; kết quả là mở đúng thư mục được chọn; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, tải lại trang khi đang ở đường dẫn /drive?folder=<id>; kết quả là hiển thị đúng nội dung thư mục đó kèm thanh điều hướng breadcrumb; Workspace: Tổ chức; Giao diện: sáng.
- [ ] [cảm ứng: edge back] Trên iPhone, mở thư mục con rồi vuốt từ mép trái để quay lại; kết quả là hiển thị lại thư mục trước đó; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, chuyển sang một workspace khác từ sheet chọn workspace; kết quả là mở tại thư mục gốc của workspace mới, bỏ các tham số folder và view cũ; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, mở đường dẫn đến một thư mục đã bị xoá hoặc không có quyền xem; kết quả là danh sách thông báo lỗi rõ ràng và cung cấp liên kết quay về thư mục gốc; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, chạm vào chip "Chia sẻ với tôi" trên đầu Tài liệu; kết quả là URL chuyển sang view=shared và danh sách hiển thị các tệp được chia sẻ kèm tên người sở hữu; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, truy cập hoặc thêm tệp vào một thư mục đã chuyển vào thùng rác; kết quả là hệ thống báo lỗi không tìm thấy (NotFound); Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, gửi yêu cầu thao tác trong workspace A nhưng dùng id thư mục thuộc workspace B; kết quả là hệ thống trả về lỗi không tìm thấy và không ghi dữ liệu; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, chọn di chuyển một thư mục vào chính nó hoặc vào thư mục con của nó; kết quả là hệ thống báo lỗi tham số không hợp lệ và cấu trúc cây không bị thay đổi; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, thực hiện di chuyển thư mục nhưng bị chính sách từ chối do tạo vòng lặp; kết quả là vị trí cũ của thư mục được giữ nguyên vẹn; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, di chuyển thư mục gặp lỗi khi gỡ liên kết cũ; kết quả là thao tác được hoàn tác và báo lỗi về giao diện; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, thao tác di chuyển đồng thời một tệp; kết quả là tệp kết thúc ở đúng một thư mục đích duy nhất, thao tác xung đột bị huỷ an toàn; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, chọn chuyển một thư mục ra ngoài cùng (không chọn thư mục cha); kết quả là thư mục xuất hiện ở cấp cao nhất của Tài liệu; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, rời Tài liệu sang Chat rồi quay lại Tài liệu; kết quả là trạng thái đóng/mở của các nhánh thư mục được giữ nguyên; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, mở một thư mục sâu theo đường dẫn; kết quả là tên thư mục hiện tại được đánh dấu đang chọn và các thư mục cha được làm nổi bật; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, người dùng được chia sẻ một thư mục con riêng lẻ mà không được chia sẻ thư mục cha; kết quả là chỉ thấy thư mục con đó, không thấy cấu trúc thư mục cha không có quyền; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, tạo thư mục đầu tiên trong workspace có cấu hình Documents OA; kết quả là thư mục được tạo chính xác dưới nút Documents của workspace; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, tạo thư mục trong workspace chưa ghi nhận Documents OA; kết quả là hệ thống sử dụng DriveRoot chuẩn và hiển thị thư mục bình thường; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, truy cập tab Tệp của hai kênh cùng mang tên "general"; kết quả là mỗi kênh hiển thị đúng tệp riêng của kênh đó mà không bị trộn lẫn; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, nếu xảy ra lỗi trong quá trình khởi tạo phân hệ tài liệu; kết quả là hệ thống hoàn tác an toàn và không tạo bản ghi rác; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, mở đồng thời Tài liệu khi chưa có thư mục gốc; kết quả là hệ thống khởi tạo đúng duy nhất một thư mục gốc và tải dữ liệu thành công; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, xem danh sách các thư mục cấp cao nhất do người dùng tạo; kết quả là tất cả thư mục đều hiển thị đầy đủ bên dưới thư mục gốc chuẩn; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, cố tình mở kho tệp của một kênh thuộc workspace khác; kết quả là yêu cầu bị từ chối quyền truy cập (PermissionDenied); Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, mở phân hệ Tài liệu; kết quả là chip "Gần đây" mở mặc định, hiển thị mục Truy cập nhanh (tối đa 6 thẻ có kèm lý do) phía trên danh sách tệp được gom nhóm Hôm nay · Tuần này · Tháng này · Cũ hơn; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, chạm vào chip "Thư mục"; kết quả là hiển thị các mục ở cấp cao nhất với thư mục xếp trước tệp, tiêu đề mang tên "Tài liệu của tôi" ở workspace cá nhân và mang tên tổ chức ở workspace tổ chức; Workspace: Cá nhân; Giao diện: tối.
- [ ] [cảm ứng: edge back] Trên iPhone, mở một thư mục; kết quả là thanh trên có nút ← quay lại thư mục cha (từ thư mục gốc quay về chip Thư mục, không về Gần đây), tên thư mục, kính lúp tìm kiếm trong thư mục, nút ⋯, thanh đường dẫn và hàng sắp xếp/đổi kiểu xem; Workspace: Tổ chức; Giao diện: sáng.
- [ ] [cảm ứng: row swipe] Trên iPhone, danh sách tệp hiển thị dạng hàng cao 60px có ô loại tệp 40px, tên tệp, ngôi sao nếu đã gắn sao và nút ⋯; vuốt hàng sang trái lộ các nút Chia sẻ (khi có quyền) · Gắn sao · Thêm, không có nút vuốt xoá; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, chạm dấu ⋯ của một hàng tệp; kết quả là mở bottom sheet hiển thị 4 lối tắt (Chia sẻ, Sao chép liên kết, Gắn sao, Tải xuống) và các mục Chuyển tới thư mục…, Tạo bản sao, Thông tin và hoạt động, Đổi tên, Chuyển vào thùng rác theo đúng quyền người dùng; Workspace: Tổ chức; Giao diện: sáng.
- [ ] [cảm ứng: row swipe] Trên iPhone, nhấn giữ một hàng tệp; kết quả là bật chế độ chọn nhiều với ô đánh dấu thay cho icon loại tệp, thanh trên hiện "Đã chọn N" kèm nút ✕ và "Chọn tất cả", thanh màu tối phía dưới thay thế thanh đáy gồm Chia sẻ, Chuyển, Tải xuống, Gắn sao, Xoá; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, chạm nút + trong thư mục; kết quả là sheet "Thêm vào <thư mục>" mở ra cung cấp các mục: Tải tệp lên, Chụp ảnh, Thư mục mới, Văn bản mới; chọn Văn bản mới sẽ tạo và mở văn bản ngay trong thư mục; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, khi tệp đang tải lên; kết quả là xuất hiện thẻ tiến trình nổi phía trên thanh đáy báo "Đang tải lên N tệp" cùng thanh tiến độ và thời gian; khi cuộn danh sách xuống thì thẻ thu gọn thành vòng tròn %; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, khi toàn bộ tệp trong hàng đợi tải lên hoàn tất; kết quả là xuất hiện thông báo toast "Đã tải lên N tệp vào <thư mục>" kèm nút Xem để mở thư mục đó; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, nếu một tệp tải lên gặp lỗi; kết quả là tệp lỗi hiển thị trên thẻ kèm nguyên nhân và nút "Thử lại", các tệp còn lại trong hàng đợi vẫn tiếp tục được tải; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, tải lại trang khi đang có tệp trong hàng đợi tải lên; kết quả là ứng dụng tự động tiếp tục tải lại từ đầu sau khi trang tải xong; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, đăng xuất khỏi tài khoản khi đang tải tệp; kết quả là quá trình tải lập tức dừng và toàn bộ hàng đợi cùng dữ liệu tệp được xoá sạch khỏi máy; Workspace: Cá nhân; Giao diện: tối.
- [ ] Trên iPhone, mở xem trước một tệp .docx lần đầu; kết quả là máy chủ chuyển đổi và hiển thị tệp PDF xem trước tương ứng; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, mở xem trước tệp .xlsx; kết quả là trình xem mở chế độ bảng tính xem dữ liệu trực tiếp mà không qua chuyển đổi PDF; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, mở xem trước tệp nén .zip; kết quả là trình xem hiển thị thông báo không hỗ trợ loại tệp này và cung cấp nút Tải xuống; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, truy cập xem trước tệp mà mình không có quyền đọc; kết quả là nhận lỗi từ chối truy cập 403 và tệp không được chuyển đổi; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, mở xem trước lại một tệp đã từng mở trước đó; kết quả là bản PDF lưu sẵn được hiển thị ngay mà không cần máy chủ chuyển đổi lại; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, đồng thời mở xem trước một tệp mới; kết quả là tệp được xử lý một lần chuyển đổi duy nhất và trả về bản xem trước cho các thiết bị; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, xoá vĩnh viễn một tệp trong thùng rác; kết quả là bản xem trước PDF của tệp đó trên máy chủ cũng được dọn sạch; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, xem trước tài liệu văn phòng khi dịch vụ chuyển đổi Gotenberg đang ngắt; kết quả là ứng dụng báo tạm thời không khả dụng và gợi ý tải xuống, các tệp khác vẫn hoạt động; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, mở xem trước một tệp tài liệu bị hỏng hai lần trong vòng 10 phút; kết quả là đều nhận thông báo lỗi chuyển đổi và máy chủ chỉ gọi chuyển đổi một lần duy nhất; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, mở xem trước khi máy chủ đang bận xử lý nhiều tệp cùng lúc; kết quả là nhận thông báo máy chủ đang bận và tệp không bị đánh dấu là lỗi vĩnh viễn; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, mở thư mục chứa tệp script danh sách phát đổi đuôi thành .mp4; kết quả là ứng dụng từ chối sinh ảnh thu nhỏ, hiển thị icon loại tệp và không thực thi mã nguy hiểm; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, xem danh sách tệp có ảnh JPEG dạng progressive kích thước 4000x4000 quá giới hạn bộ nhớ giải mã; kết quả là hiển thị icon thay vì ảnh thu nhỏ và không làm treo ứng dụng; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, xem thẻ Truy cập nhanh hoặc danh sách có ảnh chụp 4000x3000; kết quả là hiển thị ảnh thu nhỏ tỉ lệ chuẩn 320x240 được lưu trong bộ đệm; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, cố tình tải ảnh thu nhỏ của tệp mà mình không có quyền đọc; kết quả là nhận mã lỗi 403 và không được phục vụ ảnh; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, khi tệp được tải lên phiên bản nội dung mới; kết quả là ảnh thu nhỏ được cập nhật theo nội dung mới; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, hiển thị tệp .zip; kết quả là hiển thị icon loại tệp nén chuẩn; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, tải đồng thời danh sách nhiều ảnh khiến giới hạn sinh ảnh thu nhỏ bị đầy; kết quả là ứng dụng nhận phản hồi thử lại sau và tự tải lại khi máy chủ rảnh; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, xoá vĩnh viễn tệp ảnh; kết quả là toàn bộ các kích thước ảnh thu nhỏ được dọn sạch khỏi bộ nhớ máy chủ; Workspace: Tổ chức; Giao diện: tối.
- [ ] [cảm ứng: pinch] [cảm ứng: double-tap] [cảm ứng: row swipe] Trên iPhone, mở xem một ảnh trong trình xem tệp; kết quả là chụm 2 ngón để phóng to tới 4x, chạm đúp để chuyển đổi giữa vừa khung và 2x, kéo 1 ngón khi đang phóng to để lia ảnh; vuốt sang trái/phải ở mức vừa khung để chuyển tệp trước/sau (chuyển khi nhấc tay, không bám tay 1:1); kéo xuống quá 25% để đóng trình xem; Workspace: Tổ chức; Giao diện: sáng.

---

## 8. Danh bạ

- [ ] Trên iPhone, người dùng chỉ nằm trong nhóm đồng nghiệp phòng ban hoặc vai trò giới hạn mà không nằm trong Members của workspace; kết quả là không được coi là thành viên và bị từ chối truy cập Danh bạ; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, người đã bị xoá khỏi workspace mở Danh bạ; kết quả là mọi truy cập đều bị từ chối quyền; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone trong workspace chưa đặt quy tắc giới hạn nào, mở Danh bạ; kết quả là nhìn thấy tất cả phòng ban, người không thuộc phòng ban và chủ sở hữu; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, cố tình đọc danh bạ của workspace khác; kết quả là bị từ chối quyền truy cập do khác policy class; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, thành viên thông thường cố gắng quản trị danh bạ hoặc đọc nhánh ngang của phòng ban khác; kết quả là bị từ chối quyền; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, chủ sở hữu hoặc người có vai trò quản lý mở Danh bạ; kết quả là nhìn thấy toàn bộ mọi người trong công ty, bao gồm cả các phòng ban bị ẩn; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, chủ sở hữu nằm trong một phòng ban bị giới hạn mở Danh bạ; kết quả là vẫn nhìn thấy toàn bộ thành viên nhờ quyền quản trị danh bạ; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, thành viên phòng ban mở xem danh sách phòng ban; kết quả là không nhìn thấy phòng ban bị ẩn; trong khi thành viên của chính phòng ban bị ẩn mở Danh bạ thì nhìn thấy phòng ban của mình; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, người thuộc phòng ban cha mở danh sách phòng ban; kết quả là không xem được phòng ban con bị ẩn bên dưới; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, sau khi phòng ban được quản trị viên bỏ ẩn; kết quả là toàn bộ thành viên trong tổ chức lại nhìn thấy phòng ban đó trong Danh bạ; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, người dùng bị giới hạn phạm vi trong phòng ban của mình mở Danh bạ; kết quả là luôn nhìn thấy danh sách các chủ sở hữu workspace; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, thành viên thông thường mở Danh bạ khi chủ sở hữu nằm trong phòng ban bị ẩn; kết quả là không thấy phòng ban bị ẩn nhưng vẫn thấy chủ sở hữu đó; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, người giữ vai trò bị giới hạn mở Danh bạ; kết quả là bị chặn không xem được các phòng ban ngang hàng hay người không có phòng ban; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, người bị giới hạn mà không thuộc phòng ban nào mở Danh bạ; kết quả là chỉ nhìn thấy các chủ sở hữu và chính bản thân mình; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, người bị giới hạn mở Danh bạ của phòng mình; kết quả là nhìn thấy đầy đủ các đồng nghiệp trong phòng và các phòng ban con trực thuộc; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, người bị giới hạn cố mở phòng ban cha hoặc phòng ban anh em; kết quả là bị từ chối quyền truy cập; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, khi cả một phòng ban bị gắn giới hạn; kết quả là các thành viên trong phòng đó và phòng ban con đều chỉ thấy phòng mình, các phòng ban khác không bị ảnh hưởng; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, chuyển một nhân viên từ phòng ban bị giới hạn sang phòng ban bình thường; kết quả là nhân viên đó lập tức xem lại được toàn bộ cây danh bạ mở; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, người bị giới hạn nằm trong một phòng ban bị ẩn; kết quả là xem được cây nội bộ của phòng ban bị ẩn đó và không xem được cây danh bạ mở bên ngoài; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, kiểm tra hiển thị danh bạ khi tải từng mục và khi tải hàng loạt; kết quả là kết quả hiển thị danh sách người hoàn toàn đồng nhất; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, khi phòng ban bị giới hạn trong lúc đang mở ứng dụng; kết quả là danh bạ tự động làm mới và chuyển sang chế độ giới hạn ngay lập tức; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, khi một lệnh cấm giới hạn mới được tạo trên máy chủ; kết quả là màn hình Danh bạ đang mở cập nhật thu hẹp phạm vi hiển thị đúng quy tắc mới; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, khi quản trị viên gỡ bỏ quy tắc giới hạn; kết quả là người dùng lập tức nhìn thấy lại toàn bộ các phòng ban trong tổ chức; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, nhân viên thuộc phòng ban ẩn bị xoá khỏi workspace; kết quả là hoàn toàn không còn quyền truy cập bất kỳ mục nào của danh bạ; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, tạo thêm phòng con dưới phòng ẩn; kết quả là phòng con tự động kế thừa trạng thái ẩn và người ngoài vẫn không xem được; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, xoá một phòng ban ẩn đang có người trực thuộc; kết quả là thao tác bị từ chối; khi phòng đã hết người thì xoá thành công; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, thêm người vào một phòng ban mới; kết quả là các nút danh bạ được tự động sinh và thành viên nhìn thấy đồng nghiệp cùng phòng; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, quản trị viên gán nhân viên vào một phòng ban ẩn; kết quả là thao tác thành công và nhân viên đó xuất hiện trong phòng ẩn; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, người dùng không hợp lệ gọi yêu cầu lấy phạm vi danh bạ; kết quả là nhận lỗi từ chối quyền PermissionDenied; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, kiểm tra phản hồi thông tin danh bạ khi gửi yêu cầu không kèm danh tính người dùng xác thực; kết quả là bị từ chối Unauthenticated; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, người bị giới hạn mở Danh bạ; kết quả là hiển thị dòng "Chỉ phòng của bạn", danh sách chỉ có phòng mình và các phòng con, người không phòng bị ẩn; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, nếu dịch vụ chính sách hoặc tra cứu danh bạ gặp sự cố; kết quả là ứng dụng báo lỗi Unavailable (503) và không bao giờ mở rộng phạm vi hiển thị lộ thông tin; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, thành viên phòng mở xem danh sách phòng ban và số lượng thành viên; kết quả là phòng ẩn và số đếm của người trong phòng ẩn hoàn toàn biến mất (chủ sở hữu trong phòng ẩn vẫn hiện); Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, người bị giới hạn xem danh sách thành viên; kết quả là chỉ nhận được người trong phòng mình, các phòng con, chủ sở hữu và chính mình; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, thành viên thông thường truy cập trang cài đặt quy tắc danh bạ; kết quả là nhận lỗi 403 và không thể can thiệp quy tắc; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, quản trị viên cố áp quy tắc giới hạn lên phòng ban của workspace khác hoặc nhóm hệ thống; kết quả là nhận mã lỗi 404; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, ẩn rồi bỏ ẩn một phòng ban con; kết quả là phòng ban con trở về đúng vị trí phòng cha ban đầu; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, mở danh bạ của workspace cũ chưa có cây danh bạ; kết quả là mọi thành viên nhìn thấy toàn bộ mọi người mà không có quy tắc giới hạn nào; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, tìm kiếm và duyệt danh bạ khi có phòng ban bị ẩn; kết quả là không tìm thấy bất kỳ ai thuộc phòng ban bị ẩn theo cả tên lẫn email; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, người bị giới hạn lọc và tìm kiếm trong Danh bạ; kết quả là chỉ tìm thấy người trong phòng ban của mình và các chủ sở hữu; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, khi dịch vụ danh bạ không phản hồi; kết quả là ứng dụng hiển thị trạng thái lỗi 503, tuyệt đối không dùng danh sách đầy đủ làm phương án dự phòng; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, xem thẻ của chủ sở hữu nằm trong phòng ban bị ẩn; kết quả là dòng phòng ban bị để trống, tìm kiếm theo tên phòng ẩn đó không trả về kết quả; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, tài khoản ở trạng thái vô hiệu hoá hoặc đã bị gỡ khỏi workspace mở Danh bạ; kết quả là nhận lỗi từ chối 403; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, xem các vị trí lưu công việc cũ của người bị ẩn hoặc người đã rời tổ chức; kết quả là tên hiển thị và avatar vẫn xuất hiện bình thường dù Danh bạ không còn liệt kê họ; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, yêu cầu tra cứu thông tin người dùng từ workspace khác; kết quả là hệ thống từ chối và không trả về tên; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, gửi yêu cầu tra cứu đồng thời hơn 200 danh tính; kết quả là hệ thống từ chối với mã lỗi 400; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, khi có tin nhắn trực tiếp gần đây với một người bị danh bạ ẩn và người đó đang online; kết quả là mục Thường liên lạc và Đang trực tuyến đều không hiển thị người đó; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, mở thẻ hồ sơ của một đồng nghiệp; kết quả là mục "N nhóm chung" chỉ liệt kê các nhóm mà cả hai người cùng tham gia và người xem còn quyền đọc, không liệt kê tin nhắn trực tiếp hay nhóm ngoài; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, mở thẻ hồ sơ của một đồng nghiệp; kết quả là mục "Tệp đã chia sẻ với bạn" chỉ liệt kê các tệp do người đó sở hữu đã chia sẻ với mình mà mình còn quyền xem, không hiển thị tệp trong thùng rác; Workspace: Tổ chức; Giao diện: sáng.
- [ ] [cảm ứng: row swipe] Trên iPhone, trong danh sách người của một phòng ban hoặc nhóm, vuốt hàng sang trái; kết quả là lộ ra hai nút Nhắn tin (màu accent) và Gọi (màu success); Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone trong workspace cá nhân, mở Danh bạ; kết quả là không có Phòng ban, Vai trò hay Phòng của tôi; chỉ hiển thị danh mục Thành viên, Nhóm của tôi và danh sách Đang trực tuyến; Workspace: Cá nhân; Giao diện: sáng.

---

## 9. Phê duyệt

- [ ] [cảm ứng: row swipe] Trên iPhone (PWA cài đặt, 390pt), vuốt ngang hết cỡ một hàng đề nghị trong tab "Chờ tôi"; kết quả là không có hành động duyệt hay từ chối nào tự động thực thi, hai nút Duyệt… (phải, rộng 92px) và Từ chối… (trái) chỉ lộ ra để chờ người dùng chạm vào xác nhận; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone, trong phân hệ Phê duyệt, gõ từ khoá tìm kiếm đề nghị; kết quả là kết quả trả về đúng các đề nghị khớp tên mẫu hoặc tên người gửi mà người dùng được quyền xem; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, trong workspace tổ chức mở Cài đặt Phê duyệt từ Tôi hoặc dấu ⋯; kết quả là hiển thị các công tắc thông báo đề nghị chờ duyệt và kết quả duyệt; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone trong workspace cá nhân, kiểm tra thanh đáy và Thêm; kết quả là không có phân hệ Phê duyệt, truy cập /settings/approval tự động chuyển hướng về /settings; Workspace: Cá nhân; Giao diện: tối.

---

## 10. Lịch

- [ ] Trên iPhone, lưu các tuỳ chọn thứ bắt đầu tuần, kiểu xem mặc định, thời lượng mặc định và danh sách lịch hiển thị trong Cài đặt Lịch; kết quả là các thiết lập này lưu độc lập và không bị ghi đè khi lưu múi giờ hay giờ làm việc trên máy tính; Workspace: Tổ chức; Giao diện: sáng.
- [ ] [cảm ứng: row swipe] Trên iPhone, ở màn hình Lịch, vuốt ngang dải tuần hoặc lưới tháng quá 48px hoặc vuốt nhanh; kết quả là chuyển sang tuần hoặc tháng tiếp theo/trước đó tương ứng; Workspace: Tổ chức; Giao diện: tối.
- [ ] Trên iPhone, chạm vào dấu ⋯ trên tiêu đề phân hệ Lịch; kết quả là dòng đầu tiên là "Cài đặt Lịch" mở đúng trang cài đặt của Lịch; Workspace: Tổ chức; Giao diện: sáng.
- [ ] Trên iPhone trong workspace cá nhân, mở phân hệ Lịch; kết quả là chỉ hiển thị lịch cá nhân, không có lịch phòng ban hay lịch tổ chức; Workspace: Cá nhân; Giao diện: tối.
