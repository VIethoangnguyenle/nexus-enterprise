---
type: brainstorm
date: 2026-10-14
topic: Đổi ảnh đại diện (avatar upload)
status: accepted
decision: A — Postgres + endpoint auth
---

# Brainstorm: đổi ảnh đại diện

## Hiện trạng (đã kiểm)
- `users.avatar_url` có sẵn; auth (`store/store.go`, `directory.go`, `identity.go`), workspace (`directory_store.go`), messaging (`store/calls.go`) đã đọc và trả về client.
- `Avatar.tsx:76` render `<img>` khi có `src`; sidebar, AccountSheet, admin, approval, ProfileTab đã truyền avatar url.
- `PATCH /api/me/profile` cố ý từ chối `avatar_url` (`auth/internal/rest/workspaces.go:44-48`, `avatar_not_supported`): chưa có nơi lưu, URL tự gõ sẽ bị trình duyệt đồng nghiệp tải.
- Mockup đã duyệt `design/mockups/contacts-documents-settings.html:830`: avatar 64 + "Đổi ảnh" · "Bỏ ảnh", "JPG hoặc PNG, tối đa 5 MB". Không có màn cắt.
- `ProfileTab.tsx:98-103` chỉ hiện avatar + câu chú thích, chưa có nút.

## Contract
- **Outcome:** Cài đặt → Hồ sơ: "Đổi ảnh" (JPG/PNG ≤5 MB) và "Bỏ ảnh". Ảnh mới hiện ngay ở sidebar/AccountSheet của chính mình; nơi khác (danh bạ, admin, approval, cuộc gọi) ở lần tải dữ liệu kế.
- **Constraints:** chỉ ảnh của chính mình (theo token, không NGAC — như UpdateProfile). URL do server tạo; PATCH vẫn từ chối `avatar_url`. Server decode thật, center-crop vuông, resize 256px, re-encode (bỏ EXIF/GPS). Layering transport→domain→store. Route mới vào `frontend/vite.config.js`. Migration mới áp bằng `make db-migrate` và kiểm trên DB chạy. Cập nhật `docs/specs/contacts-documents-settings-screens/spec.md`.
- **Non-goals:** cropper/zoom; admin đổi ảnh người khác; ảnh theo workspace; GIF động; realtime push sang client khác; dọn rác ảnh cũ theo lịch (ảnh cũ xoá ngay khi thay/bỏ).
- **Acceptance:** upload PNG hợp lệ → `avatar_url` đổi, sidebar hiện ảnh không reload; >5 MB / không phải ảnh / giả đuôi → 400 + thông báo tiếng Việt; "Bỏ ảnh" → chữ cái đầu; `GET` ảnh có `Cache-Control: immutable` + content-type đúng, hash lạ → 404; test deny-case cho mọi nhánh từ chối; `make test s=auth`, `npm test`, `make check-layering`, `make build-check` xanh.

## Phương án
| | Cách | Hỏng trước tiên khi |
|---|---|---|
| **A (chọn)** | Bảng `user_avatars(hash PK, user_id, content_type, bytes, created_at)`; `PUT /api/me/avatar` (multipart), `DELETE /api/me/avatar`; `GET /api/avatars/:hash` public, immutable; `avatar_url = /api/avatars/<sha256>` | Lưu lượng ảnh rất lớn qua auth — cache immutable giảm gần hết; chuyển sang MinIO sau chỉ đổi store + URL |
| B | MinIO bucket public-read qua `/storage` Traefik | Phải đổi bucket policy dev+prod, rủi ro mở nhầm bucket |
| C | Presign upload thẳng MinIO | Mất kiểm tra/re-encode phía server — trái lý do code hiện tại từ chối URL |

Lý do chọn A: không đụng hạ tầng, giữ lớp kiểm tra server, rẻ nhất để bỏ. Hiển thị: ai có URL thì xem được (hash không đoán được) — chấp nhận như Slack.

## Ghi chú cho plan
- `GET /api/avatars/:hash` không cần token (thẻ `<img>` không gửi Bearer) → route public trong auth; thêm vào vite proxy và Traefik rule của auth nếu prefix `/api/avatars` chưa route.
- Frontend: `useSaveAvatar`/`useRemoveAvatar` invalidate `me` + contacts; validate loại/kích thước phía client trước khi gửi (server vẫn là chuẩn).
- Thư viện resize: `golang.org/x/image/draw` (chỉ thêm vào `services/auth/go.mod`).

## Câu hỏi còn mở
- Định dạng đầu ra: JPEG (nhỏ, không alpha) hay PNG giữ nền trong suốt? Đề xuất JPEG q≈85.
