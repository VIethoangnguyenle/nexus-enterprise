---
phase: 2
title: "Domain xử lý ảnh + REST"
status: completed
priority: P1
effort: "5h"
dependencies: [1]
---

# Phase 2: Domain xử lý ảnh + REST

## Overview
Domain nhận bytes, kiểm tra trong ngân sách bộ nhớ, xoay theo EXIF, cắt vuông, tạo 3 bản 1024/192/64, giao store.
REST mở `PUT/DELETE /api/me/avatar` và `GET /api/auth/avatars/:token[/:size]`. `PATCH /api/me/profile` vẫn từ chối `avatar_url`.

## Requirements
**Bộ nhớ (auth prod chỉ 128M — `deploy/docker-compose.prod.yml:267,279`):**
- Trước khi decode: `image.DecodeConfig` → định dạng phải là `jpeg`/`png` (theo nội dung, không tin đuôi/Content-Type).
- Ước lượng byte giải nén = W×H×bpp, bpp theo `ColorModel`: Gray 1; YCbCr 1.5 (JPEG baseline) — **JPEG progressive**
  (dò marker SOF2 trong header) +6 cho bộ đệm hệ số; RGBA/NRGBA/Paletted 4; 16-bit (RGBA64/NRGBA64/Gray16) 8.
  Vượt **48 MB** → `ErrAvatarTooManyPixels`. (12 MP JPEG baseline ≈ 18 MB qua; PNG 40 MP 160 MB bị chặn.)
- Một decode tại một thời điểm: semaphore kích thước 1 trong `Service`; đang bận → chờ tối đa **5 s** (tôn trọng `ctx`),
  hết hạn → `ErrAvatarBusy`. <!-- Updated: Validation Session 1 - chờ 5 s thay vì báo bận ngay -->
- Thu nhỏ **không** dùng kernel scaler (`CatmullRom`/`BiLinear` cấp `dw*sh*32 B` tạm — hàng chục MB). Dùng
  `draw.ApproxBiLinear` theo bước halving (mỗi bước ÷2) đến ≤ 2× đích rồi một bước cuối; 1024 → 192 → 64 từ bản trước.
- Nâng auth prod: `limits.memory` 192M, `GOMEMLIMIT: 160MiB` (đỉnh ước tính: 48 MB decode + 12 MB bước halving đầu
  + 5 MB file + nền ~30 MB).

**Ảnh ra:**
- Áp EXIF Orientation (1–8) từ APP1 của JPEG trước khi cắt (parser nhỏ tự viết, chỉ đọc tag 0x0112; lỗi parse → coi như 1).
- Center-crop vuông → 1024, 192, 64; ảnh nhỏ hơn 1024 vẫn scale lên (một bộ kích thước cố định).
- Nền trong suốt tô trắng — chỉ trên ảnh đã thu nhỏ, không trên ảnh full.
- JPEG q=85. Re-encode bỏ toàn bộ EXIF/GPS.
- Token: 16 byte `crypto/rand` → hex. URL `/api/auth/avatars/<token>`.

**Giới hạn tần suất:**
- PUT: 10 lần/giờ/người qua Redis (mẫu `takeWorkspaceCreation`, `domain/workspaces.go:104`) → `ErrRateLimited` (429 + Retry-After sẵn có).
  Không Redis (unit test) → không giới hạn, như mẫu.
- GET: `publicLimiter("avatar")` với hạn mức riêng rộng (≥ 600/phút/IP — trang danh bạ tải hàng chục ảnh); nếu
  `publicLimiter` (`rest/ratelimit.go:36`) chỉ có một hạn mức chung thì thêm tham số limit, không viết limiter mới.

**REST — chỉ REST giữ giới hạn byte:**
- `http.MaxBytesReader(5 MiB + 64 KiB)` bọc body trước `c.FormFile("file")`; sau đó kiểm `fh.Size <= 5 MiB`.
- Thứ tự map lỗi `FormFile`: `errors.As(*http.MaxBytesError)` → 413 `avatar_too_large`; `errors.Is(http.ErrMissingFile)` →
  400 `avatar_missing`; mọi lỗi multipart khác (`ErrNotMultipart`, EOF giữa chừng, thiếu boundary) → 400 `invalid_input`.
- Domain errors vào `mapError` (`rest/handler.go`, trước `default: internalFailure`): `ErrAvatarNotImage` 400 `avatar_not_image`,
  `ErrAvatarTooManyPixels` 400 `avatar_too_many_pixels`, `ErrAvatarBusy` 503 `avatar_busy` + `Retry-After: 2`.
