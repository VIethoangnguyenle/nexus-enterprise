# Report: Chat Phase 06 Attachments UI Review Fixes

**Date**: 2026-10-11  
**Plan**: `plans/261010-1559-chat-inline-threads/exec-06-review-fixes.md`  
**Review Source**: `plans/reports/code-reviewer-261011-1115-chat-06.md`  
**Progress Log**: `plans/reports/agy-progress-chat-06-fixes.log`  

---

## 1. Summary of Changes

All High, Medium, and Low findings from the review report have been systematically resolved across the chat attachments codebase.

### High Findings Fixed
- **H1 Modal Esc Isolation**: `ImageViewer` is built on `useModalFocus`, portaled to `document.body` via `createPortal`. Esc stops propagation on `document` and prevents default so SpaceView's `window` listener never receives it. Verified with test: Esc with viewer open inside an open thread panel closes only the viewer.
- **H2 Focus Trap & Return**: Tab navigation is trapped within `ImageViewer` (using `a[href], button, ...`). On unmount, focus returns to the specific thumbnail button that opened it (`MessageBlock` tracks and passes opener ref).
- **H3 Reliable Downloads**: Replaced `window.open` after async await with `useDownloadFile` (`<a download>`) and `useDownloadUrl`. On download failure, Vietnamese error toast ("Không tải được tệp. Thử lại.") is shown.

### Medium Findings Fixed
- **M4 Measured Aspect Ratio**: Single image aspect ratio is computed from measured dimensions (`single.width / single.height`), clamped to `[0.5, 2.0]` per mockup, with `4 / 3` fallback only when dimensions are absent.
- **M5 Send Failure Retention**: `ChatEditor.handleSend` is asynchronous and awaits `onSend`. If the API request rejects, attachments in the tray and composer text are preserved.
- **M6 Thread Isolation**: `SpaceView` keys `ThreadPanel` by `thread-${panel.id}` and `ThreadPanel` clears upload state on `messageId` change, preventing attachments from leaking across threads.
- **M7 Upload Cancellation**: `useAttachmentUploads` maintains an `AbortController` per upload item. Removing a chip aborts in-flight network requests so no orphaned drive file is confirmed.
- **M8 Vietnamese Error Reasons**: Raw server errors are mapped to user-friendly Vietnamese reasons (`uploadErrorReason`: 413 file too large, 403 forbidden, network, generic).
- **M9 Mockup Alignment**:
  - `AttachmentTray`: Displays real upload progress percentage (e.g. "Đang tải 64%") and Vietnamese error explanations.
  - `ChatEditor`: Displays `whyStatus` next to disabled Gửi button ("Đang tải X tệp…" or "Thử lại hoặc bỏ tệp lỗi để gửi"); desktop attach dropdown menu ("Tải lên từ máy") and mobile "+" sheet ("Máy ảnh", "Ảnh", "Tệp").
  - Drop Overlays: Updated to exact mockup copy ("Thả tệp để đính kèm" / "Tối đa 20 tệp mỗi tin") with `Upload` icon in both `SpaceView` and `ThreadPanel`.
  - Drive Picker: "Chọn từ Tài liệu" is omitted because no drive picker exists in `components/drive` (documented in spec and OPEN).
- **M10 Optimistic Thumbnails**: Optimistic message payload includes `localUrl` (blob object URL) for instant image previews, which are revoked on settlement via `onSettled` in `useSendMessage` and `useSendReply`.
- **M11 Safe Paste Guard**: Clipboard paste only attaches images when neither `text/plain` nor `text/html` is present (allowing Office text copies to paste as text). Paste-to-attach is disabled in phone edit mode (`!editRef.current`).
- **M12 Spec Status**: `docs/specs/chat-attachments/spec.md` updated to accurately describe implementation status and document the drive picker omission.

### Low Findings Fixed
- **L13 Drop Test Hack**: Removed 20ms test-only timeout in `measureImage`.
- **L14 Limit Race Condition**: Enforced 20-file limit synchronously against `itemsRef.current` to prevent simultaneous paste/drop race conditions.
- **L15 Text/Quote Regression**: Fixed `legacyFilename` regex fallback in `MessageBlock` (removed bogus fallback) and preserved `QuoteBlock` on files-only messages that quote another message.
- **L16 Drop Zone Listener Leak**: Replaced per-render listener re-registration in `ThreadPanel` by using `uploadsRef`.
- **L17 Cache Consolidation**: Consolidated download URLs using standard `useDownloadUrl` options.
- **L18 Clean Code**: Removed dead `onFileUpload` code and raw `<button>` elements, replacing them with `<Pressable>`.
- **L19 Expanded Tests**: Added unit tests for `convertChatMsgToMessage` with `attachments` and bigint `sizeBytes`, ThreadPanel upload & send assertions, and Esc modal isolation.

