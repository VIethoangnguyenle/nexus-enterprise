---
title: "Execution plan — pinch zoom for images in the FileViewer (phone)"
owner: Claude (decisions) · executor: agy/Gemini
status: ready
---

# Pinch zoom on images

Design: `design/mockups/file-viewer.html` notes (line ~482 "Phóng: ảnh … dùng chụm hai ngón; viên công cụ chỉ còn Vừa
khung / 100%"; keys `+ − 0`) and DESIGN.md §6 FileViewer. Code: `frontend/src/components/files/ImageBody.tsx`
(+ `FileViewer.tsx` only if wiring needs it). PDF stays on the browser's built-in viewer (decided: no PDF library).

## Steps (tests first)
1. Two-finger pinch on the image (pointer events, 2 active pointers) scales between fit (1×) and 4×, centred on the
   pinch midpoint; one-finger drag pans when zoomed (> 1×); double-tap toggles fit / 2×. At fit, a horizontal swipe
   still goes to the previous/next file as today.
2. The phone tool pill shows only "Vừa khung" / "100%" per the mockup; `+ − 0` keys keep working on desktop.
3. `touch-action: none` only on the zoomable image surface while zoomed (page scroll unaffected otherwise);
   `prefers-reduced-motion` → no animated transitions.
4. Tests (vitest + Testing Library, synthetic PointerEvents): pinch scales, pan only when zoomed, double-tap toggles,
   swipe at fit navigates, clamp at 1× and 4×.
5. Gates: touched tests, full `cd frontend && npx vitest run` (known: AssetsScreen), `npx eslint` touched files,
   `npm run typecheck:diff`.

## Rules
Touch ONLY `frontend/src/components/files/**`. Another worker is running a browser test against the dev stack — that's
fine. PROGRESS LOG after EVERY step: `plans/reports/agy-progress-pinch-zoom.log`. No commits, no git index.
Report: `plans/reports/agy-261011-pinch-zoom.md`.
