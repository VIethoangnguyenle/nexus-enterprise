---
type: brainstorm
date: 2026-10-16
slug: drive-gallery-refactor
status: accepted
---

# Brainstorm: UI tinh gọn — Drive kiểu Google Drive, tab Tệp, ảnh đại diện & ảnh nhóm

Ba yêu cầu của người dùng trong cùng phiên (2026-10-16):
1. Refactor UI Drive giống Google Drive, có preview; refactor tab Tệp trong chat; cho web, máy tính bảng, điện thoại.
2. UI tinh gọn: thao tác nằm trên chính đối tượng (vd chạm avatar → icon đổi ảnh, không có nút riêng bên ngoài).
3. Hỗ trợ đổi ảnh nhóm.

## Hiện trạng (đã kiểm)
- Drive (`components/drive/`): cây thư mục + bảng mảnh 44px (`DriveTable`) + `DriveDetailPanel`. Chỉ danh sách, không lưới, không thumbnail.
- `components/files/FileViewer` đã có (ảnh, PDF, Office qua Gotenberg, video, bảng tính, text), dùng chung chat / tab Tệp / Drive.
- Backend drive: chỉ `GET /api/drive/files/:fileId/preview` (`rest/handler.go:97`); không có thumbnail.
- Tab Tệp chat (`spaces/SpaceFiles.tsx`): bảng 4 cột thô, không lọc loại.
- Ảnh cá nhân (`settings/ProfileTab.tsx` `AvatarEdit`): avatar 64 + hai nút "Đổi ảnh" / "Bỏ ảnh" bên cạnh. Backend: auth `UploadAvatar/RemoveAvatar/ServeAvatar`, bảng `user_avatars` (migration 055), token mờ `/api/auth/avatars/<token>`.
- Ảnh nhóm: **không tồn tại** — `channels` không có cột ảnh, messaging không có API, frontend vẽ ô chữ cái. Đổi tên nhóm cần `manage` trên OA nội dung kênh (`messaging/internal/domain/channels.go:248`).
- DESIGN.md không có mục riêng màn Tài liệu (mockup `core-screens.html` §2); FileViewer có spec + `file-viewer.html`.

## Quyết định của người dùng (2026-10-16)
- Thumbnail do **server sinh**; **đồng ý thêm `poppler-utils` + `ffmpeg`** vào image drive service.
- Preview Drive = **lưới thẻ có thumbnail** + **panel chi tiết preview lớn** + **Space/Enter mở FileViewer**, ←/→ trong thư mục.
- Tab Tệp chat **dùng chung component với Drive**: chip lọc Tất cả · Ảnh & video · Tài liệu · Khác, nhóm theo thời gian.
- Phủ **web, máy tính bảng, điện thoại**.
- "Tinh gọn" = **nguyên tắc chung ghi vào DESIGN.md + áp cho các màn đang đụng** (ảnh cá nhân, ảnh nhóm, Drive, tab Tệp). Rà soát toàn app là plan riêng, không nằm ở đây.

## Hợp đồng

**Outcome:**
- Màn Tài liệu đọc như Google Drive: thanh công cụ gọn (tìm, lọc loại, Lưới/Danh sách, sắp xếp), thư mục dạng ô ở trên, tệp là thẻ có ảnh xem trước thật, panel phải có preview lớn, Space mở FileViewer.
- Tab Tệp của nhóm chat dùng cùng bộ hiển thị, có lọc loại và nhóm thời gian.
- Ảnh cá nhân và ảnh nhóm đổi ngay trên ảnh: chạm/hover hiện lớp phủ icon máy ảnh, mở menu **Đổi ảnh · Bỏ ảnh**; không còn hàng nút riêng.
- Nhóm có ảnh riêng, hiện ở mọi chỗ đang vẽ icon nhóm (cột điều hướng, Trang chủ di động, header, panel thông tin, tìm kiếm, thông báo).