---

## 2. Modified Files

1. `frontend/src/components/chat/ImageViewer.tsx` — Modal with `useModalFocus`, `createPortal`, focus return, `useDownloadFile`.
2. `frontend/src/components/chat/ImageViewer.test.tsx` — Verified focus trap, focus return, and download failure toast.
3. `frontend/src/components/chat/AttachmentGrid.tsx` — Clamped aspect ratio, `localUrl` support, opener pass-through, `useDownloadFile`.
4. `frontend/src/components/chat/AttachmentGrid.test.tsx` — Tested aspect ratio clamping, localUrl thumbnail, and download toast.
5. `frontend/src/components/spaces/MessageBlock.tsx` — Opener ref tracking, `legacyFilename` fix, quote block on files-only messages.
6. `frontend/src/components/spaces/MessageBlock.test.tsx` — Verified opener focus wiring, legacy format fallback, and quote rendering.
7. `frontend/src/hooks/useAttachmentUploads.ts` — `AbortController` upload cancellation, Vietnamese error mapping, synchronous limit.
8. `frontend/src/hooks/useAttachmentUploads.test.tsx` — Verified upload cancellation, Vietnamese error mapping, and limit enforcement.
9. `frontend/src/components/chat/AttachmentTray.tsx` — Real upload progress % (64%), Vietnamese error display.
10. `frontend/src/components/chat/AttachmentTray.test.tsx` — Verified progress display and error reason rendering.
11. `frontend/src/components/chat/ChatEditor.tsx` — Async send retention, `whyStatus`, safe paste guard, attach menus with `Pressable`.
12. `frontend/src/components/chat/ChatEditor.test.tsx` — Verified retention on error, disabled reason, paste guard, and menus.
13. `frontend/src/components/chat/ThreadPanel.tsx` — Panel isolation, drop overlay mockup copy, listener ref stability, async send.
14. `frontend/src/components/chat/ThreadPanel.test.tsx` — Verified attachment upload & reply send, Esc modal isolation inside panel.
15. `frontend/src/components/spaces/SpaceView.tsx` — ThreadPanel key isolation (`thread-${panel.id}`), drop overlay copy and icon, async send.
16. `frontend/src/api/messaging.ts` — Added `localUrl?: string` to `MessageAttachment`.
17. `frontend/src/hooks/useMessages.ts` — Object URL revocation in `onSettled` for `useSendMessage` and `useSendReply`.
18. `frontend/src/lib/chat-cache.test.ts` — Added unit test for `convertChatMsgToMessage` WS attachments with bigint `sizeBytes`.
19. `frontend/src/test/chat-fixtures.ts` — Supported POST `/messages` in fixtureApi returning mapped attachments.
20. `docs/specs/chat-attachments/spec.md` — Updated status line and documented omission of drive file picker.

---

## 3. Verification Gates

- **Unit / Component Tests**:
  - `npx vitest run src/components/chat/ImageViewer.test.tsx src/components/chat/AttachmentGrid.test.tsx src/components/spaces/MessageBlock.test.tsx src/hooks/useAttachmentUploads.test.tsx src/components/chat/AttachmentTray.test.tsx src/components/chat/ChatEditor.test.tsx src/components/chat/ThreadPanel.test.tsx src/lib/chat-cache.test.ts`
  - Result: 8 passed / 8 files (72 passed / 72 tests).
- **Full Vitest Suite**:
  - `npx vitest run`
  - Result: 143 passed, 1 failed (1818 passed / 1819 tests). The sole failure is the known `AssetsScreen.test.tsx:103` ("Phạm Hải Yến · 08:52"), identical to baseline.
- **ESLint**:
  - `npx eslint` across all touched files.
  - Result: 0 errors, 0 warnings.
- **TypeScript**:
  - `npm run typecheck:diff`
  - Result: 0 new errors introduced in any touched phase-06 file.
- **Build**:
  - `npm run build`
  - Result: exit code 0.
