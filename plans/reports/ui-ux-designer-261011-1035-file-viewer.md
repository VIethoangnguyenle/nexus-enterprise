# Trình xem tệp chung (FileViewer), thiết kế

Mockup: `design/mockups/file-viewer.html` (dùng tokens.css; chrome trang, primitive, thẻ đính kèm, `.vbar`/`.vstage`/`.vnav`/`.vcount` và keyframe chép từ spaces.html; sheet chép từ chat-mobile.html).
Ảnh chụp: `plans/reports/design-261011-1035-file-viewer/`: `page-*`, `kinds-*`, `states-*`, `phone-frames-*`, `live-*` (pdf, excel, word-preparing, word-ready, image, csv, 403, sheet), ở cỡ 1440/390 và giao diện light/dark.

## Kiểm chứng (Playwright chromium, frontend/)
- 1440x900 và 390x844, light và dark: 0 lỗi console, không tràn ngang (scrollWidth = clientWidth ở trang, hộp thoại và thanh trên).
- Mở từ thẻ PDF thì focus vào Đóng. ← chuyển 5/5 → 4/5. Word chưa có bản PDF hiện "Đang chuẩn bị…" rồi tự chuyển sang trang (khoảng 4 giây). Bấm Tab 30 lần thì focus vẫn ở trong trình xem. Esc đóng và trả focus về đúng thẻ đã bấm. Ở 403 thanh trên chỉ có "Trình xem tệp" (không tên tệp).
- Lỗi đã sửa trong lúc kiểm: bộ chọn focus trap `[href]` bắt cả `<use href>` của SVG nên trap bị thủng. Phase 03 cần nhớ điều này: chỉ dùng `a[href]`.

## Quyết định
1. **Một component, mọi nơi mở:** gồm thẻ tệp và ảnh trong tin, thread, tab Tệp của nhóm, Tài liệu. Hành vi thẻ tệp đổi: bấm vào thẻ thì **mở trình xem** (code hiện tải xuống; DESIGN.md hiện ghi "mở xem trước trong Tài liệu"). Nút Tải xuống khi trỏ chuột vẫn là nút riêng. Accessible name: "Xem trước <tên>".
2. **Desktop là hộp thoại phủ, không toàn màn** (khác ImageViewer cũ, theo yêu cầu): cách mép 32, radius 12, `--shadow-overlay`, nền `--color-scrim`. Bên trong giữ đúng dáng ImageViewer: thanh trên `--color-base` cao 56, sân khấu `--color-sunk`, nút trước/sau tròn 44 `--color-overlay` ở mép, "n / N" dưới. Chuyển động theo Modal.
3. **Thanh trên:** ô loại 36 (tông theo `file-kind.ts`: bảng tính success, PDF danger, văn bản info, trình chiếu warning, ảnh/video/âm thanh accent-wash, khác sunk). Tên cắt giữa: giữ 6 ký tự cuối trước đuôi và phần đuôi, tên đầy đủ trong `title`. Dòng 2: "Loại · dung lượng · [Avatar 20] Tên **gửi lúc** 09:36" (tin nhắn) hoặc "**tải lên** 02/10" (Tài liệu), có tooltip ngày giờ đủ. Dưới 1024 bỏ động từ, nút chỉ còn icon (vẫn có aria-label). Thao tác: Mở trong Tài liệu (ghost), Tải xuống (secondary), vạch, Đóng. Không có primary trên thanh, vì primary để dành cho việc chính trong thân (Tải xuống ở trạng thái lỗi hoặc tệp không hỗ trợ).
4. **Mở trong Tài liệu** chỉ hiện khi tệp nằm trong một thư mục mà người xem mở được, và không hiện khi đang mở từ Tài liệu.
5. **Phạm vi ←/→:** chỉ trong một tin (ảnh trước rồi tới tệp, gộp một dãy) hoặc một danh sách (thứ tự và bộ lọc đang hiển thị). Không quay vòng. Đổi tệp thì đặt lại mức phóng.
6. **Viên công cụ** (một overlay ở giữa đáy sân khấu, cùng ngữ pháp với thanh hover actions): ảnh có Vừa khung / 100% (`aria-pressed`, bấm vào ảnh cũng đổi; ở 100% kéo để di chuyển). PDF có "Trang [n] / N" (gõ số rồi Enter để nhảy) · − · mức % (bấm để vừa chiều ngang) · +, bước 25%, từ 50% đến 200%.
7. **PDF/Office:** trang xếp dọc, rộng tối đa 816 khi vừa chiều ngang, cách nhau 16. Trang ở xa chưa vẽ thì hiện khung giấy shimmer. Giấy giữ màu trắng ở dark mode (đó là nội dung tài liệu). Office thêm một dòng mờ 12px: "Bản xem trước từ tệp Word/Excel/PowerPoint/OpenDocument, có thể khác bản gốc đôi chút." Bảng tính và trình chiếu là trang ngang.
8. **Văn bản / mã / CSV:** nằm trên tấm `--color-raised` radius 10, rộng tối đa 1040, mono 13/1.6. Văn bản xuống dòng, tối đa 88ch. Mã có số dòng (không chọn được), cuộn ngang, comment mờ, không tô màu cú pháp (chưa có token). CSV là bảng kẻ mảnh, hàng **36** (dày hơn bảng 44 của app vì đây là dữ liệu), tiêu đề dính trên, cột số thứ tự dính trái, cột số căn phải. Giới hạn **500 dòng hoặc 1 MB**. Dải đáy luôn hiện: "Đang xem 500 dòng đầu trên 12.480 dòng." + Tải xuống để xem đủ.
9. **Media:** dùng `controls` gốc của trình duyệt, `preload="metadata"`, không tự phát. Âm thanh là thẻ 520 ở giữa gồm ô icon 56, tên, "Âm thanh · 4,2 MB · 12:40". ←/→ không hoạt động khi focus đang ở video/âm thanh (phím mũi tên dùng để tua) hoặc ở ô số trang.
10. **Trạng thái:** thanh trên vẽ ngay từ dữ liệu nơi mở, chỉ thân chờ.
    - Đang tải: khung xương đúng hình, trễ 300ms mới hiện.
    - Đang chuẩn bị: ô loại 64 + "Đang chuẩn bị bản xem trước…" + "Lần đầu mở tệp Word cần vài giây. Những lần sau sẽ mở ngay." + thanh 240×4 tạo cảm giác tiến độ (`0.92·(1−e^(−t/1.6s))`, scaleX, 160ms mỗi bước; `progressbar` không có giá trị vì không có tiến độ thật) + "Tải xuống thay vì chờ" (ghost). Sau 15 giây đổi câu phụ. Khi xong thì mờ dần sang trang và aria-live báo "Bản xem trước đã sẵn sàng". Chuyển sang tệp khác không huỷ việc chuyển đổi.
    - Lỗi chuyển đổi: nói lý do (mật khẩu / hỏng / quá thời gian, chỉ ca quá thời gian có Thử lại) + Tải xuống primary.
    - Quá lớn: "Tệp này 186 MB, vượt giới hạn xem trước 100 MB." + Tải xuống.
    - 403: không có tên, loại, dung lượng, người gửi; không Tải xuống, không sheet; tên dialog là "Trình xem tệp"; ←/→ vẫn dùng được.
    - 404: giữ tên mà nơi mở đã biết, chữ mờ, thêm "· Đã xoá", không có thao tác.
    - Lỗi mạng: thêm (ngoài yêu cầu), "Không tải được tệp" + Thử lại.
