---
phase: 11
title: "Tài liệu on the phone"
status: pending
priority: P2
effort: "2.5d"
dependencies: [8, 9]
---

# Phase 11: Tài liệu on the phone

## Overview
Phone-first Tài liệu (mockup §10): entry chips instead of the tree drawer, Truy cập nhanh, list by default, folder
screen with tappable path, richer file sheet, multi-select, background upload, star. Starts only after the
prerequisite drive-copy authorization fix is deployed.

## Requirements
- Functional: chips Gần đây · Thư mục · Gắn sao · Chia sẻ với tôi · Bản nháp (one "shared" entry; Bản nháp comes
  from the document service's drafts query); Truy cập nhanh up to 6 cards with a reason line limited to reasons the
  caller is still allowed to see ("Bạn sửa", "Thu Lan chia sẻ"; a mention reason only after re-checking read on the
  source conversation); list default on phones (grid only for real thumbnails); folder screen: ← up to parent, path
  crumbs, sort + view one row, search scoped to the folder (phase 08); file sheet: Chia sẻ, Sao chép liên kết, Gắn
  sao, Tải xuống + Gửi vào trò chuyện, Chuyển tới thư mục, Tạo bản sao (existing copy route), Thông tin và hoạt
  động, Đổi tên, Chuyển vào thùng rác (only permitted actions); multi-select via long-press with a dark action bar
  (only actions every selected item allows, via the existing batch access check); + sheet: Tải tệp lên, Chụp ảnh,
  Thư mục mới, Văn bản mới; background uploads with a progress card, per-file retry, queue kept in IndexedDB so a
  reload or app update resumes or reports it. Personal root "Tài liệu của tôi"; org root by workspace then
  departments by permission.
- Backend: star per user per item on flat routes `POST/DELETE /api/drive/items/:itemId/star`, `GET
  /api/drive/starred?workspace_id=…`; quick access `GET /api/drive/quick-access?workspace_id=…`; both filter to the
  item's own workspace, check membership of that workspace and re-check read on every item when the list is read;
  a cleanup of upload rows left `pending` for more than 24h.

## Architecture
Phone composition (`DrivePhoneHome`, `DriveFolderPhone`) sharing `drive-model.ts` and `useDriveActions`;
`DriveTreeDrawer` kept for tablet. Upload queue store (Zustand) persisted to IndexedDB. Reserved migration
`070_drive_stars.sql` (idempotent). Routes sit under the existing `/api/drive` prefix (already in vite and nginx).

## Related Code Files
- Modify: `frontend/src/components/drive/{DriveScreen,DriveToolbar,DriveItemMenu,DriveBreadcrumbs,DriveTreeDrawer}.tsx`, `use-drive-layout.ts`, `useDriveActions.ts`, drive `rest/handler.go`, `domain`, `store`
- Create: `data/migrations/070_drive_stars.sql`, `DrivePhoneHome.tsx`, `DriveSelectionBar.tsx`, `UploadQueue.tsx`, `upload-queue.store.ts`, star/quick-access handlers, pending-upload cleanup, tests
- Specs: `drive-tree-navigation`, `drive-permission-engine`, `file-preview`

## Implementation Steps
1. Backend tests first, deny first: star an unreadable item → DENY; starred list never shows an item from another workspace or one no longer readable; quick access same; mention reason hidden after removal from the conversation; non-member of `workspace_id` → 403.
2. Back up DB; `070` migration; `make db-migrate` twice; verify.
3. Endpoints; pending-upload cleanup job.
4. Frontend phone composition, sheet, selection, persisted upload queue; tests at 390px.

## Success Criteria
- [ ] Mockup §10 screens at 390px for both workspace kinds
- [ ] Deny tests for star, starred list, quick access, reasons; strict drive test run green
- [ ] Upload survives navigation and reload (resumes or reports); no orphan `pending` rows after 24h

## Risk Assessment
Quick access ranking feels random → start with recency + reason; tune later.

## Rollback
Revert the release; `drive_stars` is additive and ignored by the old version.
