---
title: "Thiết kế phase 01: workspace cá nhân và workspace tổ chức"
type: design-report
plan: plans/261010-2258-personal-workspace (phase 01)
created: 2026-10-11
status: chờ người dùng duyệt
---

# Thiết kế: workspace cá nhân và workspace tổ chức

Mockup: `design/mockups/personal-workspace.html` (có nút đổi Sáng/Tối/Theo hệ thống, ô Giảm chuyển động; `?theme=dark` mở sẵn chế độ tối).
Ảnh chụp: `plans/reports/design-261011-1030-personal-workspace/` (cả trang + từng màn, 390 và 1440, sáng và tối, hộp xác nhận thật lúc đang mở).
Chưa sửa DESIGN.md, mockup cũ hay code.

## Các màn trong mockup (hai loại đặt cạnh nhau)
1. Sidebar máy tính: bản cá nhân chỉ có Tin nhắn, Tài liệu, Danh bạ; dòng phụ dưới tên ghi "Workspace cá nhân".
2. Điện thoại: thanh đáy bản cá nhân là Tin nhắn · Tài liệu · **Danh bạ** · Thêm; sheet Thêm còn Thông báo · Quản trị · Cài đặt.
3. Văn bản không có nút Gửi duyệt; menu + của chat không có Đề nghị phê duyệt.
4. Quản trị → Vai trò: không có vùng Phê duyệt và Tài sản, ma trận quyền mất cột Duyệt.
5. Cài đặt → Workspace: thẻ "Loại workspace" cho chủ sở hữu (có nút) và cho thành viên khác (không có nút, có câu giải thích kèm tên chủ sở hữu).
6. Hộp xác nhận: tĩnh để duyệt, và bản thật (`<dialog>`) có trạng thái đang chuyển và lỗi mạng.
7. Sau khi chuyển: thẻ chuyển sang dạng tổ chức (ghi ai chuyển, lúc nào, có lối tắt đặt quyền), toast báo xong, sidebar chèn thêm 2 mục, tab thứ ba trên thanh đáy đổi lại thành Phê duyệt.

## Quyết định thiết kế
- **Mục không có thì không vẽ.** Không để mục mờ, mục khoá hay chỗ trống. Mở `/assets` hoặc `/approval` thì về trang Tin nhắn.
- **Cho biết loại workspace qua dòng phụ dưới tên** ("Workspace cá nhân"), dòng này dùng ở sidebar, sheet và danh sách đổi workspace. Workspace tổ chức vẫn ghi tên tổ chức. Không thêm badge hay màu riêng.
- **Danh bạ ở tab thứ ba không có số đếm.** Ở workspace cá nhân thanh đáy không lấy số chờ duyệt. Danh bạ đã lên thanh đáy nên bỏ khỏi sheet Thêm.
- **Văn bản cá nhân không có nút primary.** Nút Chia sẻ giữ kiểu secondary, không được nâng lên primary để lấp chỗ. Trạng thái văn bản (Bản nháp, Đang dùng, Lưu trữ) không phải phê duyệt nên vẫn giữ.
- **Quản trị chỉ vẽ những vùng server trả về** (`GET /permission-areas`). Giao diện không tự lọc theo loại workspace. Cột Duyệt bị ẩn vì không còn vùng nào dùng đến nó.
- **Thẻ loại workspace** đặt dưới Tên và Mô tả, trên dòng Quản trị. Nút "Chuyển thành workspace tổ chức" là secondary vì Lưu thay đổi vẫn là nút primary duy nhất. Chỉ người có vai trò **Chủ sở hữu** thấy nút. Người có quyền Quản lý nhưng không phải chủ sở hữu thì thấy như thành viên.
- **Hộp xác nhận** là modal 520 và liệt kê đúng ba thứ sẽ xuất hiện: Tài sản, Phê duyệt, Duyệt tài liệu. Hộp có câu "Tin nhắn, tài liệu, thành viên và vai trò giữ nguyên." và một callout cảnh báo có chữ "Không chuyển ngược lại được.". Không bắt gõ lại tên vì không có gì bị xoá. Khi mở, focus nằm ở nút Huỷ. Trên điện thoại hai nút xếp dọc, rộng hết khung.
- **Sau khi chuyển**, hai mục mới trên sidebar cao dần từ 0 lên 36 trong 220ms, nền `--color-accent-wash` tan trong 2400ms. Dùng màu accent vì thay đổi đến từ hệ thống, không phải từ một người. Bật giảm chuyển động thì hiện ngay. Người khác đang mở workspace thấy toast có tên người chuyển.
- **Kiểm chứng:** Playwright ở 390x844 và 1440x900, sáng và tối: không lỗi console hay lỗi trang, không tràn ngang ở 390 (scrollWidth = clientWidth), không có chuỗi dạng UUID, mở và chuyển trong hộp thật đều chạy (focus vào Huỷ, toast hiện ra). Mockup không có hex hay rgba rời, chỉ dùng token.

