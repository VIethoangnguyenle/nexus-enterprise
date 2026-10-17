---
type: brainstorm
date: 2026-10-17
topic: Gợi ý khi chưa gõ cho ô tìm của mọi module
status: chờ chốt hướng
---

# Brainstorm: gợi ý cho Search (zero-query suggestions)

## Đã chốt với Zane
- Loại: **gợi ý khi chưa gõ**: mở ô tìm là có sẵn danh sách. Không làm typeahead, ⌘K hay "ý bạn là".
- Module: Chat, Tài liệu, Danh bạ, Tài sản + Phê duyệt.

## Hiện trạng
- Ô tìm theo từng màn (`SearchField`, `[data-module-search]`; trên điện thoại `PhoneSearchBar` dẫn tới ô đó).
- Chỉ Chat có tìm trên máy chủ (`GET /api/messages/search`) và "Tìm gần đây" (`lib/recent-searches.ts`: localStorage
  `ngac-chat-recent-searches:<user>`, tối đa 8, xoá sạch khi đăng xuất). Spec: `docs/specs/chat-search/spec.md`.
- Tài liệu chỉ lọc trong thư mục đang mở. Danh bạ và Quản trị lọc phía client. Tài sản có `?q=`. Phê duyệt chưa có
  tìm theo chữ.
- DESIGN.md mới mô tả ô tìm của Chat (phần "Tìm trong chat") và thanh tìm điện thoại; chưa có thiết kế gợi ý cho các
  module khác.

## Contract
- **Outcome:** chạm hay focus vào ô tìm của 5 module, khi chưa gõ thì hiện ngay một panel gợi ý: "Tìm gần đây" của
  module đó và 1–2 nhóm gợi ý theo ngữ cảnh. Chạm một gợi ý là mở đúng thứ đó hoặc chạy lại lần tìm đó. Gõ chữ thì panel
  nhường chỗ cho kết quả như hiện nay.
- **Constraints:**
  - Thiết kế trước: thêm mục vào DESIGN.md và mockup trong `design/mockups/`, Zane duyệt rồi mới code.
  - Gợi ý chỉ hiện thứ người dùng đang được đọc. Dựa trên query đã qua PEP; không lưu tên hay tiêu đề vào máy.
  - Không hiện ID trên màn hình. Thực thể hiện bằng tên, người hiện bằng tên + avatar.
  - Lịch sử tìm tính theo người, theo module, trên từng máy. `clearAllRecent` phải xoá được mọi key mới khi đăng xuất.
  - Không thêm node hay operation NGAC.
- **Non-goals:** typeahead, ⌘K / tìm toàn cục, sửa chính tả (pg_trgm/unaccent), đồng bộ lịch sử giữa các máy,
  tìm toàn workspace cho Drive.
- **Acceptance:**
  - Mỗi module: focus ô trống thì thấy panel; có "Tìm gần đây" (xoá từng mục được); chạm mục thì chạy lại lần tìm.
  - Có test chứng minh người đã mất quyền đọc thì không còn thấy gợi ý về thứ đó (deny case).
  - Đăng xuất xong thì không còn key lịch sử tìm nào.
  - Lịch sử tìm của Chat hiện có vẫn còn sau khi đổi sang key mới (migrate hoặc đọc key cũ).
  - Vitest có cho từng nguồn gợi ý. `npm run lint` / `build` xanh. Spec cập nhật trong `docs/specs/`.

## Phương án

| | A. Chỉ phía client từ dữ liệu sẵn có (**đề xuất**) | B. Máy chủ ghi "mở gần đây" | C. A + "đã mở gần đây" lưu ID trên máy |
|---|---|---|---|
| Làm gì | Tổng quát hoá `recent-searches` theo module. Gợi ý lấy từ các query đã có: Chat là cuộc trò chuyện hoạt động gần nhất; Danh bạ là người vừa nhắn trực tiếp; Tài sản là tài sản đang giữ + hoạt động gần đây (`/assets/activity`); Phê duyệt là đề nghị chờ mình duyệt; Tài liệu là tệp sửa gần nhất trong thư mục đang mở | Bảng `recent_items(user, kind, ref, opened_at)`, endpoint, mỗi lần trả về đều batch check `read` | Như A, thêm danh sách ID đã mở trên máy. Lúc hiện thì lấy tên qua fetch đã có PEP |
| Backend | Không | Migration, endpoint, PEP, spec | Không |
| Dựa vào | Dữ liệu sẵn có là đủ để gợi ý có ích | Người dùng cần lịch sử chung giữa các máy | Fetch theo ID rẻ, và trả 403/404 sạch khi mất quyền |
| Hỏng trước ở đâu | Drive: gợi ý chỉ trong thư mục đang mở, khá nghèo | Thêm một đường quyết định mới; ghi mỗi lần mở thì tốn | Thêm N request lúc mở panel; dễ làm lộ tên nếu ai đó cache tên |

**Đề xuất A.** Lý do: không thêm đường quyết định quyền, không cần migration, bỏ đi cũng rẻ nhất. Nếu sau khi chạy thật
thấy gợi ý của Drive quá nghèo, chuyển sang C riêng cho Drive, không cần sửa phần còn lại.

## Hướng triển khai A
1. Thiết kế: một pattern `SearchSuggestions` dùng chung (nhóm "Tìm gần đây" + nhóm theo ngữ cảnh, hàng 48 trên điện
   thoại), ghi vào DESIGN.md và mockup cho 5 module; Zane duyệt.
2. `lib/recent-searches.ts` nhận thêm tham số module (key `ngac-recent-searches:<module>:<user>`), đọc key cũ của Chat,
   và `clearAllRecent` xoá mọi tiền tố.
3. Mỗi module viết một hook trả về các nhóm gợi ý, chỉ đọc query đã có.
4. `SearchField` thêm chế độ panel khi focus mà ô còn trống. `ChatSearchScreen` chuyển sang dùng pattern chung.
5. Viết test trước cho từng bước: deny case, xoá khi đăng xuất, migrate key cũ.
6. Cập nhật spec: `chat-search`, `contacts-documents-settings-screens`, `assets-screens`, `approval-screens`.

## Câu hỏi còn mở
- Drive có endpoint liệt kê tệp sửa gần đây trên toàn workspace mà lọc theo quyền không? Chưa kiểm tra. Nếu có, gợi ý
  của Drive tốt lên nhiều mà vẫn giữ được A.
- Phê duyệt chưa có tìm theo chữ. Gợi ý ở đó chạy lại lần tìm gì: lọc phía client theo tiêu đề, hay cần thêm `?q=`?
- Trên máy tính, panel gợi ý là dropdown dưới ô tìm hay chiếm cả cột danh sách?
