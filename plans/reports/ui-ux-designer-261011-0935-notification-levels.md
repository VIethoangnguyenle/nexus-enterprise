---
title: "Thiết kế phase 09: Mức thông báo theo cuộc trò chuyện và Thread có tin mới"
type: design-report
plan: plans/261010-1559-chat-inline-threads (phase 09)
created: 2026-10-11
status: hoàn thành
---

# Báo cáo thiết kế: Mức thông báo theo cuộc trò chuyện & "Thread có tin mới"

- **Mockup HTML:** `design/mockups/notification-levels.html` (tái sử dụng `design/mockups/tokens.css`, hỗ trợ Sáng / Tối / Theo hệ thống, Giảm chuyển động, Mạng chậm 1,5s, Giả lập lỗi lưu).
- **Bộ ảnh chụp kiểm chứng:** `plans/reports/design-261011-0935-notification-levels/` (36 ảnh chụp gồm đầy đủ 1440x900 và 390x844, light và dark, các trạng thái menu, sheet, đang lưu và toast).

---

## 1. Các quyết định thiết kế (Design Decisions)

### 1.1. Vị trí điều khiển (Where the control lives on desktop & phone)
- **Trên máy tính (Desktop):**
  - **Header nhóm/cuộc trò chuyện:** Tên nhóm là nút menu dropdown (`button.ttl`, min-height 36px, `aria-haspopup="menu"`, chevron 16 quay 180° khi mở). Menu rộng 344px, nhóm đầu tiên là "Thông báo" gồm ba `role="menuitemradio"`:
    1. **Tất cả** (`i-bell-ring`)
    2. **Tin chính và thread đang theo dõi** (`i-bell`, kèm tag "Mặc định")
    3. **Tắt** (`i-bell-off`)
    Mỗi mục có tiêu đề in đậm và một dòng mô tả 12px mờ (`aria-describedby`).
  - **Menu ⋯ ở hàng điều hướng (Navigator row menu):** Đóng vai trò là **lối tắt bật/tắt nhanh**, không lồng submenu 3 mức:
    - Nếu đang ở mức khác Tắt → hiện mục **Tắt thông báo** (icon `i-bell-off`).
    - Nếu đang Tắt → hiện mục **Bật thông báo** (icon `i-bell`, đưa về mức mặc định).
  - **Header trạng thái:** Sau tên nhóm có icon nhỏ 16px chỉ thị mức khác mặc định (chuông gạch khi Tắt, chuông reo khi Tất cả; mức mặc định không hiện icon để giảm tải thị giác).
- **Trên điện thoại (Phone):**
  - **Màn Thông tin nhóm (Group Info):** Truy cập bằng cách chạm vào tên nhóm ở thanh trên hoặc nút info. Hàng đầu tiên trong nhóm cài đặt là hàng **Thông báo** (chiều cao ≥ 56px, icon mức trong ô 32px nền sunk, tên mức hiện tại 13px mờ, chevron phải).
  - **Sheet chọn mức:** Chạm vào hàng Thông báo mở bottom sheet (`role="radiogroup"`), mỗi mức là một hàng `role="radio"` cao 64px với icon, nhãn, tag Mặc định, dòng giải thích và radio indicator 22px bên phải. Chạm chọn thì radio đổi ngay và sheet tự đóng mượt mà sau 160ms.
  - **Thao tác nhấn giữ ở danh sách (Long-press sheet):** Nhấn giữ một hàng cuộc trò chuyện ở tab Tin nhắn mở action sheet (hàng cao 52px), trong đó có mục chuyển đổi nhanh: **Tắt thông báo** (nếu đang bật) hoặc **Bật thông báo** (nếu đang tắt).

### 1.2. Hành vi với tin nhắn trực tiếp (DM behaviour)
- Tin nhắn trực tiếp (DM) chỉ có **hai mức**: **Tất cả** (mặc định) và **Tắt**.
- **Lý do:** Ở DM mọi tin nhắn đều gửi trực tiếp cho bạn, không có khái niệm "chỉ theo dõi một số thread", nên mức giữa không có ý nghĩa.
- Dùng cùng cơ chế menu và sheet với nhóm, nhưng giao diện chỉ hiển thị 2 lựa chọn Tất cả và Tắt. Thao tác lưu lạc quan và toast thông báo tương tự nhóm.