11. **Điện thoại (bố cục theo container query `vw` < 768, cùng code cho overlay thật và khung trong trang):**
    - Toàn màn, không radius.
    - Thanh trên: Đóng 44 bên trái, tên + "Loại · dung lượng", ⋯ 44 mở sheet.
    - Sheet: tên đầy đủ, loại, người gửi + lúc gửi, Tải xuống, Mở trong Tài liệu.
    - Thanh đáy `--color-base`: ‹ 44 · "n / N" · › 44. Ở tệp đầu/cuối nút bị tắt, không ẩn. Không cần vuốt.
    - PDF trên điện thoại chỉ còn [n] / N, phóng bằng chụm hai ngón.
12. **A11y:**
    - Dialog: `role=dialog` + `aria-modal`, tên lấy từ `aria-labelledby` (tên tệp).
    - Trang phía sau đặt `inert`. Focus trap và Tab vòng cũng áp cho sheet. Mở thì focus vào Đóng.
    - Esc đóng sheet trước, rồi mới đóng trình xem. Đóng thì trả focus về chỗ đã bấm.
    - Phím: `+ − 0` để phóng. Vùng cuộn có `role=region tabindex=0`. aria-live đọc "Tệp 2 / 5, <tên>" khi đổi tệp.
    - PDF thật cần lớp chữ (chọn chữ, trình đọc màn hình).
13. **Chuyển động:**
    - Desktop: nền mờ dần 220ms + hộp `translateY(8px) scale(.98)` 280ms expo, đóng 210ms ease-in.
    - Điện thoại: `scale(.98)`.
    - Đổi tệp: `translateX(±16px)` + mờ 160ms. Sheet trượt lên 280ms expo.
    - Giảm chuyển động: chỉ mờ 120ms.

