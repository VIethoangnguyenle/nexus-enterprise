---
title: "Đổi ảnh đại diện"
status: completed
created: 2026-10-14
mode: fast (codebase-driven) + tdd, red-team applied 2026-10-14
spec:
  - docs/specs/tenant-auth-flow/spec.md            # API: endpoints, lỗi, cache, deny cases
  - docs/specs/contacts-documents-settings-screens/spec.md  # UI Hồ sơ
  - docs/specs/sign-in-screens/spec.md             # bỏ câu "server không có kho ảnh"
brainstorm: plans/reports/brainstorm-261014-2247-avatar-upload.md
blockedBy: []
blocks: []
---

# Đổi ảnh đại diện

Người dùng tự đổi/bỏ ảnh đại diện của chính mình ở Cài đặt → Hồ sơ. Lúc upload, server tạo **3 bản**
(64 / 192 / 1024), lưu trong Postgres, phục vụ bởi auth qua URL token ngẫu nhiên (cache immutable). Mỗi chỗ hiển thị
chỉ tải bản vừa đủ; bản 1024 để dành cho màn xem chi tiết (plan sau — không làm ở đây).

## Phases

| # | Phase | Status | Depends |
|---|-------|--------|---------|
| 1 | [Schema + store](phase-01-schema-store.md) | completed | — |
| 2 | [Domain xử lý ảnh + REST](phase-02-domain-rest.md) | completed | 1 |
| 3 | [Frontend + spec](phase-03-frontend-spec.md) | completed | 2 |

## Contract

- **Outcome:** "Đổi ảnh" (JPG/PNG ≤ 5 MB) và "Bỏ ảnh" trong tab Hồ sơ, đúng mockup
  `design/mockups/contacts-documents-settings.html:830`. Ảnh hiện ngay ở chỗ của chính mình; ở mọi `<Avatar>` khác có
  người cụ thể (danh bạ, chat, feed, cuộc gọi, phê duyệt…) từ lần tải dữ liệu kế. URL chết → chữ cái đầu, không bao giờ vòng trống.
- **Constraints:** chỉ ảnh của chính mình (theo token, không NGAC); URL do server tạo, `PATCH /api/me/profile` vẫn từ chối
  `avatar_url`; auth không được OOM (prod 128M → nâng 192M, xem phase 2); re-encode JPEG q=85, bỏ EXIF nhưng áp Orientation;
  layering transport→domain→store.
- **Non-goals:** màn xem chi tiết avatar (plan riêng, dùng bản 1024 đã lưu); cropper/zoom; admin đổi ảnh người khác;
  ảnh theo workspace; GIF động; realtime push; giữ ảnh cũ sau khi thay (onError fallback thay thế).
- **Acceptance:** Success Criteria từng phase; tổng: `make test s=auth`, `make check-layering`, `make build-check`,
  `make check-ngac`, `cd frontend && npm test && npm run lint && npm run build` xanh; migration áp bằng `make db-migrate`
  và quan sát trên DB; thử tay: đổi → bỏ → đổi lại, ảnh dọc chụp điện thoại đúng chiều; đo network: list 24px tải bản 64.

## API (chốt)

| Method | Path | Auth | Trả về |
|--------|------|------|--------|
| PUT | `/api/me/avatar` | JWT | multipart `file` → `200 {"avatar_url": "/api/auth/avatars/<token>"}` |
| DELETE | `/api/me/avatar` | JWT | `200 {"avatar_url": ""}` |
| GET | `/api/auth/avatars/:token/:size` | public, limiter | `size ∈ {64,192,1024}`; `image/jpeg` |
| GET | `/api/auth/avatars/:token` | public, limiter | = bản 192 (cho nơi dùng thẳng `avatar_url`, vd màn cuộc gọi đến) |

- `token` = 128-bit ngẫu nhiên (32 hex) mỗi lần upload — **không** phải hash nội dung (tránh dò ảnh từ ảnh gốc công khai).
- `users.avatar_url` lưu URL gốc không size; frontend nối `/64` hoặc `/192`.
- Header 200: `Cache-Control: public, max-age=31536000, immutable`, `X-Content-Type-Options: nosniff`,
  `Content-Security-Policy: default-src 'none'`, `Cross-Origin-Resource-Policy: same-origin`,
  `Content-Disposition: inline; filename="avatar.jpg"`. 404: `Cache-Control: public, max-age=60`.
