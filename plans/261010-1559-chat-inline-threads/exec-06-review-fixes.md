---
title: "Execution plan — chat 06 attachments UI: review fixes"
owner: Claude (decisions) · executor: agy/Gemini
status: ready
---

# Fix the review of chat phase 06 (`plans/reports/code-reviewer-261011-1115-chat-06.md` — read it fully first)

Design sources stay the same as `exec-06-attachments-ui.md` (read its design section): `design/mockups/spaces.html`
`#composer` / `#tray-b` / image viewer, `design/mockups/chat-mobile.html` composer "+" sheet. Note: the later
file-preview phase replaces ImageViewer with a shared FileViewer, but fix it properly now anyway.

## Decisions (fixed — do not re-decide)
1. **H1 Esc** — the viewer is a real modal: build it on the repo's `hooks/useModalFocus.ts` (or `composites/Dialog`
   if that fits the mockup); Esc in the viewer stops propagation so SpaceView's window listener never sees it.
   Test: Esc with viewer open inside an open thread panel closes ONLY the viewer.
2. **H2 focus** — Tab is trapped inside the viewer (`a[href]`, not `[href]`); on close, focus returns to the thumbnail
   that opened it (MessageBlock passes the opener). Test the real wiring through MessageBlock, not a test-supplied ref.
3. **H3 downloads** — use the existing `components/patterns/FilePreviewCard.tsx` download path (`useDownloadFile`)
   in AttachmentGrid and the viewer; no `window.open` after an await. On failure show a Vietnamese toast
   ("Không tải được tệp. Thử lại."). Test the failure toast.
4. **Medium, all fixed:**
   - Single image keeps its measured aspect ratio, clamped per the mockup (no forced 4:3 crop).
   - Tray is cleared only after the send succeeds; on failure the files stay and the error shows.
   - Tray state is per composer target (conversation id + thread root id); switching thread does not carry files over.
   - Removing a chip aborts its upload (AbortController) and nothing is attached.
   - Chip errors are Vietnamese reasons mapped from status (too large / not allowed / network / generic), never raw
     server text.
   - Mockup states: real upload progress % on the chip, the reason next to a locked Gửi, the attach menu and the
     drop-zone copy exactly as in `spaces.html`, the phone "+" sheet as in `chat-mobile.html`. If an item in those
     mockups needs a backend capability that doesn't exist, skip just that item and list it under OPEN.
   - "Chọn từ Tài liệu": NOT built (no drive picker exists) — hide it, record in the report.
   - Optimistic message shows local thumbnails (object URLs, revoked when the server message replaces it).
   - Paste: attach clipboard images only when the clipboard has no `text/plain`/`text/html` (Word/Excel paste stays
     text); paste-to-attach is off in phone edit mode.
   - `docs/specs/chat-attachments/spec.md`: make the status line true (list what is not built: drive picker).
5. **Low:** fix the race over the 20-file limit, the MessageBlock legacy-text/quote regression, listeners re-attached
   every render, the duplicate download-URL cache, and make the ThreadPanel test assert the send. Drop the 20 ms timer
   if it only exists for tests.

## Files
Only the 20 files listed at the end of the review report, plus their tests. Do NOT touch backend/**,
components/settings/**, components/auth/**, lib/push*, lib/pwa*, sw.ts, routes/**.

## Gates
Touched tests, then full `npx vitest run` (known failures not yours: AssetsScreen, ApprovalScreen — report others),
`npx eslint` on touched files, `npm run typecheck:diff`, `npm run build`.

## Rules
PROGRESS LOG after EVERY step: `plans/reports/agy-progress-chat-06-fixes.log` (`<time> | step | result`); read it
first when resuming. No commits, no git index. NEEDS_CONTEXT instead of inventing UI.
Report: `plans/reports/agy-261011-chat-06-fixes.md`.
