## Why

Two user-facing bugs that break the file experience:

1. **Chat images render as file cards** — When an image (JPG/PNG/GIF/WebP) is uploaded in chat, it shows a generic `FilePreviewCard` (icon + filename) instead of rendering the actual image inline. Users expect to see image previews directly in the conversation, like every modern messaging app.

2. **Documents list doesn't refresh after upload** — After uploading a document in the Docs module, the list stays empty. Root cause: frontend `documentApi.list()` expects `{ documents: Document[] }` but backend `ListDocuments` proxies to Drive service which returns `{ items: DriveItem[] }`. The response field mismatch means `data.documents` is always `undefined`.

## What Changes

### Chat — Inline Image Preview
- **NEW**: `ImagePreviewCard` component — detects image files by extension/mime, fetches presigned download URL, renders `<img>` thumbnail inline
- **MODIFY**: Message rendering in `channels.$channelId.tsx` — use `ImagePreviewCard` for image files, keep `FilePreviewCard` for other file types

### Docs — Fix List Refresh
- **FIX**: `api/documents.ts` — align response type with actual backend response (`{ items }` not `{ documents }`)
- **FIX**: `hooks/useDocuments.ts` — map DriveItem fields to Document interface for backward compatibility
- **FIX**: `routes/_workspace/documents.tsx` — use `queryClient.invalidateQueries` after upload for reliable cache bust

## Impact

- **Frontend only** — no backend changes needed
- `components/patterns/ImagePreviewCard.tsx` [NEW]
- `routes/_workspace/channels.$channelId.tsx` [MODIFY]
- `api/documents.ts` [MODIFY]
- `hooks/useDocuments.ts` [MODIFY]  
- `routes/_workspace/documents.tsx` [MODIFY]
