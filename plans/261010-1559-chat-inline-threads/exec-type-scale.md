---
title: "Execution plan — larger chat type scale"
owner: Claude (planning, review) · executor: agy/Gemini
status: ready (start after exec-06 attachments UI is committed — shares MessageBlock)
---

# Execution plan — chat text a little larger than Google Chat

APPROVED (user, 2026-10-11): "chỉ phần chat"; phone messages 16px. Source of truth: DESIGN.md §3 type table (now has
`text-md` 15/23) and the designer report `plans/reports/ui-ux-designer-261011-1100-type-scale.md` — its per-file
class-change table is the work list; the mockup `design/mockups/type-scale.html` ("Đề xuất" column) is the target.

## Steps
1. Add the token in `frontend/src/index.css` `@theme`: `--text-md: 15px; --text-md--line-height: 23px` (match how
   the other text tokens are declared there).
2. Apply the report's class table: desktop chat reading text → `text-md` (message body, sender name 600, navigator
   rows, feed row names); phone (< md) message body and phone list row names → `text-base` (16/24); chat secondary
   text 13 → `text-sm` 14 (section headers in the navigator, thread summary row, quotes, inline code); "đã sửa"
   11 → `text-xs`. Keep: timestamps 12, composer 16, all UI/table/admin text 14, sidebar labels.
3. Row heights: keep 36 (navigator), 32, 64 (phone list) as minimums — use `min-h-*`, never a fixed `h-*`, where
   the report says so.
4. Update DESIGN.md only if something in §6 still says 13px/15px for these (the §3 table is already updated).
5. Tests: no test asserts font classes today; add one focused test that the message body uses `text-md` on desktop
   and `text-base` under md (class presence), so the scale cannot silently regress.

## Gates
`cd frontend && npx vitest run` (only the known AssetsScreen failure), `npm run lint`, `npm run typecheck:diff`,
`npm run build`. Screenshot the conversation stream + navigator at 1440 and 390 (light) with Playwright into
`plans/261010-1559-chat-inline-threads/reports/type-scale-screens/` for review.

## Rules
- PROGRESS LOG after EVERY step: `plans/reports/agy-progress-type-scale.log` (`<time> | step | files | result`);
  read it first when resuming.
- Only touch chat UI files named in the report + `index.css` + the new test. No commits, no git index.
- Report: `plans/reports/agy-<date>-type-scale.md`.