## Đề xuất văn bản cho DESIGN.md (chưa áp)

Đề xuất thêm một mục mới **§5.1 Loại workspace** ngay sau §5, và sửa vài câu ở §5 và §6:

> ### Loại workspace: cá nhân và tổ chức (mockup `design/mockups/personal-workspace.html`)
>
> Có hai loại workspace. **Cá nhân** là workspace hệ thống tự tạo khi đăng nhập lần đầu. **Tổ chức** được tạo
> bằng "Tạo workspace" hoặc từ Google theo tên miền. Server quyết định loại và gửi kèm workspace. Giao diện chỉ
> đọc loại đó, không tự đoán từ tên hay số thành viên.
>
> - **Workspace cá nhân không có Tài sản, Phê duyệt và duyệt tài liệu.** Ba thứ này không hiện ở bất cứ đâu:
>   sidebar (cả rail), thanh đáy, sheet Thêm, nút Gửi duyệt và menu Thêm của văn bản, mục Đề nghị phê duyệt
>   trong menu + của chat, các loại Phê duyệt và Tài sản trong Thông báo, các vùng Phê duyệt và Tài sản cùng
>   cột Duyệt trong Quản trị. Mục không có thì không vẽ: không mục mờ, không ổ khoá, không chỗ trống. Mở
>   `/assets` hoặc `/approval` thì về Tin nhắn. Workspace tổ chức giữ nguyên như hiện nay.
> - **Dòng phụ dưới tên workspace** (sidebar, sheet Thêm, danh sách đổi workspace): workspace cá nhân ghi
>   "Workspace cá nhân", workspace tổ chức ghi tên tổ chức.
> - **Sidebar cá nhân:** Tin nhắn · Tài liệu · Danh bạ. Phần đáy giữ nguyên.
> - **Thanh đáy cá nhân:** Tin nhắn · Tài liệu · **Danh bạ** · Thêm. Tab Danh bạ dùng icon `Users`, không có số
>   đếm, và không lấy số chờ duyệt. Sheet Thêm còn Thông báo · Quản trị · Cài đặt. Nút Thêm sáng khi đang ở
>   Quản trị hoặc Cài đặt. Đổi sang workspace khác loại thì tab thứ ba đổi theo bằng fade 160ms.
> - **Thông báo cá nhân:** trạng thái trống dùng hành động "Mở Tin nhắn" thay cho "Xem đề nghị của bạn".
> - **Cài đặt → Workspace, thẻ "Loại workspace"** (Panel `--color-raised`, đặt dưới Mô tả, trên dòng Quản
>   trị): ô icon 40 (`User` cho cá nhân, `Building` cho tổ chức), tên loại viết 600, một câu nói workspace có
>   gì và không có gì. Ở workspace cá nhân, người có vai trò **Chủ sở hữu** thấy Button secondary "Chuyển thành
>   workspace tổ chức". Người khác (kể cả người có quyền Quản lý) không thấy nút, chỉ thấy một dòng có ổ khoá:
>   "Chỉ chủ sở hữu, [PersonChip], mới chuyển được workspace này thành workspace tổ chức." Ở workspace tổ chức
>   thẻ chỉ để đọc. Không có đường chuyển ngược.
> - **Hộp xác nhận chuyển** (Modal 520): tiêu đề "Chuyển thành workspace tổ chức?". Câu dẫn "Mọi người trong
>   [tên] sẽ thấy thêm:" rồi ba hàng (ô icon 32 nền wash warning, tên in đậm, một dòng mô tả): **Tài sản**,
>   **Phê duyệt**, **Duyệt tài liệu**. Tiếp theo là câu "Tin nhắn, tài liệu, thành viên và vai trò giữ
>   nguyên." và một callout `--color-warning-wash` có chữ "**Không chuyển ngược lại được.** Workspace tổ chức
>   không trở về workspace cá nhân." Nút: ghost "Huỷ" và primary "Chuyển thành tổ chức". Khi mở, focus nằm ở
>   Huỷ. Esc, Huỷ hoặc Đóng thì đóng hộp và trả focus về nút Chuyển. Đang chuyển: spinner thay chữ, nút giữ
>   chiều rộng, Huỷ bị tắt. Lỗi: hộp vẫn mở, toast nói cách sửa. Dưới 480px hai nút xếp dọc, rộng hết khung,
>   primary ở trên.
> - **Sau khi chuyển:** toast "Đã chuyển thành workspace tổ chức". Thẻ ghi **ai** đã chuyển và **khi nào**
>   (theo §8), kèm Button "Đặt quyền Phê duyệt và Tài sản", bấm vào mở Quản trị → Vai trò. Mục Phê duyệt và
>   Tài sản được chèn vào sidebar (cao 0 → 36 trong 220ms, nền `--color-accent-wash` tan trong 2400ms; giảm
>   chuyển động thì hiện ngay), và tab thứ ba của thanh đáy đổi lại thành Phê duyệt. Người khác đang mở
>   workspace nhận cùng thay đổi qua realtime, kèm toast "[tên] đã chuyển workspace này thành workspace tổ
>   chức". Dữ liệu Tài sản và Phê duyệt có từ trước sẽ hiện lại.

