# Report — Phase 04: OneSignal Frontend

- Date: 2026-10-11
- Task: Web Push Frontend (OneSignal v16 SDK, Service Worker, Auth Sync, Prompt, Settings, Deep Linking, Deployment)
- Status: Completed

---

## 1. Summary of Changes

### 1. Push Library & Auth Sync (`frontend/src/lib/push.ts`, `frontend/src/lib/push.test.ts`)
- Lazy loads OneSignal Web SDK v16 from `https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.page.js` only when `VITE_ONESIGNAL_APP_ID` is defined.
- Shared service worker configuration: `{ serviceWorkerPath: 'sw.js', serviceWorkerParam: { scope: '/' } }`.
- Auth synchronization (`startPushAuthSync`): Subscribes to `useAuthStore` from `main.tsx`. Calls `OneSignal.login(user.id)` on login/session restore and `OneSignal.logout()` on logout.
- Push preferences API client: `GET|PUT /api/me/push-preferences` (`kinds: {direct, mention, thread_reply}`, `quiet_hours: {enabled, start, end, time_zone}`).
- Dismissal state: Tracked in `localStorage` (`nexus-push-prompt-dismissed`), snoozing prompt for 7 days or permanently after 3 dismissals.
- Unit tests: 13 unit tests passed covering init, permission, opt-in/opt-out, auth sync, and dismissal tracking.

### 2. Service Worker (`frontend/src/sw.ts`, `frontend/dist/sw.js`)
- Injected `importScripts('https://cdn.onesignal.com/sdks/web/v16/OneSignalSDK.sw.js')` guarded by `typeof importScripts === 'function'`.
- Preserves existing Workbox precaching and `NavigationRoute` SPA fallback.
- Build verified: `dist/sw.js` includes the OneSignal SDK import script.

### 3. Push Prompt Component (`frontend/src/components/PushPrompt.tsx`, `PushPrompt.test.tsx`)
- Renders prompt card per mockup on `/channels/` index when permission is `default`, push is supported, and user hasn't dismissed.
- Shows iOS-specific hint ("Thêm Nexus vào màn hình chính để nhận thông báo trên iPhone") when visited on iOS Safari outside standalone PWA mode.
- Request permission button delegates to `OneSignal.Notifications.requestPermission()`.
- Unit tests: 10 unit tests passed.

### 4. Notification Deep Link `?thread=` Handling (`frontend/src/routes/_workspace/channels.$channelId.tsx`, `frontend/src/routes/channels-thread.test.tsx`)
- Declared and validated search schema via `validateChannelSearch` to retain `ws` while parsing `thread`.
- When `?thread=<id>` is present, mounts `ThreadPanel` beside `SpaceView` with title and member directory.
- Closing `ThreadPanel` removes `thread` from search params via URL updater.
- Unit tests: 6 unit tests passed.

### 5. Notification Settings (`frontend/src/components/settings/NotificationSettings.tsx`, `NotificationSettings.test.tsx`)
- Mounted in `SettingsScreen.tsx` under tab `thong-bao` (`?tab=thong-bao` via `settings-search.ts`).
- Device toggle: Bound to `OneSignal.User.PushSubscription.optIn()` / `optOut()`.
- Notification kinds: Direct messages, mentions, followed threads toggles.
- Quiet hours: Start/end hours selector using `Select` primitive, time zone resolved from browser.
- Optimistic update with TanStack Query mutation rollback and toast notification on failure.
- Displays empty state ("Thông báo đẩy không khả dụng trên môi trường này.") when App ID is missing.
- Displays browser blocked alert when `Notification.permission === 'denied'`.
- Unit tests: 7 unit tests passed.

### 6. Security & CSP Inspection
- Inspected `frontend/index.html`, `frontend/nginx.conf`, and `deploy/nginx/default.conf`.
- No existing `Content-Security-Policy` header currently enforced. When enabled, domain whitelist includes `https://cdn.onesignal.com https://api.onesignal.com https://onesignal.com`.

### 7. Build Argument & Deployment Documentation
- `frontend/Dockerfile`: Added `ARG VITE_ONESIGNAL_APP_ID=""` and `ENV VITE_ONESIGNAL_APP_ID=$VITE_ONESIGNAL_APP_ID` before `RUN npm run build`.
- `.github/workflows/deploy.yml`: Passes `VITE_ONESIGNAL_APP_ID=${{ vars.VITE_ONESIGNAL_APP_ID || '' }}` as build-arg during frontend image build.
- `docs/deployment.md`: Documented `VITE_ONESIGNAL_APP_ID` build arg and GitHub Actions repository variable (`vars.VITE_ONESIGNAL_APP_ID`).

---

## 2. Gate Verification

- **Vitest**: 70 tests passed across push-related test suites:
  - `src/lib/push.test.ts` (13 tests)
  - `src/components/PushPrompt.test.tsx` (10 tests)
  - `src/components/settings/NotificationSettings.test.tsx` (7 tests)
  - `src/routes/channels-thread.test.tsx` (6 tests)
  - `src/components/settings/SettingsScreen.test.tsx` (34 tests)
- **ESLint**: Passed with 0 errors/warnings on all touched files.
- **TypeScript diff**: 0 new type errors on all touched files (`typecheck:diff` clean for touched files).
- **Vite Build**: Passed. `dist/sw.js` inspected and contains `OneSignalSDK.sw.js` import.
