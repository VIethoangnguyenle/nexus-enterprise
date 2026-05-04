## Architecture

Frontend-only changes. No new services, no proto changes, no backend modifications.

## Issue 1: Chat Image Preview

### Current Flow
```
Message with linked_entity_type='drive_file'
    ↓
FilePreviewCard always
    ↓
[icon] filename.jpg    ← Even for images!
       234 KB · JPG
```

### After Fix
```
Message with linked_entity_type='drive_file'
    ↓
isImageFile(filename)?
    ├── YES → ImagePreviewCard
    │         ┌──────────────────────┐
    │         │  ┌────────────────┐  │
    │         │  │  actual image  │  │  ← Lazy-loaded via getDownloadUrl
    │         │  │  thumbnail     │  │  ← max-width: 320px, rounded
    │         │  └────────────────┘  │
    │         │  filename.jpg · 234K │
    │         └──────────────────────┘
    │
    └── NO  → FilePreviewCard (existing, unchanged)
```

### ImagePreviewCard Component Design

```tsx
// components/patterns/ImagePreviewCard.tsx
function ImagePreviewCard({ fileId, filename }: Props) {
  // 1. Determine if image from extension
  // 2. useMemo — fetch download URL on mount
  // 3. Render <img> with loading skeleton
  // 4. Click → open full-size in new tab
}
```

**Image detection:** Check file extension against known set:
```
IMAGE_EXTENSIONS = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico']
```

**URL fetching strategy:** Use `useState` + `useEffect` to lazy-fetch `driveApi.getDownloadUrl(fileId)`. Presigned URLs have TTL so we don't cache aggressively. Re-fetch on each mount is acceptable for chat.

**Rendering:**
- Max width 320px, max height 240px, `object-fit: contain`
- Rounded corners matching design system
- Loading skeleton while URL fetches
- Error fallback → regular `FilePreviewCard`
- Click → `window.open(url, '_blank')`

### Files Changed

#### [NEW] `components/patterns/ImagePreviewCard.tsx`
- `isImageFile(filename)` helper
- `ImagePreviewCard` component with lazy URL loading
- Skeleton loader, error fallback, click-to-open

#### [MODIFY] `routes/_workspace/channels.$channelId.tsx`
- Import `ImagePreviewCard`
- In `MessageBubble` (line ~303-334): check `isImageFile(attachedFilename)` → render `ImagePreviewCard` instead of `FilePreviewCard`

---

## Issue 2: Documents List Doesn't Refresh

### Root Cause

```
Frontend: documentApi.list(wsId)
    ↓ GET /api/workspaces/:id/documents
    ↓
Backend: handler.ListDocuments
    ↓ drive.ListFolder(req)
    ↓ returns DriveItemList proto
    ↓
Response JSON: { "items": [...] }    ← Backend returns "items"
Frontend reads: data.documents       ← Frontend expects "documents"
    ↓
data.documents === undefined → empty list always
```

### Fix Strategy

Update frontend to match the actual backend response:

#### [MODIFY] `api/documents.ts`
- `documentApi.list()` — change return type from `{ documents: Document[] }` to `{ items: DriveItem[] }`
- Map DriveItem fields to Document interface in a transform function

#### [MODIFY] `hooks/useDocuments.ts`
- Add `select` transform in query options to map `items` → `documents` shape
- Maintain backward compatibility for consumers

#### [MODIFY] `routes/_workspace/documents.tsx`
- After upload: use `queryClient.invalidateQueries({ queryKey: ['documents', wsId] })` instead of `refetch()` for more reliable cache invalidation

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Image detection | Extension-based | Simple, no extra API call. Mime type unavailable in message context |
| Image URL strategy | Lazy fetch per-component | Presigned URLs have TTL, can't store permanently |
| Image max size | 320×240px | Balance between visibility and chat readability |
| Docs response fix | Frontend transform | Backend is correct (returns Drive proto), frontend needs to adapt |
| Cache invalidation | `invalidateQueries` | More reliable than `refetch()` with TanStack Query |
