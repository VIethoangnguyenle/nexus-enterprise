# Code review: PWA phase 02 (uncommitted)

## Scope
- Files: frontend/{package.json, package-lock.json, vite.config.js, index.html}, frontend/src/{sw.ts, main.tsx,
  lib/pwa.ts, lib/pwa-register.ts, routes/_workspace.tsx, hooks/useConnectionLost.ts(+test)},
  frontend/src/components/app/{OfflineBar,UpdateBar}.tsx(+tests), deploy/nginx/default.conf, docs/deployment.md,
  docs/specs/pwa-install/spec.md, docs/specs/README.md. Backend changes ignored (other tasks).
- Read-only review; git index untouched.

## Gates (run by reviewer)
| Gate | Result |
|---|---|
| `npx vitest run` | 1303 pass / 1 fail (known `AssetsScreen: overview`). First run showed 3 fails in 2 files; two re-runs clean except the known one — a load flake, not PWA tests. PWA tests x3: 77/77. |
| `npm run lint` | clean |
| `npm run typecheck:diff` | no file above baseline |
| `npm run build` | OK; `sw.js` 16 KB iife; precache 28 entries / 2343 KiB |

Precache manifest (read from built sw.js): index.html, manifest, icons, favicon.svg, apple-touch-icon, the one 2.1 MB entry
chunk, css, woff2, workbox-window + virtual_pwa-register chunks. No `/api`, no runtime caching route. Icons + manifest
appear twice (glob + plugin) with identical revisions — harmless for workbox.

## Verified OK
- SW caches only the shell; only routes are `precacheAndRoute` and a `NavigationRoute` with `denylist: [/^\/api\//]`
  (sw.ts:16,21). Non-navigation `/api` fetches and WS are never intercepted.
- Google OAuth: `/api/auth/google/{start,callback}` are navigations under `/api/` → denylisted → network. The redirect
  target `/auth/google/done` (backend google.go:33) is an SPA route; serving the shell for it is correct because the
  cookie was set on the network 302.
- No authenticated HTML is cached: index.html is static; user data never touches CacheStorage.
- Dev: `registerPwa` returns unless `import.meta.env.PROD` (pwa-register.ts:10); no `devOptions`; proxy table diff is
  only the plugin append — table intact.
- nginx: `location = /sw.js` / `= /manifest.webmanifest` beat the 30d regex (exact match wins); security headers
  repeated because `add_header` does not inherit. Hashed `assets/*` keep `immutable`. Prod mounts
  `deploy/nginx/default.conf` (docker-compose.prod.yml:517) and deploy.yml ships it.
- Update handshake: plugin `update(true)` posts `{type:'SKIP_WAITING'}`, sw.ts:24-26 handles it; no `clients.claim`,
  no auto skipWaiting — worker never activates on its own.
- No `Service-Worker-Allowed` needed (scope "/" at root) — implementer's reasoning is correct.

## Critical
None.

## High

### H1. "Để sau" after arming the update reloads the page under the draft
`frontend/src/components/app/UpdateBar.tsx:19,44-49`
Scenario: composer has text → user taps "Tải lại sau khi gửi" (`armed=true`, button disabled, no way to cancel) →
changes mind and taps "Để sau" → `dismissed=true` → `visible=false` → `useDraft(false)` runs `setDraft(false)` →
effect `[armed, draft]` sees `armed && !draft` → `applyUpdate()` → SKIP_WAITING + `window.location.reload()` with the
draft still in the composer. The draft is lost; this is precisely the "never reload mid-compose" guarantee in the spec,
broken by the button labelled "later".
Fix: on dismiss also `setArmed(false)`; and make the reload condition depend on the real draft state, not on a value
that is forced false when inactive, e.g. keep the observer running while `armed` (`useDraft(visible || armed)`), or
gate with `if (armed && visible && !draft)`. Add a test: arm with draft, click "Để sau", assert `apply` not called.

