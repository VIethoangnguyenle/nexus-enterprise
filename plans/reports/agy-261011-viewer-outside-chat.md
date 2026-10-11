# FileViewer Wiring Outside Chat Report

- Date: 2026-10-11
- Plan: `plans/261011-1035-file-preview/exec-04-viewer-wiring-outside-chat.md`
- Executor: agy (Antigravity)
- Mode: edit source, no commits, no git index changes.

## Overview

Wired `FileViewer` across all non-chat file preview surfaces according to `exec-04-viewer-wiring-outside-chat.md`:
1. Group's Tệp tab (`SpaceFiles.tsx`)
2. Tài liệu (`DriveScreen.tsx`, `DriveTable.tsx`, `DriveDetailPanel.tsx`)
3. Shared pattern cards (`FilePreviewCard.tsx`, `ImagePreviewCard.tsx`)

Protected surfaces: `components/chat/**`, `components/spaces/{MessageBlock,SpaceView,ChatNavigator,PhoneChatList}.tsx`, `components/files/**`, `routes/**`, `stores/**`, `api/messaging.ts`, `backend/**` were untouched.

---

## Changes Implemented

### 1. Group's Tệp tab (`SpaceFiles.tsx`)
- Clicking a file row opens `FileViewer` with all tab files as navigation items (`origin: 'chat'`).
- `onOpenInDrive` is wired when `file.parent_folder_id` exists, routing to `/drive?folder=${parent_folder_id}`.
- Focus returns to the clicked row element on close via `returnFocusRef`.
- Added test suite `SpaceFiles.test.tsx` verifying:
  - Opening `FileViewer` with tab files list
  - Previous/next navigation across files
  - Close / Escape restoring focus to opener element
  - `onOpenInDrive` invocation
  - No raw UUID / ID rendered on screen

### 2. Tài liệu Drive (`DriveScreen.tsx`, `DriveTable.tsx`, `DriveDetailPanel.tsx`)
- In `DriveTable.tsx`:
  - Clicking a file row (not a folder) triggers preview via `actions.onPreview`.
  - Row `⋯` action menu includes "Xem trước", "Tải xuống", and "Chi tiết".
- In `DriveDetailPanel.tsx`:
  - Added "Xem trước" button with `Eye` icon next to "Tải xuống".
  - Preserved existing download and rename actions.
- In `DriveScreen.tsx`:
  - Manages `previewIndex` and `previewOpenerRef`.
  - Maps visible non-folder files to `ViewerFile[]` list (`origin: 'drive'`).
  - Opens `FileViewer` with active file and list for seamless prev/next browsing.
- In `DriveScreen.test.tsx`:
  - Updated helpers (`openXlsx`, `openDetailPanel`) to access the detail panel via the `⋯` menu "Chi tiết" item.
  - Added tests for file row click preview, `⋯` menu preview, detail panel preview, focus restoration, and absence of raw UUIDs.
  - All 44 tests pass.

### 3. Pattern Cards (`FilePreviewCard.tsx`, `ImagePreviewCard.tsx`)
- `FilePreviewCard.tsx`:
  - Clicking card opens `FileViewer` with the file (or caller-supplied `files` array).
  - Download button uses `IconButton` with `e.stopPropagation()` to download without triggering preview.
  - Passed `returnFocusRef` for focus return on close.
- `ImagePreviewCard.tsx`:
  - Clicking the image preview opens `FileViewer` with the image (or caller-supplied `files` array).
  - Passed `returnFocusRef` for focus return on close.
- Added tests in `FilePreviewCard.test.tsx` and `ImagePreviewCard.test.tsx` (5 tests total) verifying preview, download separation, focus return, and no raw IDs.

### 4. Focus Return & UI Discipline
- All preview openers capture `openerRef` / pass `returnFocusRef` to `FileViewer`.
- When `FileViewer` closes, focus returns directly to the triggering row, button, or card.
- In all tests, verified that raw IDs and UUIDs are not displayed on screen (`noIds` regex test).

---

## Quality Gates & Verification

| Gate | Result | Notes |
|---|---|---|
| Touched tests | **50 / 50 passed** | `SpaceFiles.test.tsx` (5), `DriveScreen.test.tsx` (44), `FilePreviewCard.test.tsx` (3), `ImagePreviewCard.test.tsx` (2) |
| Full Vitest suite | **145 / 146 passed** (1859 / 1860 tests) | Only 1 pre-existing failure in `AssetsScreen.test.tsx` (known not-ours, called out in plan) |
| ESLint on touched files | **0 errors / 0 warnings** | Clean |
| TypeScript check (`npm run typecheck:diff`) | **0 new errors** | `không file nào tăng lỗi so với baseline` |
| Vite build (`npm run build`) | **Pass** | `xlsx-BSoArgVj.js` (493.28 kB / gzip 160.68 kB) is split into a separate chunk via dynamic import |

---

## Files Changed / Added

- `frontend/src/components/spaces/SpaceFiles.tsx` — Wired `FileViewer`, `returnFocusRef`, `onOpenInDrive`
- `frontend/src/components/spaces/SpaceFiles.test.tsx` — Tests for `SpaceFiles` FileViewer wiring
- `frontend/src/components/drive/DriveScreen.tsx` — FileViewer preview state, list mapping, and `returnFocusRef`
- `frontend/src/components/drive/DriveTable.tsx` — File row click preview, `⋯` menu "Xem trước" and "Chi tiết"
- `frontend/src/components/drive/DriveDetailPanel.tsx` — Added "Xem trước" button with `Eye` icon
- `frontend/src/components/drive/DriveScreen.test.tsx` — Updated panel opening helpers, added preview test cases
- `frontend/src/components/patterns/FilePreviewCard.tsx` — Added FileViewer wiring on card click, preserved download
- `frontend/src/components/patterns/FilePreviewCard.test.tsx` — Tests for `FilePreviewCard`
- `frontend/src/components/patterns/ImagePreviewCard.tsx` — Added FileViewer wiring on click
- `frontend/src/components/patterns/ImagePreviewCard.test.tsx` — Tests for `ImagePreviewCard`
- `plans/reports/agy-progress-viewer-outside-chat.log` — Updated step-by-step progress log
- `plans/reports/agy-261011-viewer-outside-chat.md` — Final report