Sửa các câu sẵn có:
- §5, gạch đầu dòng **Sidebar**: thêm sau "5 mục chính …": "(workspace cá nhân: 3 mục, xem §5.1)".
- §5, đoạn **Điều hướng di động**: thêm sau "3 tab có nhãn **Tin nhắn · Tài liệu · Phê duyệt**": "(workspace cá nhân: Danh bạ thay Phê duyệt, §5.1)". Trong đoạn *Sheet Thêm*: thêm "workspace cá nhân bỏ Tài sản và Danh bạ".
- §6 **NotificationPanel**: thêm "Ở workspace cá nhân, hành động của trạng thái trống là 'Mở Tin nhắn'".

## Câu hỏi mở cho người dùng
1. **Tên khi chuyển:** workspace cá nhân thường mang tên mặc định "Không gian của bạn". Có nên thêm ô "Tên
   workspace" vào hộp xác nhận (bắt buộc nếu đang là tên mặc định) không? Hiện mockup **không** có ô này, vì
   phạm vi đã chốt chỉ gồm việc chuyển loại. Người dùng vẫn đổi tên được ở ngay thẻ phía trên.
2. **Ai được chuyển:** mockup chỉ cho **Chủ sở hữu**. Nếu workspace cá nhân có nhiều chủ sở hữu thì ai trong
   số họ cũng chuyển được. Như vậy có đúng ý không, hay người có quyền Quản lý cũng được chuyển?
3. **Dữ liệu ẩn hiện lại:** tài sản và đề nghị có từ trước, đã bị ẩn khi workspace là cá nhân, sẽ hiện lại sau
   khi chuyển. Có cần nói điều này trong hộp xác nhận không? Hiện mockup không nói, vì phần lớn workspace cá
   nhân không có dữ liệu loại này.
4. **Nhãn dòng phụ:** "Workspace cá nhân" hay ngắn hơn là "Cá nhân"? Mockup dùng "Workspace cá nhân".

---

## Bổ sung 2026-10-11: Tạo workspace có chọn loại, hộp chuyển theo câu trả lời

Người dùng đã trả lời 4 câu hỏi mở (11/10): (1) mô hình mới: mỗi tài khoản mới có sẵn một workspace cá nhân, kể cả
Google công ty (vd @vnpay.vn); nếu một tổ chức đã sở hữu tên miền đó thì người dùng vào thêm tổ chức ấy làm thành viên.
Đăng nhập không bao giờ tạo tổ chức. Workspace khác thì tạo bằng "Tạo workspace" và chọn Cá nhân hoặc Tổ chức; một người có thể
có nhiều workspace cá nhân. (2) Chỉ Chủ sở hữu được chuyển (giữ như đã vẽ). (3) Hộp chuyển phải nói tài sản và đề nghị đã bị ẩn sẽ hiện lại;
không có ô đổi tên. (4) Dòng phụ là "Workspace cá nhân" (giữ như đã vẽ).

Mockup: `design/mockups/personal-workspace.html`, thêm §8, §9; sửa đầu trang, §1 (ghi chú), §6, §7 (ghi chú).
Ảnh mới (`plans/reports/design-261011-1030-personal-workspace/`, tiền tố `create-`), 390 và 1440, sáng và tối:
`create-screen-*` (§8 đủ trạng thái), `create-switcher-*` (§9), `create-confirm-*` (§6 tĩnh), `create-dialog-live-*`
(hộp thật đang mở), `create-desk-error-*` (form máy tính ở trạng thái thiếu tên).

