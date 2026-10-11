# Chat review fixes (H1, H2, M1-M6, L1-L3, L6, L7, reviewer Q1)

## Gates
- frontend `npx vitest run`: 2163 pass, 1 fail (AssetsScreen:103, known). `npm run lint` clean. `npm run typecheck:diff` clean. Build not run (hook).
- messaging: `go test ./...` 280 pass, `go vet ./...` clean, gofmt clean. `make check-layering` ok.
- Tests written before code for each item.

## Done
- H1: `lib/session-end.ts` (registry) ; `auth.store.logout()` calls `endSession()`; drafts store and websocket store (lastMessages, lastReplies, typing, online, approval/recent changes) register resets. Other stores checked: ui/nav-collapse/drive hold ids/UI only, left.
- H2: ChatEditor clears draft + editor at send time; on failure restores only when the scope's draft is empty (into the live editor if same scope, else into the store); destroyed-editor guard. Tests incl. unmount mid-send, new text mid-send, conversation switch mid-send.
- M1: backend `domain.ErrQuoteUnavailable` + `ReasonQuotedMessageUnavailable`; REST `SendMessage` answers 400 `{"message","reason":"quoted_message_unavailable"}`; REST tests (deleted, unknown, other channel without leak, 201, other 400 without reason, 403 without reason); domain test. Client `QUOTED_MESSAGE_UNAVAILABLE`; marks quote deleted only on that reason. Spec updated.
- L1 (with M1): a send takes the quote off the strip at once (per-send claim), failure gives it back unless another was chosen.
- M2: star rollback = inverse item patch (message, thread, unread list, channel record); fetches in flight cancelled first (awaited only when something is fetching, so the optimistic patch stays synchronous); unstar failure invalidates the starred feed; conversation star invalidates unreadCounts when settled.
- M3: mention pill/feed invalidated on useMarkRead success, thread mark-read success, `unreadCount` frame, `threadState` frame, edits that add/remove me, deletes of a message that named me (or not held).
- M4: `mentions: m.mentions ?? cur.mentions` in `applyMessageEdited`.
- M5: `currentLevel` reads the unread list first; level query `staleTime 0` + `refetchOnMount: 'always'`; level query invalidated after a change settles. One existing test seeded inconsistent data (level query off, unread list main): fixed to seed both.
- M6: specs `chat-cards` (composer creation, thread placement, client differences) and `chat-message-lifecycle` (reason, quote draft, mark-unread hold, edit mention list, mention count refresh).
- Q1 / L6: `filterConversations('unread')` uses `unreadOf(c) > 0`. Test.
- L2: `quoteMessage(m, scope?)`; MessageMenu/MessageActionSheet take `composeScope`; ThreadPanel passes the thread scope. Tests.
- L3: ThreadPanel closes itself when the loaded root belongs to another channel. Test.
- L7: `cardKindOf`/`CardKind` -> `lib/message-model.ts`, `isImageAttachment` -> `lib/file-kind.ts`; importers updated; no lib/store imports from components for these.

## Files
New: `frontend/src/lib/session-end.ts`, `lib/session-end.test.ts`, `lib/file-kind.test.ts`, `hooks/useShortcuts.rollback.test.tsx`, `hooks/mentions-refresh.test.tsx`, `backend/services/messaging/internal/rest/quote_test.go`.
Frontend modified: `stores/auth.store.ts`(+test), `stores/drafts.store.ts`, `stores/websocket.store.ts`(+test), `components/chat/ChatEditor.tsx`(+test), `components/chat/ThreadPanel.tsx`(+test), `components/chat/MessageCard.tsx`, `components/chat/AttachmentGrid.tsx`, `components/spaces/MessageMenu.tsx`(+test), `components/spaces/MessageActionSheet.tsx`(+test), `components/spaces/ConversationStream.tsx`, `components/spaces/MessageBlock.tsx`, `hooks/useMessages.ts`(+test), `hooks/useShortcuts.ts`, `hooks/useMessaging.ts`, `hooks/useThreads.ts`, `hooks/useNotificationLevel.ts`(+test), `lib/errors.ts`, `lib/chat-cache.ts`(+test), `lib/conversations.ts`(+test), `lib/message-model.ts`(+test), `lib/message-actions.ts`, `lib/file-kind.ts`.
Backend modified: `internal/domain/errors.go`, `internal/domain/lifecycle.go`, `internal/domain/lifecycle_test.go`, `internal/rest/handler.go`, `internal/rest/engagement_test.go` (one route line in the test echo).
Docs: `docs/specs/chat-message-lifecycle/spec.md`, `docs/specs/chat-cards/spec.md`.
Note: `hooks/useShortcuts.ts` is shared with the other agent (Xem toast, starred_at stamp); I edited only `cancelFetches`, the snapshot/rollback blocks and the settle invalidation, via targeted replacements.

