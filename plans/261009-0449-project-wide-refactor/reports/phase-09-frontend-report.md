# Phase 09 (frontend half): tests, dead code, large files

Date 2026-10-10, branch `refactor/project-wide`, scope `frontend/**`. Nothing staged or committed.

The plan's Key insights (7 test files, 13 untested hooks, `ChannelDrivePanel`, `DrivePreviewDialog`, `InviteMemberForm`, `CreateChannelModal`) pre-date phase 06. I re-surveyed first: `ChannelDrivePanel`, `DrivePreviewDialog`, `InviteMemberForm`, `CreateChannelModal` were already gone, and the baseline had shrunk by itself because phase 06 rebuilt the screens. Only what was still true was acted on.

## Gates (all exit 0)

- `npm run lint`: no issues
- `npm run typecheck:diff`: no file worse than baseline
- `npm test`: 86 files / 1146 tests (was 80 / 1064 at the last report; 78 / ~1100 when I started, plus 6 new files)
- `npm run build`: ok

Bundle (`vite build`, HEAD tree vs working tree, measured by building both):

| | before | after |
| --- | --- | --- |
| JS | 2,073.97 kB (gzip 569.85) | 2,073.85 kB (gzip 570.16) |
| CSS | 70.69 kB (gzip 13.70) | 65.63 kB (gzip 12.78) |

JS is flat: the deleted modules were unreachable and already tree-shaken. CSS shrinks 5 kB because Tailwind scans source files and the deleted ones contributed classes. The 500 kB chunk warning remains (single chunk, pre-existing; code-splitting is out of scope here).

## Dead code deleted

Tool: `npx --yes knip` (runs from the npx cache, nothing added to package.json). Every file was then re-checked with grep for importers (none outside the deleted set, tests included; route tree does not reference any of them).

- `src/api/notifications.ts`, `src/hooks/useNotifications.ts`, `src/components/NotificationBell.tsx`: bell chain, nothing mounted it
- `src/components/composites/{Breadcrumbs,Card,DataTable,FilterBar,Modal,PeekPanel,ResponsiveDetailPanel,Tabs,Timeline}.tsx` + `composites/index.ts`: barrel had no importer; consumers import `Dialog`, `ConfirmDialog`, `AlertBanner`, `TreeView` by path (kept)
- `src/components/patterns/TreeView.tsx`, `patterns/index.ts`: duplicate of `composites/TreeView`, no importer
- `src/components/chat/index.ts`: barrel, no importer
- `src/components/EmptyState.tsx`, `LoadingState.tsx`: superseded by `spaces/EmptyState`
- `src/components/layouts/AuthLayout.tsx` (directory removed)
- `src/lib/constants.ts`
- Legacy primitives `Input` (+test), `NavRow` (+test), `Badge`; their `primitives/index.ts` exports removed. `TextField`, `Textarea`, `Select` are still used and stay.
- Unused hooks in `useMessaging.ts`: `useReactions`, `usePoll`, `useCreatePoll`, `useVotePoll` (zero references; `messagingApi` methods and query keys kept, the websocket store writes the poll cache)
- `byNewest` in `drive/drive-model.ts` (zero references)

Left alone on purpose: knip's "unused exports" list (about 67) is mostly `export`s used inside their own file, test fixtures and generated proto types. Removing `export` keywords is churn, not dead code. CSS: no unused-rule tool was run (Tailwind purges by scan); no hand-removal was attempted.

## Dependencies removed

`@tiptap/extension-mention`, `@tanstack/react-virtual`, `@tanstack/react-router-devtools`: zero imports in `src`, `index.html`, configs. `package.json` and lockfile updated by `npm uninstall`. Kept: `@protobuf-ts/plugin` (knip flags it, but `proto:gen` uses its `protoc-gen-ts`) and `@tanstack/react-query-devtools` (used in `main.tsx`). None added.

## Tests added (6 new files, 1 extended; 82 tests)

- `stores/auth.store.test.ts` (extended): `tenantIdFromToken` (url-safe base64, malformed), login/setAccessToken derive tenant, `isAuthenticated` from persisted user, access token never persisted
- `stores/ui.store.test.ts`: module switch reopens panel, toggle, width set/reset, star toggle, persist partialize
- `stores/drive.store.test.ts` already covered; unchanged
- `lib/errors.test.ts`: status/reason extraction, `isForbidden`, each `explain` branch, no server text leaks
- `lib/people.test.ts`: directory indexes, role join, unknown-person fallback (never an id), `displayName`, accent-insensitive search
- `hooks/useArrivals.test.ts`: history vs arrival, mine vs other, wash expiry, burst coalescing, scope reset (fake timers)
- `hooks/useConnectionLost.test.ts`: 3 s grace, recovery, offline event, never-connected is not lost
- `hooks/useResizable.test.ts`: initial size, drag delta, clamp, mouseup/cursor cleanup, axis, double-click reset
- `lib/chat-cache.test.ts`: reaction add/dedupe/remove/unloaded, poll vote, preview/timestamp/convert helpers (locks the code moved out of the websocket store)

`lib/preferences` and the websocket reducer were already covered.

## Splits (behaviour unchanged, existing suites green after each)

