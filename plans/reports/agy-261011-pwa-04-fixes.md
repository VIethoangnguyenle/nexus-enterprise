# PWA phase 04 review-fixes report

- Date: 2026-10-11
- Plan: `plans/261010-2258-pwa-mobile-push/exec-04-review-fixes.md`
- Review: `plans/reports/code-reviewer-261011-1055-pwa-04.md`
- Executor: agy (Antigravity / Claude Opus 4.6)
- Mode: edit source, no commits, no git index changes.

## Fixes applied

| Issue | Status | What was done |
|---|---|---|
| **H1** | Fixed | `pwa-register.ts`: when OneSignal App ID is set, registration is no longer called via `registerSW`. Instead, `navigator.serviceWorker.ready` attaches to the existing registration and wires `updatefound` / waiting-worker detection so the "Có bản mới" update bar still works. Without App ID, `registerSW` is used as before. |
| **H2** | Fixed | `sw.ts` + `vite.config.js`: OneSignal SW script import is guarded by `__ONESIGNAL__` build-time constant (via Vite `define`). Without App ID → zero OneSignal code in `dist/sw.js`. The `importScripts` call is wrapped in `try/catch`. Build verified: `grep -c onesignal dist/sw.js` = 0 without App ID. |
| **H3** | Fixed | `channels.$channelId.tsx`: removed `ThreadPanel` rendering (and its imports: `useNavigate`, `usePeople`, `useConversations`, `useAuthStore`). `?thread=` remains validated in the route search schema. SpaceView doesn't accept `initialThreadId` yet (it's in `components/spaces/` which we can't edit), so the param is validated but unused — the chat lane will wire it. Test updated to verify no separate ThreadPanel renders. |
| **M1** | Fixed | `push.ts`: (1) `isLocalDev()` guard — init and auth-sync return null on localhost/127.0.0.1 unless `VITE_ONESIGNAL_DEV=1`. (2) OneSignalDeferred callback wrapped in `try/catch` → rejects instead of hanging. (3) 10s timeout on init promise. (4) `initPromise` cleared on failure → later call retries. `PushPrompt.tsx`: (5) `enabling` state disables "Bật thông báo" button, shows "Đang bật…" while waiting, distinct error message on failure. |
| **M2** | Fixed | `NotificationSettings.tsx`: `isError` from `useQuery` renders an error state with `AlertTriangle` icon + "Thử lại" button. Switches are NOT rendered, so defaults can never be shown as if they were the user's preferences. |
| **M3** | Fixed | `NotificationSettings.tsx`: (1) Device state starts as `null` (shows "Đang kiểm tra…", switch disabled). (2) Subscribes to `OneSignal.User.PushSubscription.addEventListener('change')` and `OneSignal.Notifications.addEventListener('permissionChange')` for reactive updates. (3) Subscribes to `navigator.permissions.query('notifications')` change events (fulfils "công tắc sẽ tự bật"). (4) Switch disabled on iOS not-installed. (5) Busy state while toggle is pending. |
| **M4** | Fixed | `NotificationSettings.tsx`: `updateQuietHours` always sets `time_zone: Intl.DateTimeFormat().resolvedOptions().timeZone` so the server applies the correct window regardless of stored value. |
| **M5** | Fixed | `NotificationSettings.tsx`: (1) `start === end` validation — inline error "Giờ bắt đầu và kết thúc không được trùng", request not sent. (2) All kind/quiet-hour switches `disabled` while `mutation.isPending` to serialize PUTs. |
| **L1** | Fixed | `NotificationSettings.tsx`: blocked and iOS banners changed from `role="alert"` to `role="status"`. |
| **L2** | Fixed | `PushPrompt.tsx`: copy changed from "việc cần bạn duyệt" → "trả lời trong thread" — backend only sends `direct`, `mention`, `thread_reply`. |

## Gates

| Gate | Result | Attribution |
|---|---|---|
| `npx vitest run` (touched files) | 41 passed / 0 failed | All push/pwa files. |
| `npx vitest run` (full suite) | 1610 passed / 2 failed | `ApprovalScreen` (template builder) and `AssetsScreen` (overview) — pre-existing, not ours. |
| `npx eslint` (touched files) | 0 errors | |
| `npx tsc --noEmit` | 0 errors in push files | 26 pre-existing errors in AttachmentGrid, SettingsScreen.test, ChatNavigator, generated proto, useAttachmentUploads.test. |
| `npm run build` | pass | `dist/sw.js` has **zero** OneSignal references when built without App ID. |

## Files changed

- `frontend/src/sw.ts` — H2: `__ONESIGNAL__` guard + try/catch
- `frontend/vite.config.js` — H2: `define: { __ONESIGNAL__: ... }`
- `frontend/src/lib/pwa-register.ts` — H1: OneSignal branch attaches to existing registration
- `frontend/src/lib/push.ts` — M1: isLocalDev, timeout, try/catch in deferred, clear on failure
- `frontend/src/lib/push.test.ts` — M1: init failure/retry test, no-script-without-appid test
- `frontend/src/components/PushPrompt.tsx` — M1: busy state; L2: copy fix
- `frontend/src/components/PushPrompt.test.tsx` — no change (tests still pass)
- `frontend/src/components/settings/NotificationSettings.tsx` — M2/M3/M4/M5/L1: error state, reactive device switch, timezone, validation, roles
- `frontend/src/components/settings/NotificationSettings.test.tsx` — M2/M3: error state test, loading state test
- `frontend/src/routes/_workspace/channels.$channelId.tsx` — H3: removed ThreadPanel
- `frontend/src/routes/channels-thread.test.tsx` — H3: rewritten for simplified route

## Decisions / NEEDS_CONTEXT

- **H3 `initialThreadId`**: SpaceView (in `components/spaces/`) doesn't accept it yet. The plan says "if SpaceView doesn't accept it yet, just leave the param validated and unused — the chat lane will wire it". That's what we did. Follow-up: add `initialThreadId` prop to SpaceView and seed `panel` state from it.
- **L2 mockup switches**: "Đề nghị cần bạn duyệt", "Công việc giao cho bạn", "Xem trước nội dung tin" from the mockup are correctly omitted (backend has only `direct`, `mention`, `thread_reply`). No action needed unless backend adds these notification kinds.
- **L3 concurrent syncUser**: Not addressed (low risk, theory-only race). Can be serialized with a promise chain in a follow-up.
- **L4 backend web_url missing ws**: Backend follow-up, not this diff.

Status: DONE