## ASK (files the other agent owns)
1. L5 `hooks/useShortcuts.ts` star toast (`useStarMessage`): the "Đã gắn sao tin" / "Đã bỏ gắn sao tin" toast is shown before the server answers. Move the `if (!input.quiet) toast(...)` block into the `try` after `await shortcutsApi...` (success only); the catch already shows the error toast.
2. L4 `hooks/useFocusMessage.ts` + `components/spaces/SpaceView.tsx` + `routes/_workspace/channels.$channelId.tsx`: add an `onDone?: () => void` argument to `useFocusMessage`, called when the flash ends (and on give-up); SpaceView passes a callback that runs `navigate({ to: '.', search: (s) => ({ ws: s.ws }), replace: true })`. Then the same link clicked again changes the search and re-triggers. Not done by me: those files are mid-edit by the other agent and the thread panel state must survive the cleared `thread` param.
3. L8 `hooks/useShortcutScreen.ts` `back()`: replace `history.length > 1` with an in-app flag (e.g. `history.state?.idx > 0` from TanStack Router's `router.history.state`, or a `cameFromApp` ref set by `useOpenStarred`/`useOpenMention` before navigating); fall back to the shortcuts home when absent.
4. L9 `hooks/useMessageLifecycle.ts`: for a message whose `content_format` is not `html` (written before rich text), do not send `mentions` in the edit request (omit it so the server keeps the stored list) and keep `m.mentions` in the optimistic patch; send `mentionsFromHtml(content)` only for html messages.

## Unresolved
- None blocking. Playwright 390px screenshots from M6's second bullet were not run (no browser here).

## ASK items done (after the other agent finished)
Gates: `npx vitest run` 2175 pass, 1 fail (AssetsScreen, known); `npm run lint` clean; `npm run typecheck:diff` clean; messaging `go vet` clean.
- L5: `hooks/useShortcuts.ts` — message and conversation star toasts now run after the server's answer (inside `try`), so a refusal shows only the error toast. Tests in `hooks/useShortcuts.rollback.test.tsx` (pending = no toast, stored = toast, refused = error only, conversation too).
- L4: `hooks/useFocusMessage.ts` gets a 6th arg `onDone` (called when the flash ends, or after the "not found" report); `components/spaces/SpaceView.tsx` passes a callback that replaces the URL with `search: { ws }` (drops `thread`/`message`) only when the link asked for something. Tests in `useFocusMessage.test.tsx` (3) and `SpaceView.test.tsx` (2). The open thread panel is SpaceView state, so it stays open after the param is dropped.
- L8: `hooks/useShortcutScreen.ts` — new `useBackOrHome()` using the router's `useCanGoBack` + `router.history.back()` (the app's own history, not `window.history.length`); `useShortcutScreen` uses it. New `hooks/useBackOrHome.test.tsx` (real memory router: goes back inside the app; first screen goes to Trang chủ even when `window.history.length` is 5). `test/router-mock.ts` gained `canGoBack`, `back`, `useCanGoBack`, `useRouter`; `routes/channels-shortcuts.test.tsx` adapted to them.
- L9: `hooks/useMessageLifecycle.ts` — `mentionsAfterEdit`: html messages send the mentions of the edited spans (unchanged); other formats keep their stored list and add any mention span picked during the edit (same in the optimistic patch). Two tests added in `useMessageLifecycle.test.tsx`. Removing a mention while editing a pre-rich-text message is not possible (no span to remove); accepted edge.
Files: `hooks/useShortcuts.ts`, `hooks/useShortcuts.rollback.test.tsx`, `hooks/useFocusMessage.ts`, `hooks/useFocusMessage.test.tsx`, `components/spaces/SpaceView.tsx`, `components/spaces/SpaceView.test.tsx`, `hooks/useShortcutScreen.ts`, `hooks/useBackOrHome.test.tsx`, `test/router-mock.ts`, `routes/channels-shortcuts.test.tsx`, `hooks/useMessageLifecycle.ts`, `hooks/useMessageLifecycle.test.tsx`.
The L9 and `onDone` tests were written in the same step as the code, so I did not observe them failing first; the L5, L4 hook and L8 tests were seen failing first (L8's test needed the new export, so it failed on import).

Status: DONE
All four ASK items are done with tests and the gates are green apart from the known AssetsScreen failure. The whole review list (H1, H2, M1–M6, Q1, L1–L9) is now addressed, except the 390px Playwright screenshots, which need a browser.

