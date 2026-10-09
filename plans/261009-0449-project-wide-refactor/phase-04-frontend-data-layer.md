---
phase: 4
title: "Frontend data layer"
status: pending
priority: P1
effort: 2-3d
dependencies: [1]
---

# Phase 04 — Frontend data layer

> **Tách 2026-10-10:** 04a = phần dùng chung (key factory, `useActiveWorkspace`, `MutationCache.onError`
> + toast, quyền). 04b = phần theo domain, làm cùng PR với nhóm màn tương ứng của phase 06.

## Overview
TanStack Query sở hữu server state (CLAUDE.md §3), nhưng thực tế có cache tự viết, fetch
imperative, query key không thống nhất và invalidation trỏ sai key. Sửa lớp dữ liệu trước khi
đụng UI (phase 06).

## Key insights (đã kiểm chứng các bug)
- **Bug invalidation:** `stores/websocket.store.ts:485` invalidate `['polls']` nhưng key thật là
  `['poll', id]` (`hooks/useMessaging.ts:249`). `websocket.store.ts:384` invalidate
  `['drive', ws, 'folder', parentId]` còn root cache dưới `'root'` (`hooks/useDrive.ts:9`) → sự
  kiện ở root không refresh.
- Key lẫn kiểu: `['unreadCounts']` vs `['unread-count']`, `['asset-summary']` vs
  `['asset-summary', wsId]`; nhiều key không scope theo workspace/tenant.
- **Workspace resolve 9 kiểu:** `admin/{index,roles,users}.tsx`, `channels.index.tsx:16`,
  `channels.$channelId.tsx:22`, `contacts.tsx:27`, `DriveContextPanel.tsx:245`,
  `CreateChannelModal.tsx:16`, `AppSidebar.tsx:54` đọc `window.location`/`workspaces[0]` thay vì
  `useActiveWorkspace`; `?ws=` chưa khai báo `validateSearch`.
- **Double cache quyền:** `stores/permission.store.ts` (TTL tự viết, mutate Map không qua `set`) +
  `hooks/usePermissions.ts`. `api/access.ts:16` mặc định có op `'delete'` — không phải 1 trong 8
  op NGAC, luôn DENY.
- Fetch imperative: `ImagePreviewCard.tsx:27`, `DriveContextPanel.tsx:118`,
  `FolderTreeSelect.tsx:135`, `ThreadPanel.tsx:30`, `channels.$channelId.tsx:69-77`,
  `admin/roles.tsx:84`, `documents.tsx:38`. Luồng download-URL lặp 5 lần.
- 48 `useMutation`, ~25 lỗi im lặng; không có toast, không `MutationCache.onError`; so khớp lỗi
  bằng chuỗi (`ThreadPanel.tsx:53`).
- 19 `apiFetch(` không có kiểu; ~19 `any`.
- Drive: `drive.store.currentFolderId` + `folderStack` local lệch nhau, folder không có trong URL.

## Requirements
- Query-key factory mỗi domain (`hooks/keys/*.ts`), mọi key scope theo workspace khi dữ liệu theo workspace.
- WebSocket invalidation dùng chính factory đó (bug ở trên biến mất theo cấu trúc).
- `useActiveWorkspace` là đường duy nhất; `?ws` khai báo qua `validateSearch`.
- Quyền: chỉ TanStack Query; op list lấy từ một nguồn khớp 8 op NGAC.
- `MutationCache.onError` + toast; phân nhánh theo `ApiError.status`.
- Drive folder path trong URL; xoá field store chết (`ui.store` activeModal/peekPanel*,
  `drive.store` activePath/selectedItemIds/viewMode trùng).

## Related files
- create `frontend/src/hooks/keys/`, `frontend/src/hooks/useDownloadUrl.ts`, `frontend/src/lib/toast.ts`
- modify `frontend/src/stores/{websocket,permission,drive,ui}.store.ts`, `frontend/src/hooks/*`,
  `frontend/src/api/{client,access}.ts`, `frontend/src/lib/query-client.ts`, các route/component ở Key insights
- modify `docs/specs/drive-tree-navigation/spec.md`

## Implementation steps
1. Test vitest đỏ cho reducer `websocket.store` (poll, drive root, move khỏi folder cũ).
2. Key factory + chuyển hook từng domain; test xanh.
3. `useActiveWorkspace` + `validateSearch`; xoá 9 resolver.
4. Permissions → Query-only; xoá `permission.store` nếu không còn consumer.
5. Mutation error + toast; fetch imperative → hook.
6. Drive URL state.

## Success criteria
- [ ] Không còn query key literal ngoài factory (lint rule hoặc grep CI).
- [ ] Không còn `window.location` để lấy workspace.
- [ ] Mỗi mutation lỗi đều hiện thông báo cho người dùng.
