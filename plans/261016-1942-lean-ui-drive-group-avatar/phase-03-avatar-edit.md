---
phase: 3
title: "AvatarEdit dùng chung + chuẩn hoá ảnh thành backend/pkg"
status: pending
priority: P1
effort: "6h"
dependencies: [1]
---

# Phase 03: AvatarEdit tinh gọn + tách chuẩn hoá ảnh

## Overview
Một primitive frontend cho "chạm ảnh để đổi" dùng ở Hồ sơ (nay) và nhóm (phase 05). Phía backend, tách hàm chuẩn
hoá ảnh của auth để messaging dùng lại mà không chép.

## Hiện trạng (đã kiểm)
- `frontend/src/components/settings/ProfileTab.tsx:172` `AvatarEdit`: Avatar 64 + Button "Đổi ảnh" + "Bỏ ảnh" + input file ẩn.
- `backend/services/auth/internal/domain/avatar.go:184` `normalizeAvatar` → `store.AvatarImages` (3 cỡ JPEG);
  `halveTo`, `onWhite`, `orient`, `jpegOrientation`, `checkDecodeBudget` là hàm thuần.

## Requirements
- Frontend: `components/patterns/AvatarEdit.tsx` — props `name, hueKey, src, label ("Đổi ảnh đại diện" | "Đổi ảnh nhóm"),
  canEdit, onUpload(file), onRemove(), busy, progress?, error?`, `shape: 'circle' | 'square'`. Một `button` duy nhất
  bọc ảnh (`aria-haspopup="menu"`), mở menu (≥ 768) hoặc sheet (< 768) theo DESIGN.md. `canEdit=false` → chỉ Avatar.
- Validate file dùng lại `validateAvatarFile`/`avatarErrorMessage` (chuyển từ `settings-model.ts` sang
  `lib/avatar-file.ts`; settings-model re-export để không đổi import cũ, hoặc sửa mọi import).
- Backend: `backend/pkg/imagenorm` (tên theo quy ước pkg hiện có): `Square(b []byte, sizes []int, quality int) ([][]byte, error)`
  + lỗi `ErrNotImage`, `ErrTooManyPixels`. Auth gọi qua nó; hành vi không đổi (test hiện có của auth giữ nguyên, xanh).

## Implementation Steps
1. Test trước: chuyển `avatar_size_test.go` các case thuần sang `backend/pkg/imagenorm`; chạy fail; tách code; auth
   gọi pkg; `make test s=auth` xanh; `make check-layering`.
2. Test trước (vitest): `AvatarEdit.test.tsx` — bấm mở menu, Enter/Space mở, Esc đóng trả focus, chọn tệp gọi
   `onUpload`, tệp sai báo lỗi, "Bỏ ảnh" chỉ khi có `src`, `canEdit=false` không có button, nhãn trình đọc màn hình.
3. Viết `AvatarEdit`; `ProfileTab` dùng nó, bỏ hai Button; cập nhật `SettingsScreen.test.tsx` cho luồng mới.
4. `make check-ngac` (không có chuỗi NGAC mới — chỉ để chắc).

## Related Code Files
- Create: `backend/pkg/imagenorm/*.go` (+ test), `frontend/src/components/patterns/AvatarEdit.tsx` (+ test), `frontend/src/lib/avatar-file.ts`
- Modify: `backend/services/auth/internal/domain/avatar.go`, `frontend/src/components/settings/ProfileTab.tsx`, `settings-model.ts`, tests liên quan

## Success Criteria
- [ ] Hồ sơ không còn nút "Đổi ảnh"/"Bỏ ảnh"; đổi/bỏ qua chạm ảnh; bàn phím + trình đọc màn hình dùng được.
- [ ] `make test s=auth` xanh, không đổi kết quả ảnh (so byte với ảnh mẫu trong test hiện có).

## Risk Assessment
- `store.AvatarImages` là kiểu của store auth → pkg trả `[][]byte`, auth tự gói lại. Nếu tách làm vỡ test auth theo cách
  không giải thích được → dừng, hỏi trước khi chép code.
