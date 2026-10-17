---
phase: 6
title: "Thumbnail: backend drive"
status: pending
priority: P1
effort: "1.5d"
dependencies: [1]
---

# Phase 06: Thumbnail — backend drive

## Overview
`GET /api/drive/files/:fileId/thumbnail?size=320|640` trả WebP/JPEG; sinh lười, cache MinIO, quyền như preview.

## Hiện trạng (đã kiểm)
- `backend/services/drive/internal/rest/handler.go:97` preview; `domain/preview.go`, `preview_kind.go`; Gotenberg nội bộ (plan `261011-1035-file-preview`).
- `backend/services/drive/Dockerfile`: builder `golang:1.26-alpine`; runtime cần xem (thêm `poppler-utils`, `ffmpeg` qua `apk`).
- `/api/drive` đã proxy 8185 (`frontend/vite.config.js:117`).

## Requirements
- Quyền: `read` trên OA cha, kiểm như `GetPreview` (dùng chung hàm), fail-closed. 403 không lộ tên/loại.
- Sinh theo loại: ảnh (Go decode + scale, dùng `backend/pkg/imagenorm` nếu hợp, không crop vuông — giữ tỉ lệ, cạnh dài =
  size); PDF: `pdftoppm -f 1 -l 1 -scale-to <size> -jpeg`; Office: PDF cache của preview (Gotenberg) → `pdftoppm`;
  video: `ffmpeg -ss 1 -frames:v 1 -vf scale=<size>:-2`. Mọi lệnh ngoài chạy với `context` timeout (10 s ảnh/PDF/video,
  Office theo giới hạn 45 s sẵn có), không qua shell, đường dẫn tạm riêng, dọn sau.
- Cache MinIO `thumbs/<fileId>/<version>/<size>.jpg`; tệp đổi phiên bản → khoá mới.
- Giới hạn đồng thời (semaphore, vd 4) + hàng chờ có timeout → quá tải trả 503 `Retry-After`; frontend coi như lỗi tạm.
- Không hỗ trợ / lỗi sinh → 404 `thumbnail_unavailable` (frontend rơi về icon); ghi log nguyên nhân.
- `Cache-Control: private, max-age=86400` + ETag theo version.
- Dockerfile + `docs/` hướng dẫn dev native cài `poppler-utils`, `ffmpeg`; khi thiếu binary → 404 thay vì 500.

## Implementation Steps
1. Test trước (domain): allow trả ảnh; **deny** 403; lần hai lấy từ cache (đếm lần gọi bộ sinh giả); version mới sinh
   lại; loại không hỗ trợ 404; binary thiếu 404; quá tải 503.
2. Interface `thumbnailer` (ảnh, pdf, video) để test không cần binary thật; một test tích hợp có `t.Skip` khi thiếu binary.
3. Domain + store MinIO + REST route.
4. Dockerfile; kiểm image build (`docker build`) và kích thước.
5. Spec `docs/specs/file-preview/spec.md` thêm mục Thumbnail.
6. `make build-check`, `make check-layering`, `make test s=drive`.

## Related Code Files
- Create: `backend/services/drive/internal/domain/thumbnail*.go` (+ test), `internal/rest/thumbnail.go` (+ test)
- Modify: `internal/rest/handler.go` (route), `backend/services/drive/Dockerfile`, `docs/specs/file-preview/spec.md`, docs dev

## Success Criteria
- [ ] JPG/PNG/WebP, PDF, DOCX/XLSX/PPTX, MP4 có thumbnail thật trên stack `make dev`.
- [ ] Test deny + cache + fallback xanh.
- [ ] Image drive build được; tăng kích thước ghi trong PR.

## Risk Assessment
- PDF độc hại / quá nặng → timeout + giới hạn trang 1 + chạy không quyền ghi ngoài thư mục tạm.
- CPU tăng khi mở thư mục lớn lần đầu → semaphore; nếu đo thấy chậm, sinh trước sau upload (để sau, không làm ở đây).
