---
phase: 6
title: "Cache trên máy (IndexedDB)"
status: pending
priority: P1
effort: "1.5d"
dependencies: [4, 5]
---

# Phase 06: Mở app từ dữ liệu lần trước

## Requirements (người dùng chốt 2026-10-16: lưu cả nội dung tin)
- TanStack Query persister trên IndexedDB (thư viện `@tanstack/query-async-storage-persister` + `idb-keyval` hoặc
  tương đương đã có; thêm dependency phải ghi lý do), khoá theo **user + workspace**, `buster` = phiên bản build,
  `maxAge` 7 ngày.
- Lưu: `chat-state`, danh bạ, `/me`, trang tin mới nhất của 20 hội thoại gần nhất, con trỏ đồng bộ (phase 08).
  Không lưu: token, OTP, nội dung tệp, URL presigned.
- Xoá toàn bộ khi: đăng xuất, đổi workspace, refresh token thất bại / 401, `buster` đổi, quá `maxAge`.
- Mở app: render shell + Trang chủ từ cache ngay; refetch nền; dải "Đang cập nhật" theo DESIGN.md (viết mục DESIGN.md
  trước, mockup nhỏ nếu cần — design-first).
- Người khác đăng nhập cùng máy: không thấy dữ liệu người trước (khoá theo user + xoá khi đăng xuất).

## Implementation Steps
1. Test trước: persist → reload (fake IndexedDB) → dữ liệu hiện trước khi fetch xong; đăng xuất / 401 / đổi workspace
   → kho rỗng; user B không đọc được kho của user A; quá 7 ngày → bỏ.
2. Persister + bộ lọc query được lưu (`shouldDehydrateQuery`) + giới hạn 20 hội thoại.
3. Dải "Đang cập nhật" + test.
4. Spec `pwa-install`, `sign-in-screens`.

## Related Code Files
- Create: `frontend/src/lib/query-persist.ts` (+ test)
- Modify: `frontend/src/lib/query-client.ts`, `frontend/src/main.tsx`, `frontend/src/api/client.ts` (xoá khi 401),
  `stores/auth.store.ts` (đăng xuất), `hooks/useSwitchWorkspace.ts`, DESIGN.md

## Success Criteria
- [ ] iPhone PWA: Trang chủ hiện < 1 s khi có cache (Web Inspector).

## Risk Assessment
- Dữ liệu doanh nghiệp trên máy dùng chung → điều kiện xoá trên + maxAge; ghi rõ trong spec.
