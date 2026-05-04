## Tasks

### Chat Image Preview
- [x] Create `ImagePreviewCard` component (`components/patterns/ImagePreviewCard.tsx`)
  - [x] `isImageFile(filename)` helper — check extension against known image set
  - [x] Lazy fetch `driveApi.getDownloadUrl(fileId)` on mount
  - [x] Render `<img>` with loading skeleton, max 320×240px
  - [x] Click handler → open full-size in new tab
  - [x] Error fallback → render `FilePreviewCard` instead
- [x] Update message rendering in `channels.$channelId.tsx`
  - [x] Import `ImagePreviewCard` and `isImageFile`
  - [x] In `MessageBubble`: branch on `isImageFile(attachedFilename)` → `ImagePreviewCard` vs `FilePreviewCard`

### Docs List Fix
- [x] Fix `api/documents.ts` — `documentApi.list()` to read `{ items }` from response
- [x] Fix `hooks/useDocuments.ts` — transform handled in `documentApi.list()` directly
- [x] Fix `routes/_workspace/documents.tsx` — use `queryClient.invalidateQueries` after upload

### Backend Fix (discovered during testing)
- [x] Fix `ensureRoot` in drive gRPC server — assign new DriveRoot OA under workspace PC
- [x] Data fix: assign orphaned DriveRoot OAs under hoang_Documents OA + flush cache + restart policy

### Verification
- [x] `npm run build` passes
- [x] `go build` drive service passes
- [x] Documents list shows 20+ files with correct titles, filenames, dates
- [ ] Manual test: upload image in chat → image preview renders inline
- [ ] Manual test: upload non-image file in chat → file card renders (no regression)
- [ ] Manual test: upload document in Docs → list refreshes with new item