**Constraints:**
- Design-first: DESIGN.md thêm (a) nguyên tắc "thao tác trên chính đối tượng", (b) mục "Tài liệu: lưới, danh sách, panel", (c) mẫu **AvatarEdit** (lớp phủ, menu, trạng thái đang tải/lỗi, bàn phím: Enter/Space mở menu, nhãn "Đổi ảnh đại diện"/"Đổi ảnh nhóm"), (d) icon nhóm có ảnh. Mockup mới `design/mockups/drive-gallery.html` + `design/mockups/avatar-edit.html` cho web + máy tính bảng + điện thoại; người dùng duyệt **trước** khi code.
- Token/hướng "Tín hiệu"; không UUID/ID ra màn hình; người = tên + avatar.
- **Thumbnail:** `GET /api/drive/files/:fileId/thumbnail` (2 cỡ, vd 320/640). Quyền như `preview` (read trên OA cha qua `policyclient`, fail-closed). Sinh lười, cache MinIO theo `fileId + version`; giới hạn số lượt sinh đồng thời. Ảnh: resize Go; PDF: `pdftoppm` trang 1; Office: Gotenberg → PDF → `pdftoppm`; video: `ffmpeg` khung ~1s. Không sinh được → frontend rơi về ô icon `file-kind`. Dockerfile drive + hướng dẫn dev native (`docs/`) có hai binary.
- **Ảnh nhóm:** migration mới `channel_avatars` theo đúng mẫu `user_avatars` (token mờ, ảnh lưu đã chuẩn hoá); cùng giới hạn JPG/PNG ≤ 5 MB, rate limit, cổng xử lý như ảnh cá nhân — dùng lại code chuẩn hoá ảnh của auth qua `backend/pkg` nếu tách được, không chép. Đổi/bỏ ảnh cần **`manage` trên OA nội dung kênh** (như đổi tên); DM không có ảnh nhóm. Phát sự kiện realtime cho thành viên. Không đổi mô hình NGAC, không node mới, không thao tác mới.
- Route mới vào proxy `frontend/vite.config.js` (route dưới `/api/workspaces/:id/` thì vào khối regex). Layering transport → domain → store; `make check-layering`, `make check-ngac`.
- Schema đổi → `make db-migrate` và kiểm trên DB đang chạy.
- Spec: cập nhật capability preview của drive (thumbnail) và capability kênh (ảnh nhóm) trong `docs/specs/`.
- Giữ `FileViewer` nguyên vẹn; chỉ thêm điểm vào.
- TDD (`ak:cook --tdd`); test deny cho cả thumbnail và đổi ảnh nhóm (không quyền → 403, không lộ thông tin).

**Non-goals:**
- Rà soát tinh gọn toàn app (Chat, Phê duyệt, Tài sản, Danh bạ, Quản trị, Cài đặt khác) — plan riêng.
- Không đổi FileViewer, không tô màu cú pháp, không sửa luồng upload/chia sẻ/di chuyển (chỉ đổi chỗ đặt nút).
- Không "Gần đây / Gắn sao / Thùng rác", không chọn nhiều/kéo thả hàng loạt, không trang "Bộ nhớ".
- Không đổi `AttachmentGrid` trong tin nhắn.
- Không cắt/xoay ảnh trước khi tải lên; không ảnh cho DM; không thumbnail cho thư mục; HEIC/RAW rơi về icon.

**Acceptance criteria:**
1. Mockup + DESIGN.md được người dùng duyệt.
2. **Drive:** Lưới/Danh sách đổi được và nhớ theo người dùng; lưới: thư mục trước, thẻ tệp có thumbnail thật cho JPG/PNG/WebP, PDF, DOCX/XLSX/PPTX, MP4; loại khác ô icon.
3. Chọn tệp → panel preview lớn + thông tin + Mở/Tải xuống/Chia sẻ; Space/Enter mở FileViewer, ←/→ trong thư mục, Esc trả focus về thẻ.
4. **Thumbnail:** không quyền read → 403 (test deny); có quyền → ảnh; lần hai lấy từ cache (test); tệp bị thay thế → thumbnail mới.
5. **Tab Tệp:** cùng component; chip lọc 4 loại; nhóm "Hôm nay / Tuần này / Tháng này / Cũ hơn"; ảnh lưới vuông; mở FileViewer.
6. **Ảnh cá nhân:** Cài đặt → Hồ sơ không còn nút "Đổi ảnh"/"Bỏ ảnh" bên ngoài; chạm/hover avatar hiện icon máy ảnh, menu Đổi ảnh · Bỏ ảnh (Bỏ ảnh chỉ khi đã có ảnh); đang tải hiện tiến độ trên avatar; lỗi hiện dưới avatar; dùng được bằng bàn phím + trình đọc màn hình.
7. **Ảnh nhóm:** người có `manage` đổi/bỏ ảnh từ panel thông tin nhóm bằng cùng mẫu; thành viên không có `manage` thấy ảnh nhưng không có lớp phủ, API trả 403 (test deny); ảnh mới hiện ngay ở mọi máy thành viên (realtime) và mọi chỗ vẽ icon nhóm; nhóm chưa có ảnh vẫn hiện ô chữ cái như cũ.
8. **Ba khổ màn** (mốc 768/1024 của DESIGN.md), áp cho Drive, tab Tệp và avatar:
   - **Web ≥ 1024:** cây thư mục trái + lưới tự giãn cột + panel phải; hover hiện ⋯ và lớp phủ avatar; chuột phải mở menu.
   - **Máy tính bảng 768–1023:** cây thư mục thành ngăn kéo (☰ cạnh breadcrumb); lưới 3–4 cột; panel thành sheet phải phủ lên; vùng chạm ≥ 44; ⋯ và icon máy ảnh trên avatar luôn hiện (không cần hover).
   - **Điện thoại < 768 (390px):** lưới 2 cột hoặc danh sách hàng 56; thanh công cụ gọn, nút + nổi để tải lên/tạo thư mục; chi tiết là bottom sheet; nhấn giữ thẻ mở sheet thao tác; chạm avatar mở sheet Đổi ảnh · Bỏ ảnh (có "Chụp ảnh"); không cuộn ngang.
   - Test bố cục theo từng khổ; xem tận mắt ở 390 / 820 / 1440.