## Đề xuất đoạn DESIGN.md (chưa áp dụng; thay mục "Trình xem ảnh (ImageViewer)" ở §6 Chat và sửa câu cuối mục "Thẻ tệp")
- Thẻ tệp, câu cuối: "…hover hiện IconButton Tải xuống; bấm thẻ mở **Trình xem tệp**."
- **Trình xem tệp (FileViewer)** (mockup `design/mockups/file-viewer.html`), một trình xem cho mọi tệp mở từ tin, thread, tab Tệp và Tài liệu.
  - **Hộp:** ≥ 768 là hộp thoại phủ (`--z-modal`, cách mép 32, radius 12, `--shadow-overlay`, nền `--color-scrim`). Dưới 768 toàn màn.
  - **Thanh trên** (`--color-base`, 56): ô loại 36 theo `file-kind`, tên cắt giữa (giữ đuôi), "Loại · dung lượng · PersonChip 20 + gửi lúc/tải lên + giờ". Phải: Mở trong Tài liệu (ghost; chỉ khi tệp nằm trong thư mục người xem mở được, và không hiện khi mở từ Tài liệu), Tải xuống, Đóng. Dưới 1024 các nút chỉ còn icon.
  - **Sân khấu** (`--color-sunk`): nút trước/sau tròn 44 ở mép, "n / N" dưới, chỉ trong cùng tin hoặc cùng danh sách, không quay vòng.
  - **Thân theo loại:**
    - Ảnh: vừa khung / 100% + kéo.
    - PDF và Office (đổi trên máy chủ): trang dọc rộng tối đa 816 + viên "Trang [n] / N · − % +". Office thêm dòng mờ "Bản xem trước từ tệp Word, có thể khác bản gốc đôi chút."
    - Video, âm thanh: `controls` gốc.
    - Văn bản, mã, CSV: mono 13; mã có số dòng; CSV là bảng tiêu đề dính, hàng 36. Tối đa 500 dòng / 1 MB, dải đáy "Đang xem N dòng đầu trên M dòng." + Tải xuống để xem đủ.
    - Loại khác: icon 64 + "Không xem trước được loại tệp này" + Tải xuống (primary).
  - **Trạng thái:**
    - Skeleton (sau 300ms).
    - "Đang chuẩn bị bản xem trước…" (Office lần đầu, thanh tiến độ không giá trị, Tải xuống thay vì chờ, aria-live khi xong).
    - Lỗi chuyển đổi và quá lớn (nói lý do + Tải xuống).
    - 403 "Bạn không có quyền xem tệp này": không tên, loại, dung lượng, người gửi; không có thao tác.
    - 404 "Tệp này đã bị xoá": tên mờ, không có thao tác.
  - **Điện thoại:** Đóng trái, ⋯ phải (sheet: tên đầy đủ, người gửi, Tải xuống, Mở trong Tài liệu). Thanh đáy ‹ n / N ›, nút tắt ở hai đầu. Phóng bằng chụm hai ngón.
  - **Bàn phím:** Esc (sheet trước), ←/→ (trừ khi đang ở media/ô số trang), + − 0. Focus trap, trang sau `inert`, mở thì focus vào Đóng, đóng thì trả focus về chỗ đã bấm.
- §7: sửa dòng "Trình xem ảnh" thành "Trình xem tệp | desktop: như Modal; điện thoại: scrim fade 220ms + `scale(.98)` → 1, 280ms expo | 210ms ease-in"; "Đổi ảnh trong trình xem" thành "Đổi tệp…". Thêm dòng "Bản xem trước sẵn sàng | fade 160ms | không".

## Ghi chú cho phase 02/03
- Viewer cần từ API, kèm mỗi tệp: tên, mime, dung lượng, người tải lên/gửi (tên hiển thị) và thời điểm, thư mục chứa nó + người xem có mở được thư mục đó không, số trang (tuỳ chọn), số dòng CSV (tuỳ chọn).
- Endpoint preview cần trả **trạng thái phân biệt được**: sẵn sàng / đang chuyển đổi (202 + thử lại sau) / lỗi kèm lý do (mật khẩu, hỏng, quá thời gian) / quá lớn (kèm giới hạn) / 403 / 404.
- `ImagePreviewCard` và `ImageViewer` của chat gộp vào FileViewer. `DriveDetailPanel` nên có nút "Xem trước" mở cùng viewer (hiện chỉ có ảnh nhỏ).

## Câu hỏi mở
1. Giới hạn thật: xem trước tối đa bao nhiêu MB theo từng loại (mockup dùng 100 MB)? Có giới hạn số trang hay thời gian cho Gotenberg (mockup ghi ~10 giây)?
2. Excel đổi qua PDF thường xấu (sheet rộng bị chia nhiều trang). Có nên xem .xlsx bằng bảng như CSV (sheet đầu, 500 dòng) thay vì PDF?
3. Tệp trong tin nhắn có luôn nằm trong một thư mục Drive không? Nếu có, thư mục nào, và người trong nhóm có quyền đọc thư mục đó không (quyết định khi nào hiện "Mở trong Tài liệu")?
4. 403: thẻ trong tin đã cho người xem thấy tên tệp, nhưng viewer vẫn giấu theo yêu cầu. Xác nhận giữ cách này. Có cần "Xin quyền xem" không? (Mockup Tài liệu có nút này, nhưng sẽ lộ tên chủ sở hữu và hiện chưa có API.)
5. Sheet điện thoại dùng radius 22 của chat-mobile.html, còn §5 DESIGN.md ghi sheet radius 12. Cần chốt một giá trị cho mọi sheet.
6. Có tô màu cú pháp cho mã không? Nếu có cần token màu cú pháp mới; mockup chỉ làm mờ comment.
