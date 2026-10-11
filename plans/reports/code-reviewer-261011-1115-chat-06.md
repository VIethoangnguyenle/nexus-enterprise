# Code review: chat phase 06 attachments UI (uncommitted, Gemini executor)

Plan: `plans/261010-1559-chat-inline-threads/exec-06-attachments-ui.md`. The executor wrote no final report;
only the progress log `plans/reports/agy-progress-chat-06.log` exists.

## Gates (run 2026-10-11 11:13)
- `npx vitest run`: 1 failed / 1611 passed (123 files). The only failure is the known `AssetsScreen.test.tsx:103`
  failure ("Phạm Hải Yến · 08:52"). It does not come from this task (the personal-workspace / assets work owns it).
- `npx eslint` on the 19 files in scope: no issues.
- `npm run typecheck:diff`: exit 1. Two files have new errors, and neither comes from this task:
  - `src/components/spaces/ChatNavigator.tsx(1,17): error TS6133: 'useRef' is declared but its value is never read.`
    The file has no uncommitted changes. The error is **already committed**: in `404ec5f fix(frontend): one place
    to create a chat section` (10:39), the last use was removed (`const newSectionRef = useRef<HTMLButtonElement>(null)`),
    but `useRef` stayed in the import. It has nothing to do with `api/messaging.ts`. The fix is to drop `useRef`
    from the import. That fix belongs to the 07/08 section work, not this task.
  - `src/components/settings/SettingsScreen.test.tsx` lines 65, 508, 542, 567, 585, 605 and 625 each raise
    `TS2353 'type' does not exist in type '{ name: string; description: string; }'`. They come from the
    personal-workspace task (the WorkspaceTab / workspaces API type).
  - None of the phase-06 files adds a type error.
- `npm run build`: exit 0. The only output is the usual chunk-size warning.

## Critical
None. There is no auth or data-loss issue on the server: the backend validates attachments (spec, e3582b3).

## High
1. **Esc in the image viewer also closes the side panel.** Files: `components/chat/ImageViewer.tsx:54-71` and
   `components/spaces/SpaceView.tsx:133-140`. Both listeners sit on `window`. SpaceView's listener was
   registered earlier, so it runs first, sees `!e.defaultPrevented`, and calls `setPanel(null)`. To reproduce,
   open a thread, open an image inside it, and press Esc: the viewer and the thread panel close together. The
   `stopPropagation()` on a `window` listener does nothing. Fix: use the shared `hooks/useModalFocus.ts`. It
   handles Esc on `document` and marks the event taken, cycles Tab, makes the layer topmost-aware and returns
   focus. Better still, build the viewer on `composites/Dialog`. The viewer currently reimplements all of this
   and gets it wrong.
2. **Focus does not return to the clicked thumbnail (plan decision 8, a11y), and the test is a phantom.** Files:
   `MessageBlock.tsx:231-240` and `ImageViewer.tsx:39-42`. MessageBlock never passes `returnFocusRef`, and it
   unmounts the viewer (`viewerOpen && …`), so the close branch of the effect never runs. After closing, focus
   lands on `<body>`. `ImageViewer.test.tsx` only passes because the test supplies `returnFocusRef` and
   re-renders with `open={false}`, which production never does. The viewer also has no Tab trap, so Tab walks
   into the page behind an `aria-modal` dialog. `useModalFocus` fixes both problems.
3. **Downloads are blocked by popup blockers and fail silently.** Files: `AttachmentGrid.tsx:82-92` and
   `ImageViewer.tsx:84-93`. Both call `window.open(url,'_blank')` after an `await`, which is no longer inside
   the user gesture. Safari, including the installed iOS PWA, and Firefox block it. Errors are swallowed with
   `catch { // ignore }`, so the user sees nothing. The repo already has `useDownloadFile()` in
   `hooks/useDownloadUrl.ts`, which clicks an `<a download>` and raises a toast on failure. The removed
   `FilePreviewCard` used it, so this is a regression from the old single-file branch.

## Medium
4. **Measured image size is never used, and portrait photos are cropped.** File: `AttachmentGrid.tsx:144-146`.
   A single image is forced into `aspect-4/3` with `object-cover`. A portrait receipt or phone photo (the main
   chat use) loses most of its height, and the layout cannot reserve the right box. Use `width/height` to set
   `aspect-ratio`, clamped, and fall back to 4/3 only when the dimensions are missing.
