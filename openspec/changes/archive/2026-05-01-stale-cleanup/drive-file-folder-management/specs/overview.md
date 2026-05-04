# Drive File & Folder Management — Specs Overview

## Stories

### S1: Delete File/Folder
User can delete a file or folder from Drive with a confirmation dialog.
- Soft-delete (trash) is the default action via the existing context menu "Delete" option
- Permanent delete option available for already-trashed items or as explicit "Delete permanently" action

### S2: Share File/Folder with Workspace Members
User can share a file or folder with other workspace members via a searchable member picker.
- Share modal opens from context menu "Share" action
- Shows workspace contacts with search/filter
- Allows selecting permission level (read/write)
- Shows current shares for the item
- Allows revoking existing shares

## Backend Contract (Verified)
All APIs exist and are fully implemented:
- `DELETE /api/drive/items/:itemId` → TrashItem (soft delete)
- `DELETE /api/drive/items/:itemId/permanent` → DeleteItem (permanent)
- `POST /api/drive/items/:itemId/share` → CreateShare
- `DELETE /api/drive/shares/:shareId` → RevokeShare
- `GET /api/drive/items/:itemId/shares` → ListShares
- `GET /api/drive/shared-with-me` → GetSharedWithMe

## Frontend Contract (To Implement)
- `driveApi.*` — All functions exist
- `useDrive.*` — All hooks exist
- UI components: Need modifications to DriveFileList context menu, ShareDialog, and DriveContextPanel