- GET: token phải khớp `^[0-9a-f]{32}$`, size ∈ {64,192,1024} (không size = 192); sai/không có → 404 + `Cache-Control: public, max-age=60`.
  200: `Content-Type: image/jpeg` (cứng), `Cache-Control: public, max-age=31536000, immutable`, `X-Content-Type-Options: nosniff`,
  `Content-Security-Policy: default-src 'none'`, `Cross-Origin-Resource-Policy: same-origin`, `Content-Disposition: inline; filename="avatar.jpg"`.

## Architecture
- `backend/services/auth/internal/domain/avatar.go`
  - Errors: `ErrAvatarNotImage`, `ErrAvatarTooManyPixels`, `ErrAvatarBusy`.
  - `normalizeAvatar(b []byte) (store.AvatarImages, error)` — thuần, test không cần DB.
  - `jpegOrientation(b []byte) int`, `isProgressiveJPEG(b []byte) bool`, `decodedBytes(cfg, format, progressive) int64` — helper thuần.
  - `(s *Service) SetAvatar(ctx, userID string, b []byte) (url string, err error)` — rate limit → semaphore → normalize → token → store.
  - `(s *Service) ClearAvatar(ctx, userID string) error`
  - `(s *Service) Avatar(ctx, token string, size int) ([]byte, error)` → `ErrNotFound` (domain) khi không có.
- `backend/services/auth/internal/rest/avatar.go`: `UploadAvatar`, `RemoveAvatar`, `ServeAvatar`.
- `rest/handler.go` RegisterRoutes: khối public
  `e.GET("/api/auth/avatars/:token", h.ServeAvatar, h.publicLimiter(...))` và `.../:token/:size`;
  nhóm JWT: `api.PUT("/me/avatar", h.UploadAvatar)`, `api.DELETE("/me/avatar", h.RemoveAvatar)`.
- `rest/workspaces.go:44`: sửa comment (đã có nơi lưu ảnh; vẫn từ chối URL tự gõ vì lý do cũ còn đúng). Mã lỗi giữ nguyên.

## Related Code Files
- Create: `backend/services/auth/internal/domain/avatar.go`, `domain/avatar_test.go`, `domain/testdata/` (ảnh EXIF orientation 6/8
  nếu không sinh được trong test), `backend/services/auth/internal/rest/avatar.go`, `rest/avatar_test.go`
- Modify: `rest/handler.go` (routes, `mapError`), `rest/ratelimit.go` (nếu cần tham số limit), `rest/workspaces.go` (comment),
  `rest/account_test.go` (đổi tên `TestProfile_RefusesAnAvatarUntilThereIsWhereToKeepOne` — vẫn từ chối),
  `backend/services/auth/go.mod`/`go.sum` (+ `golang.org/x/image`), `deploy/docker-compose.prod.yml` (memory auth),
  `docs/deployment.md` (lệnh dọn `avatar_url` khi rollback)

## Implementation Steps
1. `go get golang.org/x/image@latest` trong `services/auth` (**cần mạng**; Dockerfile đã `go mod download` ở `services/auth/Dockerfile:9-10`).
2. TDD `domain/avatar_test.go` (ảnh sinh bằng `image.NewRGBA`/`NewNRGBA64` + encode trong test):
   - PNG 600×300 → 3 JPEG decode được, đúng 1024²/192²/64²;
   - JPEG có EXIF → output không còn `Exif`; Orientation 6 và 8 → kiểm vị trí một pixel đánh dấu sau xoay;
   - bytes rác, GIF, WebP hợp lệ → `ErrAvatarNotImage`;
   - PNG header khai 10000×10000 → `ErrAvatarTooManyPixels` mà không decode đầy đủ; PNG 16-bit ngay trên ngân sách → bị chặn,
     ngay dưới → qua; JPEG progressive vs baseline cùng kích thước → khác kết quả đúng như bpp;
   - PNG trong suốt → pixel góc trắng;
   - **bộ nhớ:** normalize một JPEG baseline 12 MP và một PNG sát ngân sách, đo `runtime.ReadMemStats` (`TotalAlloc`/peak
     `HeapInuse` sau `runtime.GC()`) → đỉnh < 120 MB; ghi lại thời gian xử lý vào báo cáo phase;
   - semaphore: hai `SetAvatar` song song (normalize bị chặn bằng hook test, thời gian chờ inject được để test nhanh) →
     người sau chờ rồi thành công khi người trước xong trong hạn; quá hạn → `ErrAvatarBusy`;
   - rate limit: lần 11 trong giờ → `ErrRateLimited`. Redis **thật** theo mẫu `testRedisClient` (`grpc/server_test.go:24`, DB 14,
     tự skip khi không có Redis) — bật Redis dev và xác nhận `-v` không SKIP. <!-- Updated: Validation Session 1 - không dùng miniredis -->
