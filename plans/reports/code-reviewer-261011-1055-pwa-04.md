# Code review: PWA phase 04, OneSignal web push frontend

- Date: 2026-10-11
- Plan: `plans/261010-2258-pwa-mobile-push/exec-04-onesignal-frontend.md`
- Executor report: `plans/reports/agy-261011-pwa-04-onesignal.md`
- Mode: read-only. No edits to source, no git index changes.

## Gates (run by the reviewer)

| Gate | Result | Attribution |
|---|---|---|
| `npx vitest run` | 3 failed / 1595 passed | AssetsScreen overview (known), AppSidebar switcher and auth-source (personal-workspace task). No failures in push files. |
| `npm run lint` | 19 errors | All in AttachmentGrid, AttachmentTray, ImageViewer, CreateWorkspaceScreen (other tasks). 0 in push files. |
| `npm run typecheck:diff` | fails | AttachmentGrid, ChatNavigator, useAttachmentUploads.test, useMessages (other tasks). 0 in push files. |
| `npm run build` | pass | `sw.js` is iife (classic worker) and contains `typeof importScripts==='function'&&importScripts('https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.sw.js')` ahead of `precacheAndRoute`. |

Secrets: no REST key, `os_v2_`, or `Basic` auth string anywhere in `frontend/src`, the Dockerfile, the workflow, or the docs. `frontend/.env.local` is ignored by `frontend/.gitignore:13` (`*.local`) and not tracked. No CSP exists (`index.html`, `nginx.conf`, `deploy/`), so there was nothing to change.

Deploy: `deploy.yml:118-119` passes `vars.VITE_ONESIGNAL_APP_ID` (a repo variable, not a secret) only to the `frontend` matrix entry. `Dockerfile` has `ARG VITE_ONESIGNAL_APP_ID=""` followed by `ENV`. Both are correct.

## Plan decisions

| # | Decision | Status |
|---|---|---|
| 1 | No App ID: no SDK, no push UI | Followed |
| 2 | One SW at scope `/`, guarded importScripts | Partly followed. The guard only checks `typeof importScripts`, so the OneSignal CDN script loads in every build, App ID or not, and nothing handles a failed load (H2). OneSignal and the PWA register different URLs for the same scope (H1). |
| 3 | Load the SDK lazily, init once | Followed. If init fails, though, the promise never settles (M1). |
| 4 | login/logout on auth changes | Followed (`push.ts:127-167`, started from `main.tsx:35`) |
| 5 | Permission only from a tap, iOS hint, per-device dismissal | Followed |
| 6 | `?thread=` opens the thread panel | Implemented as a second ThreadPanel outside SpaceView (H3) |
| 7 | Settings: device switch plus kinds and quiet hours with optimistic update and rollback | Followed, with the gaps in M2 to M5 |
| 8 | CSP | No CSP exists; nothing to change |
| 9 | Build arg, repo variable, docs | Followed |

## Critical

None.

## High

**H1. OneSignal and `registerPwa` re-register the same scope with different script URLs on every load.**
- Where: `push.ts:85-89,112-116` (`serviceWorkerPath: 'sw.js'`) together with `pwa-register.ts:13` (`registerSW`, which registers `/sw.js`).
- Evidence: in the live OneSignal v16 page SDK, the worker URL is built as `${new URL(path)}?appId=<id>&sdkVersion=<v>`. The SDK re-registers whenever `active.scriptURL !== that URL` (it logs "SW href changing").
- What happens in production with an App ID set:
  1. `registerSW` registers `/sw.js`.
  2. OneSignal init finds the URL without its query string and registers `/sw.js?appId=…`.
  3. Next load, workbox registers `/sw.js` again.
- A different script URL for the same scope starts an update job each time. With `registerType: 'prompt'` this can show the "new version" banner on most loads, reinstall the precache, and leave the active worker flipping between the two URLs.
- Not exercised by any test. The unit tests mock `window.OneSignal`, and `make dev` registers no SW.
- Fix: one owner for registration. When an App ID is set, either let OneSignal register and point workbox-window at the existing registration (`getRegistration()` instead of `register`), or have `registerPwa` register the exact OneSignal URL. Check in `vite preview` with an App ID: `navigator.serviceWorker.getRegistration().then(r => r.active.scriptURL)` across three reloads, and confirm no update banner appears.

**H2. A failed `importScripts` stops the whole service worker from installing, so the offline shell is lost.**
- Where: `sw.ts:4-7`.
- How it fails: `importScripts` throws during top-level evaluation if `cdn.onesignal.com` is unreachable (corporate proxy, DNS filter, CDN outage, captive portal on first install). The worker then fails to install, so there is no precache, no offline fallback and no update.
- It also runs in builds with no App ID (other developers, CI preview, forks). That pulls a third-party script into the worker for everyone, even though the plan says that without an App ID the SDK is never loaded.
- Fix:

  ```ts
  if (import.meta.env.VITE_ONESIGNAL_APP_ID) {
    try {
      importScripts('https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.sw.js')
    } catch (e) {
      console.warn('[sw] push unavailable', e)
    }
  }
  ```

  injectManifest builds `sw.ts` through Vite, so the env value is available there.