- Lỗi (envelope `{message, code}`): `avatar_too_large` 413, `avatar_missing` 400, `avatar_not_image` 400,
  `avatar_too_many_pixels` 400, `avatar_busy` 503 (+Retry-After, sau khi chờ 5 s), `rate_limited` 429 (sẵn có), user đã xoá → 404.

**Routing — không thêm entry proxy:** `/api/me/*` và `/api/auth/*` đã trỏ về auth: `frontend/vite.config.js:42,50`,
Traefik `docker-compose.yml:245`, nginx `deploy/nginx/default.conf:105,116` (`client_max_body_size 10m`).
`routing_test.go` (`TestTraefikRoutesEveryAuthRouteToTheAuthService`) tự kiểm route mới. Service worker không cache `/api` (`frontend/src/sw.ts`).

## Deploy & rollback

- Migration 055 chỉ thêm bảng → an toàn cho image cũ.
- Prod: nâng auth `limits.memory` 128M → 192M, `GOMEMLIMIT` 100MiB → 160MiB (`deploy/docker-compose.prod.yml:267,279`).
- Rollback (auto hoặc `ROLLBACK=1`, không chạy migration — `docs/deployment.md:227`): image cũ không có route GET →
  mọi avatar đã upload 404. Fallback `onError` (phase 3) giữ chữ cái đầu; dọn dữ liệu nếu revert lâu:
  `UPDATE users SET avatar_url='' WHERE avatar_url LIKE '/api/auth/avatars/%';` — ghi vào `docs/deployment.md`.

## Rủi ro chính

- Bộ nhớ decode → ngân sách byte theo color model + một decode tại một thời điểm + thu nhỏ bằng halving (phase 2).
- Ai có URL xem được ảnh — chấp nhận (token ngẫu nhiên); "Bỏ ảnh" không xoá bản đã nằm trong cache trình duyệt — ghi vào spec.

## Red Team Review

