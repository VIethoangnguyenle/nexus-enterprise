---
title: "Execution plan — personal workspace UI (04/05): review fixes"
owner: Claude (decisions) · executor: agy/Gemini
status: ready
---

# Fix the review `plans/reports/code-reviewer-261011-1120-personal-ws-ui.md` (read it fully first)

Design: the same mockup/DESIGN.md sections as `exec-04-05-frontend.md` (read its design section).

## Decisions (fixed)
1. **Typecheck** — type the test's `details` in `SettingsScreen.test.tsx` as `WorkspaceDetails`. `npm run typecheck:diff`
   must show no new errors in any file you touch (run the whole command, not two files).
2. **Deep links** — one shared route guard helper (e.g. `lib/workspace-areas.ts` or next to the existing guard) used by
   `routes/_workspace/{assets,approval}.tsx`: in `beforeLoad` it AWAITS the workspace list via
   `queryClient.ensureQueryData` (same key/fn as the app uses) and redirects before anything renders. Tests: cold cache
   (reload / bookmark / workspace switch) for a personal workspace → redirect, no area render, no area request; cold
   cache for an organization → renders.
3. **Areas appear after convert** — give the test fixture's workspace list a `type`; add a test: personal → convert
   succeeds → Phê duyệt and Tài sản appear in the sidebar AND the phone tab bar / More sheet without reload.
4. **Admin areas** — the backend will omit Phê duyệt and Tài sản for personal workspaces (Claude does that change).
   Remove the browser-side filter so the screen shows exactly what the server returns; keep a test that it renders
   what it gets.
5. **Back on Tạo workspace** — per mockup §9: the "Tạo workspace" link carries where the person came from
   (validated `from` search param, internal paths only); Back returns there, falling back to Chọn workspace.
6. Mediums/Lows: 44px phone targets (convert button, "Đặt quyền"); switcher dropdown — the label, divider and
   "Tạo workspace" link move OUTSIDE the listbox (listbox holds only options); visible keyboard focus on the type radio
   cards (focus-visible ring from DESIGN.md tokens); several owners render as separate PersonChips (split the server's
   list; if the API only gives one joined string, render it as a text list, not one avatar, and note it in OPEN);
   fix every Low in the report.

## Files
The 23 files listed in the review report + `lib/` helper and tests. Do NOT touch backend/**, components/chat/**,
components/spaces/** except where the report names the sidebar/nav, lib/push*, lib/pwa*, sw.ts,
routes/_workspace/channels.*. `SettingsScreen.test.tsx` also holds a push-settings expectation — leave it.

## Gates
Touched tests, full `npx vitest run` (known not-yours: AssetsScreen, ApprovalScreen), `npx eslint` on touched files,
`npm run typecheck:diff` (full), `npm run build`.

## Rules
PROGRESS LOG after EVERY step: `plans/reports/agy-progress-personal-ws-fixes.log` (`<time> | step | result`); read it
first when resuming. No commits, no git index. NEEDS_CONTEXT instead of inventing UI.
Report: `plans/reports/agy-261011-personal-ws-fixes.md`.