**H3. The `?thread=` deep link adds a second thread panel outside SpaceView's single-panel model.**
- Where: `channels.$channelId.tsx:49-61`.
- SpaceView owns `panel` state, which allows one side panel at a time. It closes that panel on Esc (`SpaceView.tsx:133-142`) and renders it inside `AnimatePresence` (`:403-415`). The route's ThreadPanel is a sibling outside all of that.
- Scenarios:
  1. Open the notification link, then tap "Thread" or a reply count in SpaceView. Two panels render side by side; on a phone, two stacked full-screen sheets.
  2. Esc closes only SpaceView's panel. The deep-linked one stays open.
  3. `spaceName` comes from `conv?.title ?? ''`, so the header is blank until conversations load, or permanently when the notification link has no `ws` and the active workspace is a different one.
  4. No exit animation.
- The test (`channels-thread.test.tsx`) mocks both SpaceView and ThreadPanel, so none of this can show up there.
- Fix: give SpaceView an `initialThreadId` (or `thread` and `onThreadChange`) prop and seed `panel` from it. This touches `components/spaces/**`, which the executor was told not to edit, so it is the lead's call to schedule.

## Medium

**M1. OneSignal init can hang forever, or fail permanently after one bad load.**
- Where: `push.ts:109-119`.
- If `OneSignal.init` throws inside the deferred callback, `resolve` is never called and `initPromise` stays pending forever. Init does throw on a domain mismatch, which is what happens on `localhost` with the `.env.local` App ID, since `allowLocalhostAsSecureOrigin` is not set.
- If the script fails to load (ad blockers block `cdn.onesignal.com` often), the rejected `initPromise` is cached, so every later call fails until a full reload.
- Effects:
  - "Bật thông báo" (`PushPrompt.tsx:32-45`) waits forever with no busy state, and repeated taps queue more calls.
  - The settings device switch never learns its real state.
- Fix:
  - Wrap init in try/catch and reject on error.
  - Clear `initPromise` on failure.
  - Add a timeout.
  - Add busy and disabled state to the Enable button.
  - Gate auth sync and init on `import.meta.env.PROD` as well, as `registerPwa` does, so `make dev` does not load OneSignal and register `/sw.js` against the Vite server.

**M2. If the preferences GET fails, defaults appear as if they were the user's settings, and the next toggle overwrites what is stored.**
- Where: `NotificationSettings.tsx:47-51,95-115`.
- On a GET error, `data` is undefined, so `DEFAULT_PREFERENCES` renders. `isLoading` is false, so the switches are enabled.
- Toggling one kind then PUTs the full default object, which silently wipes the stored quiet hours and other kinds.
- Fix: on `isError`, render an error state with retry and disable the switches.

**M3. The device switch is not reactive and can show the wrong state.**
- Where: `NotificationSettings.tsx:37-45,38-39`.
- The initial state is `true`, so the switch says "Đang bật" before the SDK answers, and keeps saying it forever if the SDK never loads (see M1).
- `permission` is read once per render, with no `permissionChange` or `PushSubscription` `change` listener. The blocked banner promises "Quay lại đây, công tắc sẽ tự bật", but that does not happen until something else causes a re-render.
- On iOS when not installed, the switch is not disabled, and `optIn` will fail.
- Fix:
  - Start with an unknown/loading state.
  - Subscribe to `OneSignal.Notifications.addEventListener('permissionChange')` and to `User.PushSubscription.addEventListener('change')`.
  - Disable the switch when `isIos` is true.

**M4. Quiet hours are always sent in the server's default time zone, not the person's.**
- Where: `NotificationSettings.tsx:106-115`.
- The browser time zone appears only in `DEFAULT_PREFERENCES`. The server returns `Asia/Ho_Chi_Minh` when nothing is stored (`messaging/internal/store/push.go:22`), and the PUT echoes that value back.
- The spec (`web-push/spec.md:17`) says quiet hours apply "in their time zone", so anyone outside UTC+7 gets the wrong window.
- Fix: set `time_zone: Intl.DateTimeFormat().resolvedOptions().timeZone` whenever quiet hours are enabled or edited.

**M5. Quiet-hours validation the plan asked for is missing, and rapid toggles can race.**
- `start === end` is selectable; the server refuses it with a 400, and the user sees only a generic toast. The plan listed "quiet hours validation" as a test, and none exists.
- A stored non-hour value such as `22:30` shows as `00:00` in the Select.
- Two quick toggles send two concurrent PUTs. If the first fails, rollback restores its snapshot and erases the second toggle in the UI.
- Fix: disable the switches while `mutation.isPending`, or serialize the PUTs.

