# FileViewer, steps 1-3 (frontend) - report

Status: DONE_WITH_CONCERNS. Step 4 (wiring) not done, as instructed.

## Files
New, `frontend/src/`:
- `hooks/useFilePreview.ts` + `.test.tsx` (key `['drive','preview',fileId]`, staleTime 4 min, no retry on 403/404, one retry otherwise)
- `components/files/FileViewer.tsx` (+ `FileViewer.test.tsx`, 36 tests): dialog >=768 / full screen <768, header, stage, prev/next, n / N, phone sheet, keys, focus
- `components/files/FileViewerBody.tsx`: picks body from the preview answer; unavailable / 403 / 404 / network states
- `components/files/ImageBody.tsx` (fit / 100% + drag, toolbar), `TextBody.tsx` (text, code, CSV), `SheetBody.tsx` (SheetJS, lazy), `ViewerTable.tsx` (+ truncation strip), `ViewerStates.tsx`
- `components/files/viewer-text.ts` + `.test.ts` (capped read, line cap, CSV parser, flavor, middle truncation)

Edited: `api/drive.ts` (types `FilePreview`, `PreviewUnavailableReason`; `driveApi.preview(fileId)`), `package.json` + `package-lock.json` (xlsx).

## SheetJS choice
`npm install https://cdn.sheetjs.com/xlsx-0.20.3/xlsx-0.20.3.tgz` run in `frontend/` (pinned exact version, in package.json as that URL). The npm registry `xlsx` is frozen at 0.18.5 (SheetJS stopped publishing there; that version has known prototype-pollution and ReDoS advisories). 0.20.3 is the maintained distribution, fixed for both. `import('xlsx')` inside `SheetBody.parseWorkbook` only. Probe build with FileViewer as entry: `xlsx-*.js` 493 kB (160 kB gz) is its own chunk, entry 98 kB. The normal `npm run build` shows no xlsx chunk yet because nothing imports FileViewer (nothing wired). Re-check after step 4. Probe config and entry were deleted. `sheetRows: 501` limits parse to header + 500 rows; `raw:false` shows formatted values, never formulas; the full row count comes from `!fullref`.

## Gates
- vitest: `src/components/files` + `useFilePreview.test.tsx` = 3 files, 65 tests, all pass.
- eslint on my files: clean.
- `typecheck:diff`: no errors in my files. It does report new errors in other agents' files (chat/AttachmentGrid.tsx and several chat/spaces tests), not mine.
- `npm run build`: passes (see chunk note above).
- Playwright screens (plan step 5): not run, nothing is wired to open the viewer.

## Props contract (also documented at top of FileViewer.tsx)
```ts
interface ViewerFile { id: string; name: string; mime?: string; size?: number;
  uploaderName?: string; uploaderAvatarUrl?: string; uploaderId?: string /* colour key only */;
  time?: unknown /* anything lib/format reads */; parentFolderId?: string }
interface FileViewerProps { files: ViewerFile[]; index: number; onClose(): void;
  origin?: 'chat' | 'drive' /* default 'chat' */; onOpenInDrive?(file: ViewerFile): void;
  returnFocusRef?: RefObject<HTMLElement | null> }
```
Mount while open, unmount on close: `{open && <FileViewer .../>}`. `index` is the start only; the viewer navigates itself. "Mở trong Tài liệu" shows only when `onOpenInDrive` is set, the shown file has `parentFolderId`, and `origin !== 'drive'`. `origin:'drive'` also changes the sender verb to "tải lên" (chat uses "gửi lúc"). Download goes through `useDownloadFile`. Query key for text/sheet content is `['drive','preview-text'|'preview-sheet', url]`.

## Behaviour notes
- Header from opener data immediately; only the body waits. 403: title "Trình xem tệp", no name/type/size/sender/actions. 404: name dimmed + "Đã xoá", no actions. Arrows still work in both.
- Office-by-extension (doc, docx, rtf, ppt, pptx, odt, odp) shows "Đang chuẩn bị bản xem trước…" while pending (indeterminate progressbar eased to 92%, "Tệp lớn cần lâu hơn..." after 15 s, "Tải xuống thay vì chờ", aria-live "Bản xem trước đã sẵn sàng"). Others get a skeleton after 300 ms.
- Text/code/CSV: fetched, capped 1 MB / 500 lines, rendered only as React text (test proves `<script>` is literal). Strip says "trên M dòng" only when the whole file was read.
- Phone sheet is handled inside the viewer (Esc closes sheet first, Tab trapped in it, viewer behind `inert`), not via a second `useModalFocus`, because its cleanup would clear `#root` inert while the viewer is still open.
- Not using `useModalFocus` focus trap for `[href]`: its FOCUSABLE list already uses `a[href]`.

## Open items
- PDF page pill ("Trang n / N", zoom -/+/%) not built: no PDF library allowed, so PDF/Office rely on the browser's built-in viewer toolbar in an `<iframe>`. Keys + - 0 therefore only act on images. Pinch-zoom on the phone is not implemented for images (browser default only); drag-pan at 100% is.
- NEEDS_CONTEXT (copy not in the mockup, I wrote plain Vietnamese, please review): `conversion_unavailable` text "Dịch vụ xem trước đang tạm ngưng. Tải xuống để mở bằng ứng dụng trên máy."; `conversion_busy` text "Máy chủ đang bận chuyển các tệp khác. Thử lại sau ít phút, hoặc tải xuống để mở trên máy." (with "Thử lại"); `conversion_failed` text is generic ("có thể tệp có mật khẩu hoặc bị hỏng") because the API gives no cause. `too_large` omits the mockup's "100 MB" since the real limit is 50 MB source / 80 MB PDF. Mockup's "Thử lại" for timeouts is shown for busy/unavailable, not for failed.
- Mockup's per-kind extras not built: audio duration, image aspect-ratio skeleton (size unknown), swipe shortcut, motion specs (only `animate-fade-in`).
- Audio/video `Not implemented: navigation` jsdom noise in one test run is harmless (download anchor click).
- `package.json`/lockfile were already modified by others; my change is the single `xlsx` dependency line + its lock entries.
- The old `ImageViewer` is untouched (step 4).

Status: DONE_WITH_CONCERNS
Summary: Hook, API call, FileViewer and all bodies (image, PDF iframe, media, text/code/CSV, lazy SheetJS sheets, every state) are in with 65 passing tests, clean lint and typecheck, and a confirmed separate xlsx chunk. Wiring (step 4), the PDF page/zoom pill and a few error-copy lines need a decision from you.