3. Implement domain cho xanh.
4. TDD `rest/avatar_test.go` (kiểu `account_test.go`), deny-case cho mọi nhánh:
   PUT không token → 401; body > 5 MiB+64 KiB → 413 `avatar_too_large`; file đúng 5 MiB+1 → 413; 5 MiB−1 hợp lệ → 200;
   body JSON → 400 `invalid_input`; multipart bị cắt giữa chừng → 400 `invalid_input` (không 500); thiếu field → 400 `avatar_missing`;
   `.png` chứa text → 400 `avatar_not_image`; quá pixel → 400 `avatar_too_many_pixels`; bận → 503 `avatar_busy`;
   user đã xoá (token còn hạn) → 404 (domain đổi `store.ErrNoSuchUser` → `ErrNotFound` như `domain/profile.go:57`), không 500; hợp lệ → 200 + `avatar_url` đúng dạng;
   DELETE → 200 `""`; GET token sai dạng / không có / size lạ → 404 + `max-age=60`; GET có → 200 + đủ 6 header, cả có và không size;
   PATCH profile kèm `avatar_url` → vẫn 400 `avatar_not_supported`.
5. Implement handler + routes + `mapError`. `go mod tidy`.
6. Sửa `deploy/docker-compose.prod.yml` (auth 192M / 160MiB) và thêm mục rollback vào `docs/deployment.md`.
7. `make test s=auth`, `make check-layering`, `make build-check`, `make check-ngac`.

## Success Criteria
- [x] Mọi test trên xanh; không nhánh từ chối nào thiếu test.
- [x] Test bộ nhớ chứng minh đỉnh < 120 MB với ảnh sát ngân sách.
- [x] Không SQL trong `rest/`; REST gọi domain, không gọi store.
- [x] `curl` thật vào auth đang chạy: PUT ảnh dọc chụp điện thoại → GET `/64`, `/192`, `/1024` trả JPEG đúng kích thước, đúng chiều.
- [~] `docker stats`: dev chạy native, không đo được; thay bằng test cấp phát (ảnh 12 MP: 35 MB, 0,18 s). Đo lại trên prod sau deploy.

## Risk Assessment
- Thiếu import side-effect `image/jpeg`, `image/png` → mọi ảnh thành NotImage. Test bước 2 bắt.
- Ước lượng bpp sai với một biến thể JPEG (CMYK, 4:4:4) → đỉnh vượt ngân sách; tín hiệu: test bộ nhớ/`docker stats` vượt.
  Xử lý: tăng bpp cho model đó (CMYK = 4 + 8 progressive), không nới ngân sách.
- JPEG lạ decode lỗi → `ErrAvatarNotImage`, chấp nhận (hiếm).
- Không có mạng để `go get` → dừng và báo; không vendor tay.
- Nâng memory auth cần VPS còn chỗ; tín hiệu: deploy health check fail. Xử lý: hạ ngân sách decode xuống 32 MB, giữ 128M.

## Kết quả (2026-10-14)
- 363 test auth xanh (Redis + DB thật); check-layering, check-ngac, build-check sạch.
- E2E qua Vite proxy, auth dev chạy bản mới: ảnh 4032×3024 EXIF Orientation=6 + GPS → 200 trong 0,21 s; 64/192/1024 = 680 B / 1,2 KB / 17 KB, đúng chiều, không EXIF; PNG trong suốt → trắng; thay ảnh → URL cũ 404; Bỏ ảnh → avatar_url rỗng, 0 dòng; file giả → 400 avatar_not_image; không token → 401; không có 500 trong log.
- Lệch plan: xoay EXIF áp lên bản 1024 (không phải ảnh gốc) — ô vuông giữa không đổi khi xoay, tiết kiệm một bản sao full-size. Ước lượng JPEG đọc hệ số lấy mẫu từ SOF.
- Dùng `golang.org/x/image v0.45.0` (bản mới nhất còn `go 1.25`, khớp CI); kéo theo `x/sync` 0.20→0.22, `x/sys` 0.43→0.47, `x/text` 0.36→0.41 (indirect, chỉ module auth).
- Tài khoản thử `avatar-e2e-*@example.test` còn trong DB dev.
