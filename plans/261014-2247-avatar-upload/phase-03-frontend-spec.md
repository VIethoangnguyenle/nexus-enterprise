---
phase: 3
title: "Frontend + spec"
status: completed
priority: P1
effort: "4h"
dependencies: [2]
---

# Phase 3: Frontend + spec

## Overview
Nút "Đổi ảnh" · "Bỏ ảnh" trong `ProfileTab` đúng mockup; `Avatar` chọn bản 64/192 theo kích thước và rơi về chữ cái đầu khi
URL chết; mọi `<Avatar>` có người cụ thể đều truyền ảnh. Cập nhật ba spec.

## Requirements
**ProfileTab (mockup `design/mockups/contacts-documents-settings.html:830`):**
- Avatar 64 + "Đổi ảnh" (icon `camera`, mở `<input type="file" accept="image/jpeg,image/png">` ẩn) + "Bỏ ảnh"
  (luôn hiện như mockup; **disabled** khi chưa có ảnh) + chú thích "JPG hoặc PNG, tối đa 5 MB. Không có ảnh thì dùng chữ cái
  đầu trên màu riêng của bạn."
- Kiểm phía client trước khi gửi (`validateAvatarFile` trong `settings-model.ts`): không phải jpeg/png → "Chọn ảnh JPG hoặc PNG.";
  > 5 MiB → "Ảnh lớn hơn 5 MB. Chọn ảnh nhỏ hơn.".
- Lỗi server map theo `code`: `avatar_not_image`/`avatar_too_large` → câu tương ứng ở trên; `avatar_too_many_pixels` → "Ảnh quá lớn
  về kích thước. Chọn ảnh nhỏ hơn."; `avatar_busy` → "Máy chủ đang bận xử lý ảnh. Thử lại sau vài giây."; `rate_limited` →
  "Bạn đổi ảnh nhiều quá. Thử lại sau." 
- Đang tải: nút disabled + "Đang tải ảnh…"; xong: toast "Đã đổi ảnh" / "Đã bỏ ảnh".
- Không hiện URL, token hay tên file nội bộ. Upload độc lập với form tên/chức danh.

**Avatar primitive (`components/primitives/Avatar.tsx`):**
- `src` là URL gốc từ server; Avatar nối `/64` khi kích thước hiển thị ≤ 32, `/192` khi > 32 (helper `avatarVariant(src, px)`
  — chỉ nối với URL dạng `/api/auth/avatars/<token>`, URL khác giữ nguyên).
- `onError` → chuyển sang chữ cái đầu (state cục bộ, reset khi `src` đổi). Áp cho mọi nơi dùng Avatar.

**Truyền ảnh ở mọi `<Avatar>` có người cụ thể:** audit `grep -rn "<Avatar" frontend/src` — mọi call site có id người
(`hueKey`/user id) mà không có `src` thì truyền `people.byUserId.get(id)?.avatarUrl` (hoặc trường avatar sẵn có trong payload).
Đã biết (theo red-team): `spaces/FeedParts.tsx:126`, `calls/PreJoin.tsx:144`, `calls/MiniCall.tsx:63`, `chat/MentionDropdown.tsx:91`,
`spaces/SpaceInfoPanel.tsx:137`, `spaces/SpaceSearchPanel.tsx:54`, `spaces/SpaceView.tsx:327`, `auth/ProfileScreen.tsx:101`,
`approval/ApprovalScreen.tsx:375`, `drive/DriveBurstBanner.tsx:22`, cộng các call site nhiều dòng (`ThreadSummary`, `ThreadsPanel`,
`PhoneThreadsScreen`, `UnreadThreadsSection`, `CallCard`, `ContactPanel`) — kiểm từng cái. Chỗ không có user id
(vd `AuthShell` danh sách workspace, lời mời chưa đăng nhập) giữ chữ cái đầu, ghi lý do vào báo cáo phase.

## Architecture
- `frontend/src/api/client.ts`: `apiUpload(path, body, method = 'POST')` — giữ `withRefreshRetry` sẵn có (đã refresh token).
- `frontend/src/api/profile.ts`: `uploadAvatar(file)` → `apiUpload('/me/avatar', fd, 'PUT')` (field `file`);
  `removeAvatar()` → `apiFetch('/me/avatar', {method:'DELETE'})`.
- `frontend/src/hooks/useProfile.ts`: `useUploadAvatar`, `useRemoveAvatar` cạnh `useUpdateProfile`, dùng lại đúng ba invalidation
  của nó (`keys.contacts.every()`, `keys.auth.profiles()`, `keys.auth.me()`). Sidebar/AccountSheet lấy ảnh qua `usePeople` →
  contacts, nên `contacts.every()` là cái làm sidebar đổi ngay.
- `frontend/src/api/auth.ts:69`: sửa comment (avatar có endpoint riêng, không qua PATCH).

