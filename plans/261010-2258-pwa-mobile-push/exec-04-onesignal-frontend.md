---
title: "Execution plan — PWA phase 04 OneSignal frontend"
owner: Claude (planning, review) · executor: agy/Gemini
status: ready
---

# Execution plan — frontend only

Read: `plans/261010-2258-pwa-mobile-push/plan.md` (Quyết định), `phase-04-onesignal-frontend.md`, the APPROVED
mockup `design/mockups/push.html` (permission prompt card, Cài đặt → Thông báo screen, states), `docs/specs/web-push/spec.md`
(backend contract: `GET|PUT /api/me/push-preferences` with `{kinds:{direct,mention,thread_reply}, quiet_hours:{enabled,start,end,time_zone}}`),
`docs/specs/pwa-install/spec.md`, `frontend/src/sw.ts`, `frontend/src/lib/pwa.ts`, `frontend/src/lib/pwa-register.ts`.

## Decisions (fixed)
1. App ID from `import.meta.env.VITE_ONESIGNAL_APP_ID` (already set in `frontend/.env.local`, git-ignored; it is
   public). No App ID (tests, other devs) → every push UI is hidden and the SDK is never loaded.
2. ONE service worker: keep ours (`sw.ts`, scope `/`). Add at the top of `sw.ts`:
   `importScripts('https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.sw.js')` (guard so tests/build still pass).
   Init the page SDK with `serviceWorkerPath: 'sw.js'`, `serviceWorkerParam: { scope: '/' }` so OneSignal reuses it.
   Make sure our precache/navigation logic still works (the PWA tests must stay green).
3. Load the page SDK lazily (`https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.page.js`) from `lib/push.ts`
   only in production builds or when App ID is set; init once (`OneSignal.init({appId, ...})`).
4. Identity: after sign-in / session restore with a user → `OneSignal.login(user.id)`; on sign-out →
   `OneSignal.logout()`. Hook into the auth store (`stores/auth.store.ts`) via a small subscriber in `lib/push.ts`
   started from `main.tsx`; do not edit `api/client.ts`.
5. Permission only from a tap: `PushPrompt.tsx` card per mockup ("Nhận thông báo…"), shown on Trang chủ/Tin nhắn
   when permission is `default`, the browser supports push, and the person hasn't dismissed it (remember dismissal
   per device, try/catch localStorage). iOS Safari not installed to home screen → show the mockup's "add to Home
   Screen first" hint instead of the button. Button → `OneSignal.Notifications.requestPermission()`.
6. Click on a notification: the backend sends `web_url` = absolute app URL (`APP_BASE_URL + /channels/<id>?thread=<msg>`).
   OneSignal opens it; ensure our SW / app handles `?thread=` by opening the thread panel (check the existing
   channel route — if `?thread=` is not handled, add handling in the channel route file only, with a test).
7. Cài đặt → Thông báo (`components/settings/NotificationSettings.tsx`, mount it in the settings screen as the
   mockup shows — keep the edit to `SettingsScreen.tsx` to the minimum, another task edits its Workspace tab):
   device on/off (`OneSignal.User.PushSubscription.optIn/optOut`, state from `.optedIn`), kinds + quiet hours
   bound to `GET|PUT /api/me/push-preferences` via TanStack Query (optimistic, rollback, toasts per mockup).
8. CSP: if a Content-Security-Policy exists (check `frontend/index.html`, `deploy/nginx/default.conf`,
   `frontend/nginx.conf`), allow `https://cdn.onesignal.com https://api.onesignal.com https://onesignal.com`.
9. Production build: the Dockerfile/CI build must receive `VITE_ONESIGNAL_APP_ID` — find where VITE_* build args
   are passed (`frontend/Dockerfile`, `.github/workflows/*`, `deploy/`), add it as a build arg defaulting to empty,
   and document it in `docs/deployment.md` (value is public; set it as a repo variable, not a secret).

## Tests first (vitest; mock the OneSignal global)
- No App ID → no SDK load, no prompt, Thông báo screen shows the "not available" state from the mockup.
- Prompt: shows only when permission default + supported + not dismissed; tap → requestPermission; dismiss remembered.
- Auth: login(user.id) after sign-in; logout() on sign-out.
- Settings: toggling kinds sends PUT with the right body; quiet hours validation; rollback on error.
- `?thread=` link opens the thread panel (if you add handling).

## Gates
`cd frontend && npx vitest run` (only the known AssetsScreen failure), `npm run lint`, `npm run typecheck:diff`,
`npm run build` (and confirm `dist/sw.js` contains the OneSignal importScripts).

## Rules for the executor
- PROGRESS LOG (mandatory): after EVERY step append `<time> | step | files | result` to
  `plans/reports/agy-progress-pwa-04.log`; read it first when resuming and skip what is done.
- Do NOT edit: `backend/**`, `frontend/src/components/spaces/**`, `frontend/src/components/chat/**`,
  `frontend/src/stores/websocket.store.ts`, `frontend/src/api/messaging.ts`, `frontend/src/api/client.ts`,
  `components/patterns/AppSidebar.tsx`, `MobileNav.tsx`, `MobileMoreSheet.tsx` (other work in progress).
- Never print or commit secrets; the REST API key is NOT a frontend value and must never appear in frontend code.
- Do NOT commit; do NOT touch the git index. Don't invent UI outside the mockup — NEEDS_CONTEXT in the report.
- Report: `plans/reports/agy-<date>-pwa-04-onesignal.md`.