5. **A send failure loses the attachments.** File: `ChatEditor.tsx:249`. `currentUploads?.clear()` runs right
   after `onSend`, before the mutation settles. If the POST fails (offline, 4xx on a stale file), the toast says
   it could not send, but the tray is empty and the uploaded files have to be added again. Keep the items until
   success, or restore them in `onError`.
6. **The thread tray carries over between threads.** Files: `ThreadPanel.tsx:56` and `SpaceView.tsx:438`.
   ThreadPanel is mounted with `key="thread"`, so switching from thread A to thread B keeps the same
   `useAttachmentUploads` state. Files queued in A are then sent as a reply in B. Fix: key the panel by
   `panel.id`, or clear the queue when `messageId` changes.
7. **Removing a chip does not cancel its upload.** File: `useAttachmentUploads.ts:149-151`. `remove` only
   filters the state. The in-flight `createFile`/`uploadToStorage`/`confirmFile` still finishes, and the file
   still shows up in the channel's Drive folder. The same happens on `retry` after a failure that came after
   `createFile`: a pending file record is orphaned. At minimum, document this. Better: use an AbortController,
   or delete the file once it confirms after removal.
8. **The raw error string is shown on screen.** Files: `useAttachmentUploads.ts:108` and
   `AttachmentTray.tsx:102`. The chip shows `err.message || 'Upload failed'`, which is English, server text or
   axios text. The mockup (`spaces.html` line 1204) wants "lỗi nói vì sao" in Vietnamese. Use
   `explain(err, 'tải tệp lên')` from `lib/errors`.
9. **The composer does not render the mockup's states.** File: `spaces.html` `#tray-b`, lines 1146-1206. It
   specifies "Đang tải 64%" with real progress (`progress` is never set; the bar is a pulsing 100% bar), a
   `tray-note`, and a `why` status **next to Gửi** explaining why it is locked (missing: the button just
   disables silently). It also specifies the Đính kèm menu with "Tải lên từ máy". No drive picker exists in
   `components/drive`, so omitting "Chọn từ Tài liệu" is allowed, but the executor never reported this. The
   drop overlay copy differs: the mockup reads "Thả tệp để đính kèm" + "Tối đa 20 tệp mỗi tin", the code reads
   "Thả tệp vào đây để đính kèm". The mobile "+" sheet (Máy ảnh / Ảnh / Tệp / Tài liệu,
   `chat-mobile.html`) is not implemented.
10. **The optimistic message has no local thumbnail (plan decision 3).** Files: `SpaceView.tsx` handleSend and
    `ThreadPanel.tsx` onSend. They pass the real `drive_file_id`, so the tile fetches a presigned URL and shows
    a pulse until it arrives. No object URL is used.