### §8 Tạo workspace: chọn loại
- **Bố cục:** giữ auth shell như `auth-motion.html` (brand pane trên `--color-sunk` từ 1024, form trên base; điện thoại chỉ có
  logo ở đầu). Thứ tự trong form: Quay lại · tiêu đề "Tạo workspace" + câu dẫn "Mỗi workspace có thành viên, tin nhắn và tài liệu
  riêng." (bỏ câu cũ có "duyệt đề nghị", vì workspace cá nhân không có duyệt) · hộp lỗi (khi có) · **Loại workspace** · ghi chú một
  chiều · **Tên workspace** (giữ 80 ký tự, có bộ đếm, gợi ý "Mời người sau, trong Quản trị.") · nút primary rộng hết khung, cao 44.
- **Lựa chọn loại:** `fieldset` + `legend`, hai radio thật, cả thẻ là vùng bấm (cao hơn 44), ↑↓ để đổi. Thẻ đang chọn dùng nền `--color-accent-wash` và
  viền trong 1.5 accent, chấm radio đặc. Thẻ nào cũng có icon (`User` / `Building`, khớp thẻ Loại workspace ở §5), tên loại, một câu nói dùng cho ai, và danh sách phân hệ:
  - **Cá nhân:** "Cho việc của bạn, hoặc một nhóm nhỏ không cần quy trình duyệt." · Tin nhắn · Tài liệu · Danh bạ.
  - **Tổ chức:** "Cho phòng ban, công ty: có quy trình duyệt và quản lý tài sản." · "Như Cá nhân, thêm:" **Tài sản · Phê duyệt · Duyệt tài liệu** (in đậm, chỉ liệt kê phần thêm).
  - Ghi chú dưới hai thẻ (`aria-describedby` của fieldset): "Workspace cá nhân chuyển thành workspace tổ chức được về sau, trong Cài đặt. Không chuyển ngược lại."
- **Mặc định: Cá nhân.** Lý do:
  (a) Cá nhân là lựa chọn đổi được (chuyển lên tổ chức chỉ một bước ở Cài đặt), còn tổ chức thì không quay lại. Mặc định nên là
  lựa chọn an toàn, cùng nguyên tắc với focus vào Huỷ ở hộp chuyển.
  (b) Không có trạng thái "chưa chọn", nên không cần thêm một lỗi cho người dùng phải sửa.
  (c) Chọn Tổ chức thì thấy ngay ba phân hệ thêm, nên người cần quy trình duyệt khó bỏ sót.
  Đánh đổi: người tạo workspace cho phòng ban phải bấm thêm một lần. Bấm sai cũng sửa được về sau, còn ngược lại thì không.
- **Phản hồi theo lựa chọn:** gợi ý tên ("Ví dụ: Việc nhà, Nhóm chạy bộ" / "Ví dụ: Tổ Đối soát") và nhãn nút ("Tạo workspace cá nhân" /
  "Tạo workspace tổ chức") đổi theo loại đang chọn.
- **Trạng thái** (5 khung điện thoại + bản máy tính chạy được): mặc định · chọn Tổ chức · chưa có tên (viền danger, "Nhập tên workspace.",
  `aria-invalid`, focus về ô) · lỗi mạng (hộp báo đỏ ở đầu form "Không tạo được workspace. Kiểm tra kết nối rồi thử lại.", giữ loại và tên đã nhập) ·
  đang tạo (spinner, nút giữ chiều rộng, `fieldset disabled`, thẻ không chọn mờ 55%, ô tên `readonly`, Quay lại tắt). Tạo xong thì vào thẳng Tin nhắn của workspace mới.
  Server trả 400 thì câu của server hiện dưới ô tên, như code hiện nay.
- **Focus:** bỏ autofocus của ô tên. Trên máy tính, focus đầu tiên là thẻ đang chọn; trên điện thoại không tự focus.
- **Steps 3/3** bỏ: tài khoản mới luôn có workspace nên không ai tới màn này khi chưa có workspace nào. Nhánh "chưa xác minh email" và "tài khoản số điện thoại" giữ nguyên.

### §9 Lối vào trong danh sách đổi workspace
- Hiện nay chỉ màn Chọn workspace có "Tạo workspace mới". Thêm hàng **Tạo workspace** ở cuối danh sách đổi workspace, sau divider:
  máy tính là menu dưới tên workspace ở sidebar (rộng 300, hàng cao 44, ô chữ cái 28); điện thoại là sheet "Đổi workspace" (hàng cao 60, ô chữ cái 40).
  Ô `Plus` nền `--color-sunk`, chữ 600, không dùng primary.