### 1.3. Dấu hiệu cuộc trò chuyện đã tắt thông báo (Muted indicator)
- **Hàng cuộc trò chuyện đã Tắt** (ở Navigator, Home feed và danh sách trên điện thoại):
  - Tên cuộc trò chuyện đổi sang `--color-ink-muted` font weight 400 (kể cả khi có tin nhắn mới).
  - Icon avatar/nhóm mờ còn 72% opacity.
  - Không hiển thị badge số tin chưa đọc thông thường.
  - Cuối hàng xuất hiện icon **chuông gạch 14px** (`mute-ic`, `--color-ink-subtle`), kèm chữ ẩn `sr-only` "đã tắt thông báo" cho screen reader.
  - Cuộc trò chuyện vẫn xếp theo thời gian hoạt động mới nhất nhưng **không mang lớp phủ màu người gửi realtime** và không trượt lên có hiệu ứng nảy: tôn trọng tuyệt đối trạng thái yên lặng.

### 1.4. Mục "Thread có tin mới" trên Trang chủ (Home 'Thread có tin mới')
- Đặt ngay trên danh sách cuộc trò chuyện ở Trang chủ (Desktop) và đầu tab Tin nhắn (Phone).
- **Điều kiện hiển thị:** Chỉ hiển thị các thread mà người dùng **đang theo dõi** và có câu trả lời chưa đọc, thuộc các nhóm **chưa bị Tắt thông báo**. Thread của các nhóm đã Tắt thông báo sẽ **không** xuất hiện tại đây.
- **Thứ tự & Giới hạn:** Xếp theo `last_reply_at` mới nhất ở trên. Hiển thị tối đa 3 mục trên Desktop (2 mục trên Mobile); phần dư nằm sau nút "Xem thêm N thread" / "Ẩn bớt".
- **Cấu trúc hàng thread:** Nút `role="listitem"` với avatar nhóm, tên nhóm nguồn (12px mờ), tác giả tin gốc + trích đoạn tin gốc 1 dòng (có ellipsis cắt ngắn), cụm avatar người trả lời + số câu trả lời mới accent + giờ trả lời cuối, cuối hàng là pill đếm số trả lời mới.
- **Tương tác:** Bấm vào thread sẽ mở nhóm tương ứng, cuộn tới tin gốc trong luồng chính và mở SidePanel thread. Nếu không có thread nào thoả mãn, toàn bộ khối tự ẩn hoàn toàn (không để lại empty state chiếm diện tích). Hỗ trợ nút Thu gọn/Mở rộng nhớ theo phiên.

---

## 2. Các điểm đã khắc phục trên Mockup (Remaining Fixes Applied)

1. **Cắt ngắn tên dài trên hàng thread (`.tn .tx`):**
   - Đã cập nhật `.tn .tx` từ inline sang `display: block; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; margin-top: 1px;`.
   - Kết quả: Khi tên tác giả hoặc nội dung thread dài, văn bản được thu gọn với dấu ba chấm (`...`) chính xác, không tràn hay phá vỡ layout grid.
2. **Khôi phục focus chính xác khi re-render (`renderAll`):**
   - Đã nâng cấp `focusKey(el)` quét toàn bộ `data-*` attributes (`data-act`, `data-k`, `data-frame`, `data-lv`, `data-press-k`, `data-nav-k`, `data-feed-k`, `data-thread-k`, ...) kết hợp cùng container ID (`host.id` hoặc class).
   - Trong `setLevel()`, biến `sel` được xác định thông qua `focusKey(trig)`, bảo đảm sau khi menu đóng và DOM re-render (cả lúc lạc quan lẫn sau khi timer lưu 220ms/1.5s hoàn tất), focus được khôi phục chính xác về control ban đầu thay vì nhảy lên nút demo ở đầu trang.
   - Bổ sung data attributes cho tất cả các hàng động trong `renderNav`, `renderFeed`, `renderThreads`, `renderP2`.