## Related Code Files
- Modify: `frontend/src/components/settings/ProfileTab.tsx`, `settings-model.ts`, `settings-model.test.ts`, `SettingsScreen.test.tsx`,
  `frontend/src/components/primitives/Avatar.tsx` (+ test), `frontend/src/api/client.ts`, `frontend/src/api/profile.ts`,
  `frontend/src/hooks/useProfile.ts`, `frontend/src/api/auth.ts` (comment), các call site `<Avatar>` ở trên
- Modify spec: `docs/specs/tenant-auth-flow/spec.md` (endpoints, lỗi, cache, token ngẫu nhiên, 3 kích thước, deny scenarios; thay
  câu "no image store" ở :229,235-237), `docs/specs/contacts-documents-settings-screens/spec.md` (UI đổi/bỏ ảnh, lỗi, fallback),
  `docs/specs/sign-in-screens/spec.md:38` (bỏ "server keeps no image store"; bước hồ sơ vẫn không có upload)

## Implementation Steps
1. TDD `settings-model.test.ts`: `validateAvatarFile` — gif → lỗi; 5 MiB+1 png → lỗi; 1 MB jpeg → null.
2. TDD Avatar test: `src` + size 24 → `img.src` kết thúc `/64`; size 64 → `/192`; URL ngoài dạng → giữ nguyên;
   bắn `error` trên img → hiện chữ cái đầu; đổi `src` → thử ảnh lại.
3. TDD `SettingsScreen.test.tsx`: không có ảnh → "Đổi ảnh" bật, "Bỏ ảnh" disabled; chọn png hợp lệ → gọi PUT, toast "Đã đổi ảnh";
   chọn gif → lỗi, **không** gọi API; server trả `avatar_not_image` / `avatar_busy` / 429 → đúng câu tiếng Việt;
   có ảnh → "Bỏ ảnh" gọi DELETE, toast "Đã bỏ ảnh".
4. Implement `apiUpload` method, api, hooks, Avatar, ProfileTab (Button + icon sẵn có, token DESIGN.md, không style mới).
5. Audit và sửa các call site `<Avatar>`; chạy lại test của các component bị đụng.
6. Cập nhật 3 spec; ghi rõ: "Bỏ ảnh" không xoá bản đã nằm trong cache trình duyệt người khác.
7. `cd frontend && npm test && npm run lint && npm run build`.
8. Chạy app (dùng stack đang chạy nếu có, không mở trùng; nếu không `make dev`): đổi ảnh → sidebar đổi không reload; tải lại vẫn còn;
   Danh bạ + chat + feed hiện ảnh; DevTools Network: avatar 24px tải `/64` (~vài KB), avatar 64 tải `/192`, không request `/1024`;
   bỏ ảnh → chữ cái đầu; ảnh dọc điện thoại đúng chiều. Request qua Vite proxy trả 200 (không 404). Chụp màn hình.

## Success Criteria
- [x] Test frontend mới xanh, gồm nhánh từ chối và onError.
- [x] Lint + build xanh.
- [x] Thử tay đủ bước 8, có ảnh chụp và số liệu Network.
- [x] Không còn `<Avatar>` có user id mà thiếu `src` (trừ ngoại lệ đã ghi lý do).
- [x] Ba spec cập nhật cùng commit.

## Risk Assessment
- Invalidate thiếu query → ảnh không đổi ở đâu đó; tín hiệu: bước 8. Xử lý: tìm query nguồn của chỗ đó và thêm invalidation.
- `apiUpload`/`toResult` logout khi 401 sau retry (`client.ts:144-149`) → session hết hạn giữa lúc upload sẽ đăng xuất; đây là
  hành vi chung của app, giữ nguyên, có một test xác nhận.
- Audit call site phình to: chỉ truyền `src`, không đổi layout; call site nào cần đổi dữ liệu backend để có avatar → ghi lại,
  không mở rộng phạm vi trong plan này.

## Kết quả (2026-10-15)
- 2.800 test frontend xanh; lint, build sạch; typecheck chỉ còn lỗi có sẵn trong `src/generated/`.
- Trình duyệt thật (gstack headless, `.gstack/browse-reports/2026-10-15-0001/`): đổi ảnh → Hồ sơ + sidebar đổi ngay, avatar 32px tải `/64` (680 B), 64px tải `/192`, không request `/1024`; Bỏ ảnh → chữ cái đầu, nút tắt; file giả → "Chọn ảnh JPG hoặc PNG.".
- Call site: 14 chỗ được truyền ảnh (helper `avatarOf` trong `lib/people.ts`). Giữ chữ cái đầu có lý do: `AuthShell` (là workspace, không phải người), `InvitationRow` và dòng "chuyển bởi" ở `WorkspaceTab` (payload chỉ có tên, không có id — cần đổi backend, ngoài phạm vi).
- Lệch plan: ô chọn file là `<input>` ẩn kèm `eslint-disable` có lý do, theo đúng mẫu `DriveScreen`/`ChatEditor`.