- Hàng nào cũng có dòng phụ nói loại ("Workspace cá nhân" / tên tổ chức; điện thoại thêm " · vai trò"), nên hai workspace cá nhân
  ("Không gian của bạn", "Nhóm chạy bộ") đứng cạnh nhau vẫn không lẫn. Code hiện nay ở menu máy tính chỉ hiện tên.
- Quay lại từ Tạo workspace trả về nơi người dùng vừa rời (workspace đang mở, hoặc Chọn workspace).

### §6 Hộp xác nhận chuyển (theo câu trả lời 3, 4)
- Thêm câu thứ hai dưới "Tin nhắn, tài liệu, thành viên và vai trò giữ nguyên.":
  **"Tài sản và đề nghị phê duyệt có từ trước, đang bị ẩn, sẽ hiện lại cho người có quyền xem."** Câu luôn có mặt (đúng cả khi
  không có dữ liệu loại đó, và giao diện không phải hỏi server trước). "cho người có quyền xem" vì vai trò vẫn quyết định ai thấy.
- Không có ô đổi tên (đã khẳng định trong mô tả §6). Dòng phụ "Workspace cá nhân" giữ nguyên ở mọi nơi.

### Đề xuất cho DESIGN.md (chưa áp, cộng thêm vào đề xuất §5.1 ở trên)
- Sửa câu mở đầu §5.1: "**Cá nhân**: mỗi tài khoản mới có sẵn một workspace cá nhân; workspace khác tạo bằng Tạo workspace và người tạo chọn
  loại. Đăng nhập không tạo tổ chức; email có tên miền thuộc một tổ chức thì người dùng vào thêm tổ chức đó làm thành viên."
- Thêm gạch đầu dòng **Tạo workspace** (tóm §8: fieldset hai thẻ, mặc định Cá nhân, ghi chú một chiều, nhãn nút theo loại, các trạng thái).
- Thêm vào gạch đầu dòng Hộp xác nhận: câu "Tài sản và đề nghị phê duyệt có từ trước…".
- §5 (Sidebar, sheet Đổi workspace): hàng cuối "Tạo workspace" + dòng phụ nói loại.

### Kiểm chứng
Playwright (Chromium) ở 390x844 và 1440x900, sáng và tối:
- 0 lỗi console, 0 lỗi trang. Một lần chạy gặp `ERR_NETWORK_CHANGED` khi tải Google Fonts, chạy lại thì sạch.
- Ở 390, `scrollWidth = clientWidth`.
- Không có chuỗi dạng UUID, kể cả trong hộp thật.
- Ở 390, mọi nút, ô và thẻ chọn trong §8 và sheet §9 đều ≥ 44. Ô tên 16px.
- Tương tác chạy đúng:
  - chọn Tổ chức thì nút đổi nhãn và ô tên đổi gợi ý;
  - bấm Tạo khi trống thì ô có `aria-invalid` và nhận focus;
  - nhánh lỗi mạng hiện hộp báo;
  - tạo thành công thì báo trong demo;
  - hộp chuyển thật có câu "sẽ hiện lại".

Mockup chỉ dùng token. Trong lúc kiểm chứng đã sửa một lỗi: tiêu đề sheet §9 bị xuống dòng vì quy tắc `.msheet .ibtn` có sẵn.

### Câu hỏi mở mới
1. **Workspace tổ chức do một người tạo thuộc tổ chức nào?** Hiện dòng phụ của tổ chức là tên tổ chức (NovaPay). Workspace tổ chức tạo từ
   "Tạo workspace" (hay chuyển từ cá nhân) chưa thuộc tổ chức nào thì dòng phụ ghi "Workspace tổ chức" (như §7). Nếu người tạo là thành viên của một tổ chức
   (vd vnpay.vn), workspace mới có tự thuộc tổ chức đó không, hay cần thêm ô chọn tổ chức trong form? Mockup chưa có ô này.
2. **Tên workspace cá nhân tự tạo lúc đăng ký:** vẫn là "Không gian của bạn"? Khi một người có nhiều workspace cá nhân, tên mặc định này dễ trùng
   với workspace tự đặt tên. Có muốn đổi thành "Không gian của [tên]" không?
3. **Trường hợp ít gặp ở `CreateWorkspaceScreen`:** tài khoản số điện thoại và tài khoản chưa xác minh email vẫn bị chặn tạo. Theo mô hình mới thì họ có được workspace cá nhân
   tự động lúc đăng ký không? Nếu có, chỉ màn Tạo thêm mới bị chặn.
