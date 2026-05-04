# Drive File & Folder Management + Sharing

## Evidence Summary
- **Backend**: ✅ Complete — Proto defines TrashItem, DeleteItem, CreateShare, RevokeShare, ListShares, GetSharedWithMe. gRPC server and REST handlers fully implemented.
- **Frontend API**: ✅ Complete — `drive.ts` has `trashItem()`, `deleteItem()`, `createShare()`, `revokeShare()`, `listShares()`, `sharedWithMe()`.
- **Frontend Hooks**: ✅ Complete — `useDrive.ts` has `useTrashItem()`, `useDeleteItem()`, `useCreateShare()`, `useRevokeShare()`, `useDriveShares()`, `useSharedWithMe()`.
- **Frontend UI**: ⚠️ Partial — Delete/trash confirmation dialog exists (`DeleteConfirmDialog`). Share UI exists in `DriveContextPanel` permissions tab. BUT:
  - Delete action only does **soft trash** via `DELETE /drive/items/:itemId`. No **permanent delete** option in UI.
  - Share modal uses raw `target_node_id` input — no user-friendly workspace member picker.
  - "Shared with me" section exists in `DriveSidebar` but may need verification.
- **Proto**: ✅ All needed RPCs defined (TrashItem, RestoreItem, DeleteItem, CreateShare, RevokeShare, ListShares, GetSharedWithMe)
- **DB**: ✅ Drive items + shares tables exist

## Product Assessment
- **Size**: M (backend complete, frontend needs 3-5 component modifications)
- **Risk**: Low (all backend APIs exist, similar patterns in chat member management)
- **Target user**: Workspace members managing files
- **Core actions**: (1) Delete files/folders permanently, (2) Share files with workspace contacts via picker

## Scope

### In scope
1. **Delete File/Folder**: Add permanent delete option to UI with 2-step flow (trash → permanent delete)
2. **Share File/Folder**: Replace raw node-ID input with workspace member picker modal (like chat AddMemberModal)
3. **Shared with me**: Verify "Shared with me" view works end-to-end

### Out of scope
- Bulk select + batch delete (future)
- Public link sharing (proto supports it but not this iteration)
- Trash view with restore (requires separate tab/view — deferred)

### Deferred
- Trash view with list of trashed items + restore action
- Share link generation for external users
- Permission levels UI (read vs write vs admin)

## Success Criteria
- User can delete a file or folder from Drive UI with confirmation dialog
- User can share a file/folder with workspace members via a searchable picker
- User can see who has access to a file (shares list)
- Shared items appear in "Shared with me" view
- All actions work at 375px, 768px, 1280px breakpoints
