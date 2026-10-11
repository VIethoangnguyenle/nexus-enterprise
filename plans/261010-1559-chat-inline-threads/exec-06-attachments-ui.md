---
title: "Execution plan — chat phase 06 attachments UI"
owner: Claude (planning, review) · executor: agy/Gemini
status: ready (start after chat 07/08 UI is committed)
---

# Execution plan: phase 06 attachments — frontend only

Backend is DONE (commit e3582b3): `POST /api/channels/{id}/messages` takes
`attachments: [{drive_file_id, width, height}]` (≤ 20), every message/thread/WS frame carries
`attachments: [{drive_file_id, file_name, mime_type, size_bytes, width, height}]`. Contract:
`docs/specs/chat-attachments/spec.md`. Design (approved, render it — do not redesign):
`design/mockups/spaces.html` sections `#composer` (attachment tray `#tray-b`) and `#viewer`
(image viewer), `design/mockups/chat-mobile.html` composer "+" sheet (Máy ảnh / Ảnh / Tệp / Tài liệu),
`DESIGN.md` chat composer + attachments paragraphs.

## Decisions (fixed — do not change)
1. Upload goes to the channel drive exactly like today's `handleFileUpload` in
   `frontend/src/components/spaces/SpaceView.tsx` (channelDrive → folder → createFile → uploadToStorage
   → confirmFile). Move that into a hook `frontend/src/hooks/useAttachmentUploads.ts` that manages a
   queue: `{ key, file, status: 'uploading'|'done'|'error', progress?, fileId?, width?, height?, error? }`,
   `add(files)`, `remove(key)`, `retry(key)`, `clear()`, `ready` (all done), `items`.
2. Images: measure `width/height` client-side with `createImageBitmap(file)` (fallback: `Image()` +
   object URL); send them with the attachment.
3. One message carries text + all files: `send.mutate({ content, attachments })`. Extend
   `SendMessageInput` in `frontend/src/api/messaging.ts` with `attachments?: {drive_file_id, width, height}[]`
   and `useSendMessage` in `hooks/useMessages.ts` to pass it (optimistic message shows the tray files as
   attachments with a local object URL thumbnail).
4. Limits: max 20 files per message (refuse the 21st with toast "Tối đa 20 tệp mỗi tin"); upload errors
   name the file: toast "Chưa tải lên được “<tên>”…" and the chip shows error + Thử lại.
5. "Gửi" disabled while any item is uploading or errored; a message may be files only (empty text OK).
6. Inputs: attach button menu "Tải lên từ máy" (file input, multiple) and "Chọn từ Tài liệu" (only if a
   drive picker component already exists — search `components/drive` for a picker; if none, omit the item
   and say so in the report); drag-and-drop onto the conversation pane and onto the thread panel; paste
   images (Ctrl/Cmd+V) into the composer.
7. Rendering in a message (`MessageBlock.tsx`): images → 1 large / 2–4 grid (more than 4: 4th tile shows
   "+N"); other files → card (icon by type via `lib/file-kind.ts`, name, "LOẠI · dung lượng"). Old
   `linked_entity_type === 'drive_file'` messages already arrive as one attachment from the server — remove
   the old single-file branch once attachments render (keep behaviour identical for those messages).
8. Image viewer (`ImageViewer.tsx`): modal dialog, ←/→ between the message's images, "Tải xuống"
   (`driveApi.getDownloadUrl`), Esc closes, focus returns to the clicked thumbnail; phone: swipe-free,
   buttons only. Thumbnails/images load via `driveApi.getDownloadUrl(fileId)` (cache per fileId with
   TanStack Query key `['drive','download',fileId]`, staleTime 5 min).
9. Thread panel composer gets the same tray (ChatEditor is shared).
10. No UUID/ID on screen. 44px targets on phones. All copy Vietnamese as in the mockup.

## Files
- Create: `frontend/src/hooks/useAttachmentUploads.ts` (+ `.test.tsx`)
- Create: `frontend/src/components/chat/AttachmentTray.tsx` (+ test), `AttachmentGrid.tsx` (+ test),
  `ImageViewer.tsx` (+ test)
- Modify: `components/chat/ChatEditor.tsx` (tray above the editor, paste, attach menu, Gửi rules),
  `components/spaces/SpaceView.tsx` (drop zone, use the hook, remove old `handleFileUpload`),
  `components/chat/ThreadPanel.tsx` (drop zone), `components/spaces/MessageBlock.tsx` (grid/cards),
  `api/messaging.ts`, `hooks/useMessages.ts`, `test/chat-fixtures.ts` (fixtures with attachments)
- Spec: add a "Status: matches code" note / any client detail to `docs/specs/chat-attachments/spec.md`.

## Tests to write FIRST (vitest, they must fail before the code)
1. Hook: add 2 files → both upload through mocked driveApi, `ready` true after; one fails → status error,
   retry succeeds; remove drops it; 21st file refused.
2. ChatEditor: Gửi disabled while uploading; enabled with files only; sends `{content, attachments}` with
   width/height for an image; paste of an image file adds a chip.
3. AttachmentGrid: 1/2/4/6 images → layout classes + "+2" tile; a PDF renders a card with name and
   "PDF · 1,2 MB"; no UUID in text.
4. ImageViewer: opens on click, → / ← move, Esc closes and focus returns.
5. MessageBlock: a message with attachments and no text renders only the grid; legacy drive_file message
   renders as one card.

## Gates (all must pass; report exact output)
`cd frontend && npx vitest run` (only the known AssetsScreen overview failure allowed) ·
`npm run lint` · `npm run typecheck:diff` · `npm run build`.

## Rules for the executor
- Do NOT commit; do NOT run `git add`/`git reset`/anything touching the git index.
- Touch only the files listed (+ their tests). Ask (report NEEDS_CONTEXT) instead of inventing UI not in
  the mockup.
- Report: `plans/reports/agy-<date>-chat-06-attachments-ui.md` — files changed, tests added, gate output,
  anything skipped and why.
