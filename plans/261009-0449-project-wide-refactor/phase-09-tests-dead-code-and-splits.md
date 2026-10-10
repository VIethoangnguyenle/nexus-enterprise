---
phase: 9
title: "Tests, dead code, large files"
status: done
priority: P3
effort: 1-2d
dependencies: [4, 8]
---

# Phase 09 — Tests, dead code, large files

## Overview
Dọn phần còn lại sau khi cấu trúc đã ổn định: lấp test ở vùng trống, xoá code chết, tách file lớn.

## Key insights
- **Go không có test:** messaging domain (1,227 dòng) + store (865), workspace domain (899) + rest,
  drive store/domain/rest, asset grpc (1,061) + domain, toàn bộ document, policy EPP
  (`epp_*`), `write_server` invalidation.
- **Frontend:** 7 file test / 761 dòng; 0 test cho 13 hook, 5 store, 27 route.
- **Code chết Go:** drive `internal/domain/` (nếu phase 08 chưa xoá), document `PutObjectDirect` +
  `db` không dùng, workspace `ListPermissions`/`DeletePermission` placeholder + `DepartmentService`,
  asset REST stub `ListAssetRequests`/`GetAssetRequest` (`rest/handler.go:318-325`), messaging
  `handleLegacyJSON` (`hub.go:493`), `export_test_helpers.go` lọt vào build production, `nilStr` trùng.
- **Code chết frontend:** `composites/FilterBar.tsx`, `patterns/ChannelDrivePanel.tsx`,
  `drive/DrivePreviewDialog.tsx`, `layouts/AuthLayout.tsx`, `InviteMemberForm.tsx`; ~18 hook không
  dùng; dep `@tiptap/extension-mention`, `@tanstack/react-router-devtools`; `frontend/README.md`
  vẫn là template Vite.
- **File lớn:** `approval.tsx` 473, `ChannelInfoPanel.tsx` 473, `DriveContextPanel.tsx` 440,
  `useMessaging.ts` 443, `auth/internal/domain/service.go` 699 (legacy + multi-tenant trộn),
  `messaging/internal/domain/service.go` 672.
- Plan cũ `docs/superpowers/plans/2026-08-01-*` đã thực thi xong nhưng checkbox chưa tick.

## Requirements
- Test cho mọi domain Go chưa có test, ưu tiên đường ghi và nhánh deny.
- Test cho hook/store frontend có logic (websocket reducer đã làm ở phase 04).
- Xoá code chết; xoá legacy auth register/login nếu không còn caller.
- Tách file > ~400 dòng theo ranh giới trách nhiệm, không đổi hành vi.

## Implementation steps
1. Test trước cho vùng sắp xoá/tách (khoá hành vi).
2. Xoá code chết, `make build-check` + `npm run build` xanh.
3. Tách file lớn.
4. Tick checkbox plan cũ, cập nhật `frontend/README.md`.

## Success criteria
- [ ] Mỗi package domain Go có test.
- [ ] Không còn component/hook/dep không được tham chiếu.

## Spec
Không đổi hành vi — không spec nào.