### Session — 2026-10-14
**Findings:** 15 (15 accepted, 0 rejected; #3 accepted một phần) · **Severity:** 1 Critical, 5 High, 9 Medium
Reviewers: Security Adversary, Assumption Destroyer, Failure Mode Analyst. Evidence đã đối chiếu tay các điểm then chốt.

| # | Finding | Sev | Disposition | Applied To |
|---|---------|-----|-------------|------------|
| 1 | 40 MP cap vs auth 128M prod → OOM sập đăng nhập | Critical | Accept | Phase 2, plan Deploy |
| 2 | PUT/GET không rate limit | High | Accept | Phase 2 |
| 3 | Thay ảnh → URL cũ 404, `Avatar` không onError | High | Accept (onError; không giữ blob cũ) | Phase 3 |
| 4 | Rollback để lại URL treo | High | Accept | plan Deploy & rollback |
| 5 | 2 fake store test vỡ biên dịch | High | Accept | Phase 1 |
| 6 | Lỗi mới rơi `mapError` default → 500; thứ tự lỗi multipart | High | Accept | Phase 2 |
| 7 | Thứ tự khoá tx / FK 23503 → 500 | Medium | Accept | Phase 1 |
| 8 | sha256 nội dung dò được từ ảnh gốc | Medium | Accept → token ngẫu nhiên | Phase 1, 2, plan |
| 9 | Mất EXIF Orientation → ảnh điện thoại nằm ngang | Medium | Accept | Phase 2 |
| 10 | Content-Type từ cột text tự do | Medium | Accept → bỏ cột, cứng jpeg + CORP | Phase 1, 2 |
| 11 | Sai về `apiUpload`, hook, query key | Medium | Accept | Phase 3 |
| 12 | Nhiều `<Avatar>` không truyền `src` | Medium | Accept | Phase 3 |
| 13 | Spec `tenant-auth-flow`, `sign-in-screens` lỗi thời | Medium | Accept | Phase 3, frontmatter |
| 14 | Lệch mockup (Bỏ ảnh luôn hiện, icon camera) | Medium | Accept → Bỏ ảnh disabled khi chưa có ảnh | Phase 3 |
| 15 | `golang.org/x/image` cần mạng; CPU/mem chưa đo | Medium | Accept | Phase 2 |

Thay đổi theo ý người dùng cùng phiên: lưu 3 bản 64/192/1024 thay vì 1 bản 256 (avatar nhỏ không tải bản lớn); màn xem chi tiết để plan sau.

### Whole-Plan Consistency Sweep
Đã đọc lại plan.md + 3 phase sau khi áp. Thuật ngữ cũ đã thay hết: "hash" (khoá URL) → "token"; "256×256" → 3 bản 64/192/1024;
"40 MP" → ngân sách byte; `content_type` column → bỏ; `keys.auth.profile` → `keys.auth.profiles()`; hook ở `useProfile.ts`;
`apiUpload` có tham số method. Không còn mâu thuẫn chưa giải quyết.

## Validation Log

### Session 1 — 2026-10-14
Verification (guard: red-team đã có evidence → chỉ kiểm claim thêm sau red-team): Tier Standard, 3 claims —
Verified 2 | Failed 1 | Unverified 0.
- Failed: "miniredis" — repo không có; test dùng Redis thật (`grpc/server_test.go:24`). Đã sửa sang Redis thật.
- Verified: `publicLimiter` một hạn mức chung 60/phút (`domain/ratelimit.go:22`) → phase 2 đã yêu cầu tham số limit riêng;
  `ErrNoSuchUser` → `ErrNotFound` (`domain/profile.go:57`) → user đã xoá trả 404.

| # | Câu hỏi | Quyết định | Áp vào |
|---|---------|-----------|--------|
| 1 | RAM auth prod | Nâng 192M / GOMEMLIMIT 160MiB, ngân sách decode 48 MB | Phase 2, Deploy (giữ nguyên) |
| 2 | Khi đang bận | Chờ tối đa 5 s rồi mới `avatar_busy` | Phase 2, API |
| 3 | ~15 call site `<Avatar>` | Làm trong plan này | Phase 3 (giữ nguyên) |
| 4 | Test rate limit | Redis thật theo mẫu sẵn có | Phase 2 |

### Whole-Plan Consistency Sweep
Đã đọc lại plan.md + 3 phase: không còn "miniredis", "ngay (không xếp hàng)", "401/404"; mọi chỗ nói về bận đều là "chờ 5 s".
Không còn mâu thuẫn.

## Review sau cài đặt (2026-10-15)

Tester: mọi gate xanh, không skip; báo thiếu test cho một số nhánh lỗi. Code-reviewer: 1 High, 2 Medium, đã sửa:

| Phát hiện | Sửa |
|-----------|-----|
| High: byte đệm `0xFF` trước SOF làm parser trượt → JPEG progressive bị ước lượng như baseline → 205 MB RSS > 192M (auth OOM) | Parser bỏ qua byte đệm; header không đọc được → tính trường hợp xấu nhất (full-size, progressive). Test + phát lại đúng file của reviewer → 400 |
| Medium: slot giải nén không trả lại khi decoder panic | `defer release()` |
| Medium: `x/image v0.47.0` ép module auth lên `go 1.26.0`, CI ghim 1.25 | Hạ về `x/image v0.45.0` (go 1.25.0); `go.mod` giữ `go 1.25.0` |
| Low: thông báo `avatar_not_supported` lỗi thời | Đổi message, giữ mã |
| Tester: thiếu hướng EXIF 2/4/5/7, thiếu nhánh Bỏ ảnh thất bại | Thêm test đủ 8 hướng (ảnh mẫu thêm góc xanh lá để phân biệt lật/xoay) và test Bỏ ảnh lỗi |

Còn lại, chấp nhận: client ngắt kết nối khi đang chờ slot → log 500 (không tới người dùng); thiếu test cho lỗi Redis/DB hiếm trong rate limit và GET.

## Câu hỏi mở

Không.