### H2. Offline cold start signs the user out (implementer concern 1) — confirmed, blocks the phase's own criterion
`frontend/src/api/client.ts:43-52` (pre-existing code, surfaced by this phase)
Scenario: installed app opened on a phone with no signal → shell served from precache → `bootstrapSession` →
`refreshAccessToken` → `fetch` throws `TypeError` → `catch { logout() }` → `queryClient.clear()`, persisted `user`
cleared → router lands on `/login`. When signal returns, `bootstrapSession` skips refresh because `user` is null
(client.ts:69-72) — the person must sign in again although their refresh cookie is still valid. Same path fires on a
502/503 from `/api/auth/refresh` during a deploy (`!res.ok` → logout), which an installed app that reopens right after a
release will hit. On `/login` there is no OfflineBar (it lives only in `_workspace.tsx:158`), so the person sees a
sign-in form that silently fails.
Consequences: the phase success criterion "mất mạng vẫn mở được khung app" is ticked `[x]` in
phase-02-pwa-foundation.md:37 but does not hold for the cold-start path, and the spec scenario "Offline navigation →
cached shell + connection bar" (spec.md) is violated while its Status says "Matches code".
Fix (needs product decision, small code): log out only on 401/403 (refresh token rejected); on network error or 5xx keep
`user`, finish bootstrapping with `accessToken=null`, mark the session "pending", and retry `refreshAccessToken` on the
`online` event / when the WS reconnects. Route guards must treat "user present, token pending" as signed in (render
shell + bar, queries disabled). Move `OfflineBar` to the root layout (or also render it on auth routes).

### H3. "Thử lại" can spawn parallel reconnect chains and orphan sockets
`frontend/src/components/app/OfflineBar.tsx:36` → `frontend/src/stores/websocket.store.ts:189-249`
`connect()` only short-circuits when the existing socket is `OPEN`; it ignores `CONNECTING` and never clears the pending
`reconnectTimer`. Scenario (flaky network, user taps Thử lại twice): tap 1 opens socket A (CONNECTING); tap 2 opens
socket B and `set({ws: B})`; A's handlers stay attached. If A opens it sends auth and runs `handleServerMessage` for
every envelope alongside B → duplicate event handling (unread counts, notifications, cache patches) for the life of the
session; A's `onclose` is ignored (`current !== ws`) so it is never cleaned up. Separately, tapping while a backoff
timer is pending leaves that timer running (variable overwritten, only the latest is cleared in `disconnect`), so two
backoff chains run. This is the first UI caller of `connect` (grep: only OfflineBar), so the hazard is new.
Fix: add a `retryNow()` action in the store: clear `reconnectTimer`, return if `ws` is OPEN or CONNECTING, else connect
with the current token from `useAuthStore` (not a token captured at render). Test: two rapid taps → one `WebSocket`
constructed.

## Medium

### M1. "Reload once the draft is empty" fires while the send is still in flight
`UpdateBar.tsx:47-49`; `ChatEditor.tsx:99-100` calls `onSend` then `clearContent()` synchronously; `SpaceView.tsx:142`
`send.mutate(...)`. The MutationObserver sees the empty editor immediately → SKIP_WAITING → reload while the POST is
in flight. If the request has not left, the message is lost; if it fails (5xx), the error toast never shows and the
text is already gone. Same for an in-progress drive upload (`handleFileUpload`) when "Tải lại" is pressed with no
text. Spec scenario says "reloads after the message is sent".
Fix: also require `queryClient.isMutating() === 0` (subscribe via `useIsMutating()`) before `applyUpdate`, both for the
armed path and for the direct "Tải lại" click.

### M2. Draft detection misses real unsaved work and is stuck by prefilled fields (implementer concern 5)
`UpdateBar.tsx:7-13`. Only `.chat-editor-content` and `textarea`. Misses `<input>` forms (approval/asset request
dialogs, titles) and the document editor (`.doc-prose`, DocumentEditor.tsx:43) — the document case is partly covered by
`useDocumentSave`'s `beforeunload` (useDocumentSave.ts:213-220), but iOS standalone PWAs do not honour `beforeunload`.
Conversely any textarea rendered with an existing value (settings/description fields) counts as a draft forever, so
"Tải lại sau khi gửi" never fires.
Fix: replace DOM sniffing with an explicit registry (`usePwaStore.holdReload(key)` / `release(key)`) that the composer
and form owners call; keep the DOM probe only as a fallback if desired.

### M3. Update bar covers the composer on phones
`UpdateBar.tsx:56` — `fixed bottom-tabbar mb-3 min-h-11 z-40`. MobileNav is always shown below 1024px
(MobileNav.tsx:50), and in a conversation the composer sits directly above it (`main.pb-tabbar`). The bar therefore
lands on the composer/send button exactly when the person is drafting — the state in which the bar persists longest.
`z-40` also equals `--z-modal` and comes later in DOM, so it overlays bottom sheets' action rows. Use the z token scale
(`z-toast`) and lift above the composer in conversation routes (or render it in the top region there), mirroring the
mockup §4 note that floating bars must not cover the composer.