| File | before | after | Extracted |
| --- | --- | --- | --- |
| `hooks/useMessaging.ts` | 534 (479 after dead removal) | 143 | `hooks/useMessages.ts` (messages, thread, reply, reactions, pins; 246) and `hooks/useChannelTasks.ts` (98); importers in `components/spaces/*` and `chat/ThreadPanel` repointed |
| `drive/DriveScreen.tsx` | 563 | 381 | `useDriveActions.ts` (dialog state, mutations, uploads), `DriveBody.tsx` (empty/error/missing states), `DriveBurstBanner.tsx`, `DeleteItemDialog.tsx` |
| `assets/AssetsScreen.tsx` | 554 | 434 | `useRequestDecisions.ts` (approve/assign/reject flow), `AssetListBody.tsx` (list tab) |
| `stores/websocket.store.ts` | 775 | 655 | `lib/chat-cache.ts` (reaction, pin, poll, task cache patches and message helpers) |

Not split further: `websocket.store.ts` remaining bulk is one `handleServerMessage` switch sharing module-level timers and sequence state; moving it needs a shared-state module and is a behaviour risk for little gain. `ApprovalScreen.tsx` (442) and `SpaceView.tsx` (400) are at the soft limit and were left. `AssetsScreen` stays above 400 for the same reason (requests and types tab bodies are small and tightly bound to `go`).

## README

`frontend/README.md` replaced: scripts table, dev proxy (non-flat `/api/workspaces` dispatch, `/api/messages` ordering, ws on :8081, rule for new routes), generated code (`routeTree.gen.ts`, `src/generated/`), design source `../DESIGN.md` + mockups, test/lint/typecheck:diff, short layout map.

## Baseline delta

`typecheck-baseline.txt`: 23 entries / 66 errors to 2 entries / 4 errors. The screens rebuilt in phase 06 had already cleared most files; this round fixed the rest: the 5 errors in `useMessaging.ts` and 7 in `websocket.store.ts` (reaction groups spread from a possibly-undefined index, fixed with a guarded `const current = reactions[idx]`; covered by `chat-cache.test.ts`), and `global` in `api/messaging.test.ts` (to `globalThis`). The remaining 4 are in `src/generated/` (never hand-edited).

## Visual smoke

Playwright from the gstack install (headless `/usr/bin/google-chrome`), Vite on :5173 started by me with `--strictPort`, `/api/*` answered in-page by `page.route`, no DB. Screens: Tài liệu (`/drive`, plus `?folder=`), Tài sản (overview and `?section=list`), Tin nhắn (`/channels/<id>`). All rendered with names and no ids, zero console or page errors, layout consistent with phase 06 shots. Vite stopped (pid checked, :5173 free). The three pre-existing Vite servers of the user were not touched. Scripts and screenshots were kept in the session scratchpad only.

## Notes

- Build prints a TanStack warning that `routes/*-redirects.test.ts` do not export a Route (pre-existing; harmless; renaming with the `-` prefix would fix it).
- I swapped `src` with the HEAD copy briefly to measure the "before" bundle, then restored it and diffed it identical; the git index was never touched.

## Review fixes

1. 413 quota (HIGH). Tests first: `errors.test.ts` (413 gives the "kho tài liệu đã đầy" sentence, not the network one) failed red, then added `QUOTA_EXCEEDED = 'quota_exceeded'` and the 413 branch in `explain()` before the fallback. The upload path does not bypass `explain()`: `useUploadFile` runs through the shared `mutationCache.onError`, which calls `explain(error, 'tải tệp lên')`. A component-level test in `DriveScreen.test.tsx` (createFile rejected with `ApiError` 413 `quota_exceeded`) asserts the toast says the storage is full and does not mention the network.
2. Stale guidance. `eslint.config.js` raw-tag message now points to Button/IconButton/TextField/Select/Textarea/Pressable. `docs/specs/lark-sidebar-layout/spec.md` now says sidebar items are router `Link`s styled in `AppSidebar.tsx` and that `NavRow` was removed (the sidebar never used a nav primitive after the redesign).
3. Dead realtime cache writes. Notification handling kept in `websocket.store.ts` (`notification`) and `lib/realtime.ts` (`keys.notifications.all()`), each with a comment that the keys are kept for the pending notifications UI. Polls: no component, hook or route reads `keys.messaging.poll` (grep over `src`; `useVotePoll`/`usePoll` were already removed), so I removed `applyPollVote`, `keys.messaging.poll`/`pollsAll`, the `pollsAll()` resync entry, the poll test in `websocket.store.test.ts` and the `applyPollVote` tests in `chat-cache.test.ts`. The `pollVote` envelope case stays as a commented no-op. `messagingApi` poll methods are untouched.

Gates after fixes: lint clean, `typecheck:diff` no regression, `npm test` 86 files / 1145 tests, build ok. No git index change.

Status: DONE_WITH_CONCERNS

Summary: Dead code, 3 unused deps and legacy primitives removed; 82 tests added across stores/lib/hooks; useMessaging, DriveScreen, AssetsScreen and the websocket store split along real boundaries; README rewritten; typecheck baseline shrunk to the 4 generated-file errors. Lint, typecheck:diff, 1146 tests and build are green, smoke clean.

Concerns:
- `websocket.store.ts` (655), `AssetsScreen.tsx` (434), `ApprovalScreen.tsx` (442) still exceed ~400; further splitting judged riskier than useful (see Splits).
- JS bundle is unchanged (2.07 MB single chunk, build warns); the dead code was already tree-shaken. Route-level code-splitting is a separate task.
- knip's ~67 unused-export hits were deliberately not touched.
