---
phase: 1
title: "Schema + store"
status: completed
priority: P1
effort: "2h"
dependencies: []
---

# Phase 1: Schema + store

## Overview
Bảng `user_avatars` (3 bản ảnh/người) và ba hàm store: đặt ảnh, bỏ ảnh, đọc một bản theo token + size. Đặt/bỏ cập nhật
`users.avatar_url` trong cùng transaction, khoá `users` trước để có một thứ tự khoá duy nhất.

## Requirements
- Một người tối đa một ảnh (3 bản); thay ảnh ghi đè (upsert theo `user_id`), token mới mỗi lần.
- Xoá user → xoá ảnh (FK `ON DELETE CASCADE`).
- `users.avatar_url` và `user_avatars` luôn khớp (một transaction).
- User không tồn tại → `store.ErrNoSuchUser` (sentinel sẵn có, `store/store.go:65`), không bao giờ lỗi FK thô.

## Architecture
```sql
-- data/migrations/055_user_avatars.sql  (chỉ thêm — an toàn cho image cũ)
CREATE TABLE IF NOT EXISTS user_avatars (
  user_id    TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,  -- users.id là TEXT (init.sql:44)
  token      TEXT NOT NULL UNIQUE,     -- 32 hex ngẫu nhiên, khoá của URL
  small      BYTEA NOT NULL,           -- 64×64 JPEG
  medium     BYTEA NOT NULL,           -- 192×192 JPEG
  large      BYTEA NOT NULL,           -- 1024×1024 JPEG (màn xem chi tiết, plan sau)
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```
Không có cột content type: chỉ có JPEG, handler ghi cứng `image/jpeg`.

Store (`backend/services/auth/internal/store/avatar.go`):
- `type AvatarImages struct{ Small, Medium, Large []byte }`
- `SetAvatar(ctx, userID, token string, img AvatarImages, url string) error` — tx:
  1. `UPDATE users SET avatar_url=$2 WHERE id=$1` — `RowsAffected()==0` → `ErrNoSuchUser` (khoá users trước).
  2. `INSERT … ON CONFLICT (user_id) DO UPDATE SET token, small, medium, large, created_at=now()`.
- `ClearAvatar(ctx, userID) error` — tx cùng thứ tự: `UPDATE users SET avatar_url=''` (0 dòng → `ErrNoSuchUser`), rồi
  `DELETE FROM user_avatars WHERE user_id=$1`. Không có ảnh vẫn thành công.
- `GetAvatar(ctx, token string, size int) ([]byte, error)` — chọn cột theo size từ một map cố định
  {64:"small",192:"medium",1024:"large"} (size lạ → `nil, nil`; không bao giờ ghép chuỗi từ input). Không có → `nil, nil`
  (quy ước `GetUserByID`, `store.go:194`).
- Thêm 3 method vào interface `AuthStore` (`domain/service.go:15-43`).

## Related Code Files
- Create: `data/migrations/055_user_avatars.sql`, `backend/services/auth/internal/store/avatar.go`,
  `backend/services/auth/internal/store/avatar_test.go`
- Modify: `backend/services/auth/internal/domain/service.go` (interface),
  `backend/services/auth/internal/domain/fakes_test.go` (`fakeWorld` — in-memory 3 method),
  `backend/services/auth/internal/rest/account_test.go` (`acctStore` — in-memory 3 method)

## Implementation Steps
1. Backup DB dev (`pg_dump` vào scratchpad) trước khi migrate.
2. Viết migration; `make db-migrate`; `\d user_avatars` xác nhận; chạy lại `make db-migrate` lần 2 không lỗi.
3. Thêm 3 method vào interface + hai fake (để `domain` và `rest` test còn biên dịch).
4. TDD `avatar_test.go` (fixture như `profile_test.go`; **đặt `TEST_DATABASE_URL` cổng 5433**, nếu không test skip im lặng):
   - set → `users.avatar_url = url`, GetAvatar trả đúng bytes từng size;
   - set lần 2 → token cũ không còn (GetAvatar token cũ → nil);
   - clear → url rỗng, row mất; clear khi không có ảnh → nil;
   - set/clear với user không tồn tại → `ErrNoSuchUser`, không row nào được ghi;
   - token lạ / size lạ (vd 100) → nil;
   - xoá user → row mất;
   - set và clear chạy đồng thời 20 lần (goroutine) → không deadlock, trạng thái cuối khớp (`avatar_url` rỗng ⇔ không row).
5. Implement cho xanh. Không bỏ qua lỗi (`check-layering` bắt dropped store errors).

## Success Criteria
- [x] Migration áp được trên DB chạy, chạy lại không lỗi.
- [x] `go test -v ./internal/store/ -run Avatar` trong `services/auth` chạy thật (không SKIP) và xanh.
- [x] `make test s=auth` biên dịch và xanh (fake đã cập nhật).
- [x] `make check-layering` xanh.

## Risk Assessment
- Test DB skip im lặng → tưởng xanh; tín hiệu: `-v` báo SKIP. Xử lý: đặt `TEST_DATABASE_URL` rồi chạy lại.
- Deadlock vẫn xảy ra (vd cascade xoá user): tín hiệu: test đồng thời lỗi 40P01. Xử lý: cả hai tx bắt đầu bằng
  `SELECT 1 FROM users WHERE id=$1 FOR UPDATE`.