9. `make build-check`, `make check-layering`, `make check-ngac`, `make test s=drive`, `make test s=messaging`, `make test s=auth`, `npm test`, `npm run lint`, `npm run build` xanh; `make db-migrate` đã chạy; xem trên app thật (`make dev`).

## Hướng đã chọn & phân pha gợi ý
1. **Thiết kế:** nguyên tắc tinh gọn + mục Tài liệu + mẫu AvatarEdit + icon nhóm có ảnh trong DESIGN.md; mockup `drive-gallery.html`, `avatar-edit.html` (3 khổ) → duyệt.
2. **Mẫu AvatarEdit dùng chung (frontend):** primitive lớp phủ + menu/sheet; thay `ProfileTab` sang mẫu mới. Không đụng backend.
3. **Ảnh nhóm (backend):** tách chuẩn hoá ảnh của auth ra `backend/pkg` (nếu được) → migration `channel_avatars` → domain/store/REST messaging (upload, remove, serve), quyền `manage`, sự kiện realtime → proxy + spec + test allow/deny.
4. **Ảnh nhóm (frontend):** AvatarEdit trong panel thông tin nhóm; mọi chỗ vẽ icon nhóm nhận `avatar_url`; cập nhật cache theo sự kiện realtime.
5. **Thumbnail (backend):** endpoint + sinh lười + cache MinIO + giới hạn đồng thời; Dockerfile thêm poppler-utils + ffmpeg; docs dev native; test allow/deny/cache/fallback.
6. **Frontend dùng chung tệp:** `components/files/` thêm `FileGrid`, `FileCard`, `FolderTile`, `FileThumb` (skeleton, fallback icon), toggle view (Zustand, persist).
7. **Drive:** thân màn lưới/danh sách, panel preview, phím tắt, 3 khổ.
8. **Tab Tệp chat:** `SpaceFiles` dùng `FileGrid` + chip lọc + nhóm thời gian.
9. **Kiểm thử + review:** `ak:test`, `ak:code-review`, xem trên app ở 3 khổ.

Pha 2–4 (avatar) và 5–8 (tệp) độc lập, có thể ship thành 2 PR: avatar trước (nhỏ hơn), tệp sau.

## Lỗi kèm theo: lề phải luồng chat trên điện thoại quá rộng (người dùng báo 2026-10-16)
- **Nguyên nhân (từ source):** `spaces/MessageBlock.tsx:133-134`: tin của mình luôn dùng lưới `grid-cols-[minmax(0,1fr)_2rem]` + `gap-x-2` + `px-1`, cột 32 bên phải chỉ để chứa giờ của tin gom cụm (`GutterTime`). Cộng với `px-2.5` của `ConversationStream.tsx:322`, bong bóng của mình cách mép phải **54px** (10 + 4 + 8 + 32), trong khi tin người khác cách mép trái 14px rồi tới avatar. Cột trống có mặt cả ở tin không gom cụm.
- Spec gây ra điều này: DESIGN.md "Gom tin" đặt giờ vào "cột 32 bên phải với bạn" mà không phân biệt khổ; phần chat điện thoại chỉ nói "chạm tin hiện giờ trong cụm".
- **Hướng sửa:** dưới 768 bỏ cột phải của tin mình (bong bóng sát lề 14px, đối xứng với bên trái); chạm tin gom cụm thì giờ hiện ngay trên bong bóng, căn phải. Desktop giữ nguyên (giờ hiện khi hover ở lề). Sửa DESIGN.md mục "Gom tin" + "Chat trên điện thoại" trong cùng PR. Áp cho cả panel thread (cùng `MessageBlock`).
- Kiểm: test theo khổ (không có cột phải dưới md, chạm hiện giờ), xem tận mắt ở 390px trên `make dev`.
- Lỗi nhỏ và độc lập: sửa trước qua `ak:fix`, không đợi plan lớn.

## Rủi ro chưa giải
- Thư viện chuẩn hoá ảnh của auth có thể dính chặt vào auth domain; nếu tách ra `backend/pkg` tốn nhiều công thì plan phải chọn giữa tách hay gọi auth qua gRPC — kiểm ở đầu pha 3.
- Chi phí CPU khi mở thư mục nhiều tệp lần đầu: cần giới hạn đồng thời + kiểm thời gian sinh Office (Gotenberg đã có giới hạn 45s).
- Image drive service nặng thêm ~80–120 MB (người dùng đã chấp nhận).

## Câu hỏi còn mở
- Không còn.
