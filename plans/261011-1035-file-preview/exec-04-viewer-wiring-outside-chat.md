---
title: "Execution plan — FileViewer wired outside chat (Tệp tab, Tài liệu, shared file cards)"
owner: Claude (decisions) · executor: agy/Gemini
status: ready
---

# Open every non-chat file in FileViewer

FileViewer is built and tested: `frontend/src/components/files/FileViewer.tsx` — its props contract is documented at
the top of that file and in `plans/reports/fullstack-developer-261011-1140-file-viewer.md` (read both). Design:
`design/mockups/file-viewer.html` + DESIGN.md §6 "Trình xem tệp (FileViewer)". This is step 4 of
`exec-03-file-viewer.md` for everything EXCEPT chat attachments (chat is wired later by another wave).

## Steps (tests first for each)
1. Group's Tệp tab `frontend/src/components/spaces/SpaceFiles.tsx`: clicking a file opens FileViewer with the tab's
   visible files as the list (prev/next across them), `origin: 'chat'`, `onOpenInDrive` when the file has a parent
   folder the app can open (use the drive route that already exists for a folder; if none fits, omit).
2. Tài liệu `frontend/src/components/drive/*`: clicking a file (not a folder) opens FileViewer over the current
   folder's files, `origin: 'drive'`; `DriveDetailPanel` gets a "Xem trước" action. Keep existing download/rename
   actions unchanged.
3. `frontend/src/components/patterns/{FilePreviewCard,ImagePreviewCard}.tsx`: clicking the card opens FileViewer
   with that one file (callers that pass a list may pass it). Keep the download button behaviour.
4. Focus returns to the clicked element on close (pass `returnFocusRef`). No UUID/ID on screen.
5. Gates: touched tests, full `npx vitest run` (known not-yours: AssetsScreen; another worker is editing
   components/chat/** — report failures there separately, do not fix them), `npx eslint` on touched files,
   `npm run typecheck:diff` (report only errors in your files), `npm run build` (note whether a separate SheetJS chunk
   now appears in the build output listing).

## Do NOT touch
components/chat/**, components/spaces/{MessageBlock,SpaceView,ChatNavigator,PhoneChatList}.tsx, components/files/**
(use FileViewer as is; if it truly needs a change, write NEEDS_CONTEXT), routes/**, stores/**, api/messaging.ts,
backend/**.

## Rules
PROGRESS LOG after EVERY step: `plans/reports/agy-progress-viewer-outside-chat.log` (`<time> | step | result`); read
it first when resuming. No commits, no git index. Report: `plans/reports/agy-261011-viewer-outside-chat.md`.