3. **Chiều cao nút Toast trên thiết bị di động:**
   - Đã áp dụng `min-height: 44px; height: 44px;` cho `.ptoasts .toast .btn` trong khung phone demo và media query `@media (max-width: 767px) { .toast .btn { min-height: 44px; height: 44px; } }`.
   - Đảm bảo đạt chuẩn WCAG touch target (≥ 44px) cho màn hình cảm ứng di động.
4. **Triệt tiêu hiện tượng tràn ngang ở viewport 390px (Horizontal overflow fix):**
   - Phát hiện nguyên nhân: Trong `.spec .rowdemo` ở bảng trạng thái (#states), phần tử ẩn hỗ trợ screen reader `<span class="sr">` có `position: absolute` nhưng không có ancestor nào được định vị tương đối (`position: relative`), khiến nó thoát ra initial containing block làm `document.documentElement.scrollWidth` phình lên 799px.
   - Khắc phục: Bổ sung `position: relative;` cho container `.scroll` và `.board`.
   - Kết quả: Tại viewport 390x844 (cả light và dark), `scrollWidth` hoàn toàn bằng `clientWidth = 390px`, không còn bất kỳ pixel tràn ngang nào.

---

## 3. Đề xuất bổ sung vào DESIGN.md (chưa áp)

Đề xuất thêm tiểu mục **Mức thông báo theo cuộc trò chuyện & Thread trên Trang chủ** vào §6 (sau mục Chat: tạo trong chat) của `DESIGN.md`:

> ### Mức thông báo theo cuộc trò chuyện & Thread trên Trang chủ (mockup `design/mockups/notification-levels.html`)
>
> Mỗi người dùng tự chọn mức thông báo riêng cho từng cuộc trò chuyện, độc lập với các thành viên khác.
>
> - **Ba mức cho nhóm:**
>   - **Tất cả:** Mọi tin mới trong luồng và mọi trả lời trong mọi thread của nhóm đều tính vào số chưa đọc và gửi thông báo in-app / đẩy. Sau tên nhóm ở header hiện icon chuông reo 16 (`i-bell-ring`).
>   - **Tin chính và thread đang theo dõi (Mặc định):** Chỉ tin mới trong luồng chính và trả lời trong các thread bạn chủ động nhấn "Theo dõi" mới tính vào số chưa đọc và gửi thông báo. Trả lời ở các thread khác chỉ cập nhật số đếm trên thẻ tóm tắt. Sau tên nhóm không hiện icon mức.
>   - **Tắt:** Không tính số chưa đọc, không gửi thông báo, không phủ màu realtime người gửi khi có tin mới. Hàng cuộc trò chuyện đổi tên sang `--color-ink-muted` 400, avatar mờ 72%, cuối hàng có icon chuông gạch 14 (`i-bell-off`). Sau tên nhóm ở header hiện icon chuông gạch 16.
> - **Tin nhắn trực tiếp (DM):** Chỉ có hai mức: **Tất cả** (mặc định) và **Tắt**.
> - **Vị trí điều khiển:**
>   - Máy tính: Tên nhóm/DM ở header là nút mở menu (`ttl`, min-height 36), chứa 3 `menuitemradio` (DM: 2 radio) có dòng mô tả. Menu ⋯ ở navigator là lối tắt một chạm: "Tắt thông báo" ↔ "Bật thông báo".
>   - Điện thoại: Màn Thông tin nhóm có hàng "Thông báo" (≥ 56px) mở bottom sheet chọn mức (hàng radio 64px). Thao tác nhấn giữ ở danh sách cuộc trò chuyện mở sheet có nút Tắt/Bật thông báo nhanh (hàng 52px).
> - **Lưu & phản hồi:** Lưu ngay (lạc quan) tại client. Quá 300ms chưa hoàn tất thì dòng phụ header hiện "· Đang lưu…" cùng spinner quay 12px. Thành công: Toast xác nhận kèm nút "Hoàn tác" (5s). Thất bại: rollback mức cũ kèm Toast cảnh báo có nút "Thử lại".
> - **Thread có tin mới (Trang chủ & đầu danh sách Mobile):** Khối nổi phía trên danh sách hội thoại, hiển thị các thread đang theo dõi có phản hồi chưa đọc (của các nhóm không bị Tắt). Xếp theo `last_reply_at` mới nhất. Desktop tối đa 3 mục, Mobile tối đa 2 mục, có nút xem thêm và thu gọn. Bấm vào mục mở nhóm và trượt panel thread.

---

## 4. Câu hỏi mở (Open Questions)

1. **Khi cuộc trò chuyện ở mức "Tắt", việc nhắc tên (@mention) tôi có thông báo không?**
   - **Hiện trạng trên mockup:**
     - Mockup hiện tại quy định và hiển thị rằng: **Vẫn thông báo khi bạn được nhắc tên!**
     - Cụ thể tại Quy tắc 3 và 6 trong mockup: Khi nhóm bị Tắt, nếu có người @mention bạn trong nhóm đó:
       - Hàng cuộc trò chuyện sẽ hiện thêm một **pill "@"** màu accent (`pill.at`) đứng ngay trước icon chuông gạch, kèm badge số lần nhắc tên và `aria-label="N lần nhắc tên bạn"`.
       - Số lần nhắc tên này vẫn được cộng dồn vào số tổng của tab Tin nhắn và badge thanh bên (chỉ số chưa đọc thông thường là bị triệt tiêu về 0).
       - Mô tả của mức Tắt trong menu nêu rõ: *"Không tính số chưa đọc, không thông báo. Vẫn báo khi có người nhắc tên bạn."*
     - **Vấn đề cần quyết định:** Đây là hành vi tương tự Google Chat / Slack (giúp người dùng không bỏ lỡ việc khẩn cấp được réo tên trực tiếp dù đã tắt ồn ào của nhóm). Tuy nhiên, có những trường hợp người dùng muốn "tắt hoàn toàn tuyệt đối" (Mute All). Cần làm rõ với Product/CEO: Giữ mặc định như mockup (Tắt nhưng vẫn báo @mention) hay bổ sung tùy chọn con "Bỏ qua cả nhắc tên"?
2. **DM ở mức "Tắt" có cho phép thông báo ngoại lệ nào không?**
   - Mockup hiện tại quy định DM khi "Tắt" là im lặng 100% (mô tả: *"Không tính số chưa đọc, không thông báo."*), không có pill hay badge nào hiện lên, vì mọi tin nhắn DM đều mặc định nhắm vào người nhận. Cần xác nhận hành vi này đã đạt kỳ vọng.
3. **Độ ưu tiên của "Thread có tin mới" khi nhóm bị Tắt:**
   - Mockup hiện loại bỏ hoàn toàn các thread thuộc nhóm đã Tắt khỏi danh sách "Thread có tin mới" trên Trang chủ để tránh làm phiền. Nếu trong thread đó bạn được @mention thì sao? Hiện mockup chỉ hiện pill "@" trên hàng nhóm chính, không đưa thread lên Trang chủ. Hành vi này có hợp lý không?

---

## 5. Kết quả kiểm chứng (Verification)

- **Headless Chromium Playwright:**
  - `390x844 Light`: 0 console errors, `clientWidth = 390`, `scrollWidth = 390` (PASS, không tràn ngang).
  - `390x844 Dark`: 0 console errors, `clientWidth = 390`, `scrollWidth = 390` (PASS, không tràn ngang).
  - `1440x900 Light`: 0 console errors (PASS).
  - `1440x900 Dark`: 0 console errors (PASS).
- **Kiểm tra tương tác:**
  - `tnTxStyle`: `display: block; min-width: 0; text-overflow: ellipsis; white-space: nowrap;` (PASS).
  - `toastBtnHeight`: Chiều cao nút toast trên phone viewport = 44px (PASS).
  - `focusKey`: Khôi phục focus chính xác về control đang thao tác sau khi `renderAll()` chạy (PASS).
- **Bộ ảnh chụp:** Đã ghi đè toàn bộ 36 ảnh chụp vào thư mục `plans/reports/design-261011-0935-notification-levels/`.
- **Git status:** Không commit mã theo quy định.
