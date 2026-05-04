# S1: Delete File/Folder

## User Story
As a workspace member, I want to delete files and folders from Drive so that I can manage my workspace content.

## Acceptance Criteria

### AC-1: Trash (Soft Delete)
- GIVEN a file or folder in Drive
- WHEN user clicks "Delete" in the context menu
- THEN a confirmation dialog appears with the item name
- AND on confirm, the item is soft-deleted (status = "trashed")
- AND the item disappears from the folder view optimistically
- AND if the API fails, the item reappears (rollback)

### AC-2: Permanent Delete
- GIVEN a file or folder in Drive
- WHEN user clicks "Delete permanently" in the context menu
- THEN a destructive confirmation dialog appears with a warning "This cannot be undone"
- AND on confirm, the item is permanently deleted via `DELETE /drive/items/:itemId/permanent`
- AND the item disappears from the folder view optimistically

### AC-3: Folder Delete
- GIVEN a folder with children (files/subfolders)
- WHEN user deletes the folder
- THEN the confirmation dialog warns "This folder contains X items"
- AND all children are deleted with the folder (server-side cascade)

### AC-4: Error Handling
- GIVEN a delete operation fails
- THEN the item reappears in the folder view
- AND an error toast or message is shown

## UI Flow
1. User right-clicks or clicks "..." on a Drive item
2. Context menu shows "Delete" option (with trash icon, text-error on hover)
3. `DeleteConfirmDialog` opens with item name
4. User clicks "Delete" → optimistic removal → API call
5. On success: item gone. On error: item restored.

## Existing Components Used
- `DeleteConfirmDialog` — already exists in `components/drive/`
- `useTrashItem` / `useDeleteItem` — already exist with optimistic updates
- DriveFileList context menu — needs "Delete permanently" option
