---
title: "Execution plan — PWA phase 04 review fixes"
owner: Claude (decisions) · executor: agy/Gemini
status: ready
---

# Fix the review of PWA phase 04 (`plans/reports/code-reviewer-261011-1055-pwa-04.md` — read it first)

## Decisions (fixed)
1. **H1 — one owner for service-worker registration.** When `VITE_ONESIGNAL_APP_ID` is set, OneSignal owns the
   registration (it registers `/sw.js?appId=…&sdkVersion=…`); `lib/pwa-register.ts` must NOT call
   `navigator.serviceWorker.register` itself in that case. Instead the PWA update flow attaches to the existing
   registration (`navigator.serviceWorker.getRegistration('/')` / `ready`) and listens for `updatefound` /
   waiting worker exactly as today, so "Có bản mới" still works. Without an App ID, today's registration stays.
   Add tests for both branches (mock navigator.serviceWorker). Manual check: `npm run build && npx vite preview`
   with the App ID set, reload 3 times, confirm (Playwright: `navigator.serviceWorker.getRegistration()` →
   `active.scriptURL`) the URL is stable and the update bar never appears; stop the preview server after.
2. **H2 — OneSignal must never break the offline shell.** In `sw.ts`, import the OneSignal SW script only when
   the build had an App ID (inject a build-time constant via Vite `define`, e.g. `__ONESIGNAL__`), and wrap
   `importScripts` in try/catch (log once). Test: build without App ID → `dist/sw.js` has no OneSignal URL.
3. **H3 — no second thread panel.** Remove the thread panel rendering added to
   `routes/_workspace/channels.$channelId.tsx` and its test; keep `?thread=` in the route's search schema (validated)
   and pass it to `SpaceView` as a prop `initialThreadId` WITHOUT editing SpaceView (if SpaceView doesn't accept it
   yet, just leave the param validated and unused — the chat lane will wire it). Record this in the report.
4. **M1** — `lib/push.ts`: init has a timeout (e.g. 10 s) and a failed/blocked SDK load resets so a later tap
   can retry; "Bật thông báo" shows a busy state while waiting and a clear error after; never load the SDK on
   `localhost`/`127.0.0.1` unless `VITE_ONESIGNAL_DEV=1` (OneSignal web push needs the registered origin).
5. **M2** — NotificationSettings: if loading preferences fails, show the error state from the mockup with Thử lại
   and disable all controls (never show defaults as if they were the person's).
6. **M3** — device switch reflects the SDK's real `optedIn` (subscribe to its change event and to
   `Notification.permission` changes via the permissions API where available); busy while pending; disabled with
   the iOS hint when on iOS and not installed.
7. **M4** — quiet hours use the browser's `Intl.DateTimeFormat().resolvedOptions().timeZone` when saving, and show
   the stored zone when it differs.
8. **M5** — validate start ≠ end (inline error, no request); serialise PUTs (latest wins; rollback only the change
   that failed).
9. Low items: fix banner roles/a11y as the report says; hide settings the backend does not support yet.

## Files
`frontend/src/lib/{push,pwa,pwa-register}.ts(+tests)`, `src/sw.ts`, `vite.config.js` (define only),
`src/components/PushPrompt.tsx`, `src/components/settings/NotificationSettings.tsx(+test)`,
`src/routes/_workspace/channels.$channelId.tsx`, `src/routes/channels-thread.test.tsx` (remove/adjust).
Do NOT edit components/spaces/**, components/chat/**, components/patterns/**, components/auth/**, backend/**.

## Gates
vitest on the touched files + full `npx vitest run` (report failures that are not yours separately),
`npx eslint` on touched files, `npm run typecheck:diff`, `npm run build`.

## Rules
PROGRESS LOG after EVERY step: `plans/reports/agy-progress-pwa-04-fixes.log`; read it first when resuming.
No commits, no git index. Report: `plans/reports/agy-<date>-pwa-04-fixes.md`.