## Low

- **L1.** `NotificationSettings.tsx:131,146`: the blocked and iOS banners use `role="alert"`, so they are announced on every visit to the tab. A plain region or `role="status"` fits better.
- **L2.** Mockup parity (`design/mockups/push.html`): the mockup also shows "Đề nghị cần bạn duyệt", "Công việc giao cho bạn" and "Xem trước nội dung tin". These were correctly omitted because the backend contract has only `direct`, `mention` and `thread_reply`, but the executor did not report them as NEEDS_CONTEXT. The PushPrompt copy ("việc cần bạn duyệt sẽ báo…", `PushPrompt.tsx:63`) still promises approval pushes the backend does not send.
- **L3.** `push.ts:135-149`: concurrent `syncUser` calls are not serialized. For example, a fast sign-out followed by sign-in as a different user could, in theory, finish out of order inside OneSignal. Chaining the calls on a single promise would remove the risk.
- **L4.** The backend `web_url` (`messaging/internal/domain/push.go:294`) has no `ws`, so a channel from a non-active workspace opens with the wrong people and conversation list. This is a backend follow-up, not this diff.
- **L5.** Tests:
  - `push.test.ts` checks that init is not called without an App ID, but not that no `<script>` is injected.
  - There are no tests for script-load failure or init rejection.
  - `channels-thread.test.tsx` mocks every collaborator, so it proves routing only, not the panel UX.

## Positive observations (risk calibration)

- Only the frontend matrix entry gets the build arg, and it comes from a repo variable.
- The REST API key never appears in frontend code.
- Edits to `SettingsScreen.tsx` and `settings-search.ts` are minimal (3 lines and 3 lines).
- The SW still precaches only the app shell. `/api` is not cached.

## Files that belong to this task (for the commit)

- `frontend/src/lib/push.ts` (new)
- `frontend/src/lib/push.test.ts` (new)
- `frontend/src/sw.ts`
- `frontend/src/main.tsx` (whole diff, 2 lines)
- `frontend/src/components/PushPrompt.tsx` (new)
- `frontend/src/components/PushPrompt.test.tsx` (new)
- `frontend/src/components/settings/NotificationSettings.tsx` (new)
- `frontend/src/components/settings/NotificationSettings.test.tsx` (new)
- `frontend/src/components/settings/SettingsScreen.tsx` (whole diff, 3 lines)
- `frontend/src/components/settings/SettingsScreen.test.tsx` (whole diff, 1 line: the tab list)
- `frontend/src/lib/settings-search.ts` (whole diff)
- `frontend/src/routes/_workspace/channels.index.tsx`
- `frontend/src/routes/_workspace/channels.$channelId.tsx`
- `frontend/src/routes/channels-thread.test.tsx` (new)
- `frontend/Dockerfile`
- `.github/workflows/deploy.yml` (whole diff, build-args hunk only)
- `docs/deployment.md`: **partial, hunk-level staging required.** Only the hunk adding "For the frontend, `VITE_ONESIGNAL_APP_ID` is baked into…" under "Web push (OneSignal)" belongs here. The gotenberg network bullet, the memory-limit line and the "File previews (Gotenberg)" section belong to the file-preview task.
- `plans/261010-2258-pwa-mobile-push/exec-04-onesignal-frontend.md` (new)
- `plans/reports/agy-261011-pwa-04-onesignal.md` (new)
- This report

Do not include `frontend/.env.local` (ignored), anything under `components/chat/`, `spaces/` or `patterns/`, `auth-fixtures.ts`, or the drive/backend files.

## Recommended actions (in order)

1. Fix H2: env guard plus try/catch around `importScripts`.
2. Fix H1, then verify the worker registration URL in `vite preview` with an App ID across reloads.
3. Fix M1 (init error and timeout handling, PROD gate, busy state on the button) and M2 (error state on the preferences GET).
4. Decide on H3: either add a SpaceView prop for the deep-linked thread (touches `components/spaces/`), or accept the duplicate-panel behaviour for now.
5. Fix M3, M4 and M5. Add tests for SDK load failure, the GET error, and `start === end`.

## Unresolved questions

- Should `docs/specs/web-push/spec.md` gain frontend requirements (prompt rules, the `?thread=` deep link, settings), as CLAUDE.md asks for behaviour changes? It currently describes the backend contract only.
- Are the mockup's approval, task and preview switches deferred to a later phase, or should they be dropped from the mockup?

Status: DONE_WITH_CONCERNS
Summary: The plan was mostly followed and the gates are clean for the push files, with no secrets and the App ID in a repo variable. Three high issues must be resolved before shipping: OneSignal and workbox fight over the service worker URL on every load, a failed `importScripts` from the CDN prevents the worker (and offline) from installing, and the `?thread=` link opens a second thread panel outside SpaceView.
