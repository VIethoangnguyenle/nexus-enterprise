---
title: "Execution plan — phase 03 FileViewer (frontend)"
owner: Claude (planning, review) · executor: agy/Gemini
status: ready (start after chat 06 attachments UI is committed — it replaces chat's ImageViewer)
---

# Execution plan — one viewer for every file

APPROVED design: `design/mockups/file-viewer.html` + DESIGN.md §6 "Trình xem tệp (FileViewer)" (read both fully,
incl. the mockup's notes). Backend contract (`docs/specs/file-preview/spec.md`, LIVE):
`GET /api/drive/files/{id}/preview` (same auth as download) →
`{kind:"native", url, mime}` | `{kind:"pdf", url, source_mime}` | `{kind:"sheet", url, mime}` |
`{kind:"unavailable", reason: "unsupported"|"too_large"|"conversion_failed"|"conversion_unavailable"|"conversion_busy"}`;
403/404 like download. The request blocks while an Office file converts (≤ 45 s) → show "Đang chuẩn bị bản xem
trước…" while pending, with "Tải xuống thay vì chờ".

## Steps (tests first for each)
1. API + hook: `api/drive.ts` `preview(fileId)`; `hooks/useFilePreview.ts` (TanStack Query key
   `['drive','preview',fileId]`, staleTime ~4 min — presigned URLs expire; no retry on 403/404).
2. `components/files/FileViewer.tsx` (+ test): dialog ≥768 / full screen <768 per the mockup; header (type icon via
   `lib/file-kind.ts`, middle-truncated name, "Loại · dung lượng", PersonChip + time, actions Tải xuống / Mở trong
   Tài liệu / Đóng; phone ⋯ sheet), stage with prev/next + "n / N" over a provided list of files, focus trap with
   `a[href]` (NOT `[href]` — SVG `<use href>` escapes the trap), Esc/←/→/+/−/0, return focus to the opener.
3. Bodies by kind: image (fit / 100% + drag); pdf (render with the browser's built-in PDF viewer in an `<iframe>` of
   the presigned URL — no new PDF library; page/zoom pill only if achievable without a library, otherwise rely on the
   built-in toolbar and note it); video/audio native `controls`; text/code/csv (fetch the URL, cap 1 MB / 500 lines,
   mono, line numbers for code, CSV table with sticky header + the "Đang xem N dòng đầu…" strip); sheet: parse with
   SheetJS loaded lazily (`xlsx` from the SheetJS CDN ESM build pinned to an exact version, or the npm package —
   pick the maintained SheetJS distribution and justify in the report; dynamic import so it never enters the main
   bundle), tabs per sheet, 500 rows, values not formulas; unavailable reasons and 403/404 states exactly as mocked.
   Never render html/svg as markup (the backend already sends them as text/plain — still treat any `text/*` as text).
4. Wire it everywhere a file is opened: chat attachments (replace `components/chat/ImageViewer.tsx` usage from
   phase 06 — images AND other files of the same message open in FileViewer; delete ImageViewer and move its tests'
   intent), the group's Tệp tab (`components/spaces/SpaceFiles.tsx`), Tài liệu (`components/drive/*` — clicking a
   file opens FileViewer; DriveDetailPanel gets "Xem trước"), and `components/patterns/{FilePreviewCard,
   ImagePreviewCard}.tsx` clicks. "Mở trong Tài liệu" only when the file has a parent folder the viewer can open
   (use what the drive item API returns; if unknown, hide it).
5. Gates: vitest (only the known AssetsScreen failure; report others separately), lint, typecheck:diff, build
   (check the SheetJS chunk is separate). Playwright on the running dev stack (Vite :5173; OTP 999999, rate-limited;
   reuse one session): open an image, a PDF, a .docx (first view then cached), an .xlsx, a .txt, a .zip at 1440 and
   390 → screenshots `plans/reports/file-viewer-screens/`.

## Rules
- PROGRESS LOG after EVERY step: `plans/reports/agy-progress-file-viewer.log`; read it first when resuming.
- No backend edits. No commits, no git index. NEEDS_CONTEXT instead of inventing UI.
- Report: `plans/reports/agy-<date>-file-viewer.md`.
