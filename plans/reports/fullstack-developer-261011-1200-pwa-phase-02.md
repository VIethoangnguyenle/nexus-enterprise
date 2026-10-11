# PWA phase 02 report

## Decisions
- vite-plugin-pwa 2.0.0 supports Vite 8; used with `injectManifest` and a hand-written SW (`frontend/src/sw.ts`, workbox-precaching/routing). `rollupFormat: 'iife'` is required (default `.mjs` rename fails on Vite 8/rolldown). Precache limit raised to 4 MiB (entry chunk 2.12 MB). Phase 04 can `importScripts` OneSignal's worker into sw.ts.
- SW registered in production only (`src/lib/pwa-register.ts`, called from main.tsx); dev server and proxy table untouched.
- One app-wide OfflineBar in `_workspace.tsx` (4 lines: 2 imports, `<OfflineBar/>` under MobileTopBar, `<UpdateBar/>` after MobileNav). Hook `useConnectionLost(graceMs=3000)` extended; new `useConnectionStatus()` (2 s grace, "restored" 2 s).
- UpdateBar: SW waits (registerType prompt); "Tai lai" posts SKIP_WAITING and reloads; with draft text (`.chat-editor-content` / textarea) label becomes "Tải lại sau khi gửi" and reload fires once the draft is empty.
- Manifest/theme colours as hex from tokens (#f1f8f7 light base, #0a1414 dark meta). Existing icons reused.
- nginx: exact `/sw.js` and `/manifest.webmanifest` no-cache (a 30d immutable regex would otherwise pin sw.js). No Service-Worker-Allowed needed (scope "/"). Traefik labels unchanged. `frontend/nginx.conf` (standalone Dockerfile, not prod) not touched.
- Docs: `docs/specs/pwa-install/spec.md`, README index, `docs/deployment.md` section; plan/phase marked completed.

## Gates
- vitest: 1301 pass, 1 fail (known AssetsScreen overview). New tests: OfflineBar (6), UpdateBar (4), hook grace (1); red first.
- lint clean; typecheck:diff no increase; build OK (precache 28 entries, 2343 KiB). nginx snippet `nginx -t` OK.
- Playwright (Chrome, built app served by scratch server with fake API): manifest served, 4 icons, no manifest errors; CDP installability errors only `in-incognito` (test context); SW activated, scope `/`, controls page; cache has 23 entries, 0 under /api; offline bar appears after ~2 s (matches mockup, screenshot checked); update bar appears when sw.js changes, "Tải lại" reloads onto it and bar is gone. Lighthouse not run (CDP installability used). Servers stopped.

## Concerns / follow-ups (outside my file ownership)
1. Cold start offline: `refreshAccessToken` (api/client.ts) calls `logout()` on a network error, so reopening the PWA offline shows the shell on /login and signs the person out locally. Should not log out on fetch failure; needs a decision on offline session handling.
2. Composer lock while offline: not done (components/chat off limits). Chat editor can use `useConnectionStatus() === 'lost'`.
3. Duplicate notices: ContactsScreen and DriveScreen still render their own "Đang kết nối lại…" (different wording) under the new bar; remove them when owners touch those files.
4. WS backoff in websocket.store is 1,2,4,8,16,30 s vs mockup 1,2,5,10,30 s.
5. Update-bar draft detection is DOM-based (`.chat-editor-content`, textarea); fragile if the composer class changes.
6. Real-device install (Android/iOS) belongs to phase 06.

Status: DONE_WITH_CONCERNS
Summary: PWA shell, manifest, SW, offline and update bars, nginx headers, docs and spec done; gates green except known AssetsScreen test; concern 1 (offline cold start signs user out) needs a decision.

## Review fixes (review: code-reviewer-261011-0925-pwa-phase-02.md)
Tests written first (13 red), then code. vitest 1384 pass, 1 fail (known AssetsScreen overview); lint clean; build OK (precache 28 entries). `typecheck:diff` reports new errors only in other agents' files (useChatSections.test.tsx, message-model.test.ts); none in my files.

- **H1 fixed.** "Để sau" clears the armed state; reload fires only while `armed && bar visible && nothing held`. Test: arm, dismiss, send the draft, no reload.
- **H2 verified** (client.ts fix is the coordinator's). Built app, offline cold start with a prior session: shell served, local user kept, bar "Mất kết nối…" shown. Because the token is still null, route guards land the person on `/workspace-select` (shell, own "không tải được" message), not the workspace. To show the bar there I mounted `<OfflineBar/>` in `routes/_auth.tsx` too (3 lines). Spec scenario and Status updated; phase checkbox left `[x]` (criterion now holds). Note: Playwright's offline mode leaves `navigator.onLine` true on cold load; I forced it false in the test page (a real offline phone reports false).
- **M1 fixed.** Reload waits for `useIsMutating()+useIsFetching()==0` as well as no draft, on both the armed path and a direct "Tải lại" tap (a tap while held arms instead). Uploads outside TanStack Query are not seen (listed in spec gaps).
- **M2 fixed, rule:** a field holds the reload only if the person typed in it (real `input` events: input, textarea, any contenteditable) and it still has text, so a pre-filled form never holds it forever; any edit in the last 3 s holds too (covers a document between keystroke and autosave); `.doc-prose` text is otherwise ignored since documents autosave and keep their `beforeunload` guard; removed fields are forgotten. A 500 ms poll while the bar shows catches programmatic clears. Tests: pre-filled, input, document window.
- **M3 fixed.** Bar uses `z-sticky` (above content, below backdrop/modal); when a composer (`.chat-editor-content`) is on screen it moves to the top (safe-area aware, `data-placement="top"`), else mockup position above the tab bar.
- **M4 fixed.** `applyUpdate` reloads directly when no worker is waiting (another tab already activated it).
- **M5 / L4 / L1 / L8.** Spec Status rewritten with real gaps, README line softened; deployment.md now says every static file gets the 30-day cache and names the fixed-name icons; both bars have a live region present from first render whose content swaps; grace test now also asserts false before 2 s.
- **Not done:** focus return after "Để sau" (Low); L5 `frontend/nginx.conf` (not my file; listed in spec gaps); L2/L3 composer lock and duplicate notices still open.

### Still open
- **H3** (duplicate sockets from "Thử lại"): untouched, needs `retryNow()` in `websocket.store.ts` (another agent is editing it); button still calls `connect(token)`.
- Composer lock offline, duplicate notices in Danh bạ/Tài liệu, backoff schedule vs mockup.

Status: DONE_WITH_CONCERNS
Summary: H1, H2 verification, M1-M5 and the Low items in scope are fixed test-first; H3 stays open pending the store owner.