### M4. Multi-tab: second tab's "Tải lại" can become a no-op
vite-plugin-pwa's `updateServiceWorker(true)` attaches the `controlling` listener then calls `messageSkipWaiting()`,
which targets `registration.waiting`. If tab A already activated the new worker, tab B still shows the bar
(`needRefresh` stays true), `waiting` is null, and `controlling` already fired → tapping does nothing. Fix:
`applyUpdate` should fall back to `location.reload()` when `registration.waiting` is null (verify against plugin
source; not read due to tooling block on node_modules). Desktop browser users with several tabs will hit this.

### M5. Spec status is inaccurate
`docs/specs/pwa-install/spec.md` "Status: Matches code" while the "Offline navigation" scenario fails on cold start (H2)
and the update requirement fails under H1. README index repeats "Matches code". Mark as "Partially matches" with the
gaps until H1/H2 land. Plan phase file marks success criterion `[x]` (phase-02:37) — report to lead, not editing.

## Low
- L1. `role="status"` regions are mounted already populated (OfflineBar.tsx:21,29; UpdateBar.tsx:55); VoiceOver/NVDA
  often skip live regions inserted with content. Keep a persistent empty `role="status"` container and swap its
  children. Focus handling is otherwise correct (bars do not steal focus); when "Để sau" unmounts the focused button,
  focus drops to `<body>` — return it to the previous element.
- L2. Composer not locked offline (concern 2) — mockup §5 "ô soạn khoá" unmet; spec lists it as a gap. Acceptable to
  defer, but the plan should own it.
- L3. Duplicate notices in Contacts/Drive (concern 3) and backoff 1,2,4,8,16,30 vs mockup 1,2,5,10,30 (concern 4):
  cosmetic; track.
- L4. `docs/deployment.md` says "everything else under `assets/` is hashed and cached for 30 days"; the regex also
  pins non-hashed root files (`/icon-192.png`, `/favicon.svg`, `/apple-touch-icon.png`) as `immutable` for 30 days.
  The SW precache revisions refresh them, but home-screen icons fetched by the OS will not update for 30 days.
- L5. `frontend/nginx.conf` (used by docker-compose `app` profile) gives `sw.js` `expires 30d; immutable`. Browsers
  bypass HTTP cache for the top-level SW script by default (`updateViaCache: 'imports'`), so impact is small; add the
  same exact-match block for parity.
- L6. Manifest declares only light `theme_color`; dark-mode installed app's splash/titlebar stays light. Mockup §4 note
  3 asks for theme-aware status bar — meta tags do it in-browser, manifest cannot; acceptable, note in spec.
- L7. Google Fonts stylesheet (index.html, pre-existing) is cross-origin and not precached; offline cold start renders
  Manrope fallback. Self-hosted fonts are precached. Acceptable.
- L8. Tests: `useConnectionLost grace` test only asserts the allow side (true at 2.1 s), not that it is still false
  before 2 s; OfflineBar "retries" test proves only that a mock was called. UpdateBar tests miss H1 and M1 cases.

## Edge cases scouted
- `/api/auth/google/*` navigations, `/auth/google/done` SPA route — OK.
- Old tabs after another tab activates the new worker: old precache entries are deleted on activate; the app is one
  entry chunk so no lazy-chunk 404s, only possible font-weight 404s (pre-existing class).
- Large first-visit precache (2.3 MB) on mobile data — note for phase 06 budget.

## Recommended actions (priority)
1. Fix H1 (one-line reset of `armed` + condition), add test.
2. Fix H3 with a store-level `retryNow()`; test single socket on double tap.
3. Decide H2 offline-session policy; implement non-401 refresh failures as "keep session, retry on online"; mount
   OfflineBar app-wide.
4. M1: gate reload on `useIsMutating() === 0`.
5. M2/M3: explicit draft registry; reposition bar in conversations; z token.
6. Correct spec/README status (M5) until 1-3 land.

## Plan follow-ups (for lead)
- phase-02 success criterion "mất mạng vẫn mở được khung app" should not be `[x]` until H2 is resolved.
- Composer lock (L2) and duplicate notices (L3) need an owner phase.

## Unresolved questions
- Product: is a cold start offline allowed to keep the person "signed in" with no token until the network returns?
- M4 depends on vite-plugin-pwa 2.0 `updateServiceWorker` internals; reviewer could not read node_modules (tool block).

Status: DONE_WITH_CONCERNS
Summary: Shell-only caching, dev isolation, nginx headers and OAuth paths are correct and all gates pass (only the known AssetsScreen failure); three High issues remain — "Để sau" after arming reloads under a draft, offline cold start signs the user out (phase criterion not met), and "Thử lại" can create duplicate/orphan WebSockets.