11. **Pasting from Word, Excel or a browser can attach an image instead of pasting text.** File:
    `ChatEditor.tsx:115-139`. Any clipboard item with `kind === 'file'` or `image/*` triggers the attach path
    and `preventDefault()`. Office copies (and some browsers' "copy image" with HTML) put an `image/png`
    rendering next to `text/html`, so the user's text is swallowed and a picture is attached. Only intercept
    when the clipboard has files and no `text/plain`/`text/html`, or when `clipboardData.files` is non-empty
    and there is no text. Paste is also active in phone edit mode, where the tray shows but "Lưu" ignores it.
    Gate the paste on `!editRef.current`.
12. **`spec.md` says "Status: matches code", which is false.** File: `docs/specs/chat-attachments/spec.md:39-47`.
    It claims "Esc closes with focus restored to thumbnail". Items 2, 9 and 10 contradict the status line.

## Low
13. **The 20ms timeout in `measureImage` is a test-only hack in production code.** File:
    `useAttachmentUploads.ts:55`. When the `Image()` fallback runs (no `createImageBitmap`, or it rejects, for
    example on SVG), the timer almost always wins and the dimensions are dropped.
14. **The 20-file limit can be exceeded.** File: `useAttachmentUploads.ts:121`. `itemsRef` only updates on
    render, so two `add()` calls in one tick (drop + paste, or a double change event) each see the old count.
    Compute the limit inside the `setItems` updater.
15. **A drive-file message whose content does not match `📎 name` loses its text.** File: `MessageBlock.tsx:74-89`.
    `legacyFilename` now falls back to `'file'` when the regex misses, so such a message now hides its text and
    shows a card called "file" (the old code rendered the text). Also, `hasText` is false for a files-only
    message that quotes another message, so `QuoteBlock` (rendered inside the text bubble) disappears.
16. **The thread drop-zone effect re-subscribes on every render.** File: `ThreadPanel.tsx:139-167`. Its
    dependency is `[uploads]`, a new object each render, so it removes and re-adds three native listeners on
    every render. Use a ref, or React `onDrop` on the panel body.
17. **A duplicate URL cache.** Files: `AttachmentGrid.tsx:31-36` and `ImageViewer.tsx:75-80`. They use
    `['drive','download',id]` (5 min, as the plan asked) next to the existing
    `downloadUrlQueryOptions`/`keys.drive.downloadUrl` (10 min). The same URL is cached twice. Reuse
    `useDownloadUrl` and note the plan deviation.
18. **Dead and catch-all code.** `ChatEditor.onFileUpload` is no longer passed by any caller (dead branch at
    lines 301-307). The `Đính kèm` button now shows even when neither `uploads` nor `onFileUpload` is set. There
    is a `catch (err: any)`.
19. **Weak tests.**
    - The ThreadPanel test "sends attachments with reply" asserts only that the overlay hides. It never checks
      upload or send.
    - The `chat-cache.ts` WS attachment mapping (bigint `sizeBytes`) has no test.
    - The viewer is not portaled. It currently works because framer-motion leaves `transform: none` at rest,
      but it is fragile inside the animated `motion.aside`.

## Checked and OK
- One message carries text + attachments.
- Files-only send works (`SpaceView`/`ThreadPanel` guards and `useSendMessage`/`useSendReply` pass `attachments`).
- Gửi is disabled while any item is uploading or errored. The Enter path re-checks.
- Edit mode bypasses the attachment rules.
- The 21st file is refused with the toast.
- Grid layout is 1 / 2 / 3 / 4, with "+N" on the 4th tile and N = images − 4.
- File cards show "LOẠI · size" via `fileKind`.
- No UUID is rendered anywhere; aria-labels use file names.
- `SpaceView` is keyed by `channelId`, so the main tray never leaks across channels.
- 07/08 composer features (formatting, mentions, edit, Enter) keep their tests green (18 ChatEditor tests).

## Exact file list for the commit (this task only)
```
docs/specs/chat-attachments/spec.md
frontend/src/api/messaging.ts
frontend/src/hooks/useMessages.ts
frontend/src/hooks/useAttachmentUploads.ts
frontend/src/hooks/useAttachmentUploads.test.tsx
frontend/src/lib/chat-cache.ts
frontend/src/test/chat-fixtures.ts
frontend/src/components/chat/AttachmentTray.tsx
frontend/src/components/chat/AttachmentTray.test.tsx
frontend/src/components/chat/AttachmentGrid.tsx
frontend/src/components/chat/AttachmentGrid.test.tsx
frontend/src/components/chat/ImageViewer.tsx
frontend/src/components/chat/ImageViewer.test.tsx
frontend/src/components/chat/ChatEditor.tsx
frontend/src/components/chat/ChatEditor.test.tsx
frontend/src/components/chat/ThreadPanel.tsx
frontend/src/components/chat/ThreadPanel.test.tsx
frontend/src/components/spaces/MessageBlock.tsx
frontend/src/components/spaces/MessageBlock.test.tsx
frontend/src/components/spaces/SpaceView.tsx
```
`lib/chat-cache.ts` is not named in the progress log, but its whole diff is the WS `attachments` mapping, so it
belongs here. `api/messaging.ts`, `useMessages.ts`, `chat-fixtures.ts` and `SpaceView.tsx` contain only
attachment hunks. The executor's report file is missing, so no `plans/reports/agy-*` file goes into the commit.

## Recommended order
Before committing, fix items 1, 2, 3 and 6. Then fix items 4, 5, 8 and 11, and correct the spec status line
(item 12). Track item 9 (the mockup states) as a follow-up, or do it now if the design gate is strict.

Status: DONE_WITH_CONCERNS
Summary: The gates are clean for this task: lint and build pass, the only vitest failure is AssetsScreen, and the ChatNavigator TS6133 is pre-committed in 404ec5f. Three High defects block a commit as is: Esc closes the thread panel along with the viewer, focus is not returned and there is no focus trap (the test is a phantom), and downloads use window.open after an await, which popup blockers stop and which fails silently.
