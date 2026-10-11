# Chat follow-ups (six items from wiring A/B)

Gates: vitest 2136 pass / 1 fail (AssetsScreen, known; earlier runs also hit auth.store / ChatEditor flakes under parallel load, both pass alone and in the last full run). `npm run lint` clean. `typecheck:diff` clean. `npm run build` ok. Messaging `go test ./...` 280 pass, `go vet` clean, `make check-layering` ok. No commits, git index untouched. `make proto` regenerated Go only (frontend proto:gen only reads ws.proto, unaffected).

## Per item
1. Local types dropped: `EditRequest` (useMessageLifecycle.ts), `TaskRequest` (TaskDialog.tsx). `useCreateTask` input now `Parameters<typeof messagingApi.createTask>[1]` (carries `parent_message_id`). `CreatePollInput.parent_message_id` in api/chat-cards.ts is a real field of the poll API (kept). Type-only, typecheck is the check.
2. Star toast "Xem" -> `useOpenStarred()` passed as `onView` in MessageMenu + MessageActionSheet. Tests: both open Đã gắn sao on Xem.
3. MobileNav Tin nhắn badge from `unreadByLevel(useUnreadCounts().data?.channels)`, same pill as Phê duyệt (99+, hidden at 0), aria-label "Tin nhắn, N chưa đọc". 3 tests (by level, 99+, muted-only = none). Sidebar had no test of the derivation: new `routes/workspace-layout.test.tsx` (2 tests: by level, zero).
4. `useFocusMessage` gets a 5th arg `{ settled, onMissing }`: toast once when everything that could hold the message has loaded (stream: first page + no older pages; panel: thread query) and it is still absent (400 ms grace), or after the 15 s give-up. SpaceView toasts info "Không tìm thấy tin nhắn này. Có thể tin đã bị xoá hoặc bạn không còn xem được." Conversation/thread stays open. Tests: hook (4) + SpaceView (stream missing, panel missing, found = quiet).
5. Push link: thread reply -> `?thread=<root>&message=<reply id>`; mention in main stream -> `?message=<id>`; other main messages unchanged. `notificationFor` takes the recipient kind. Tests updated (mention URL, reply URL). Spec web-push updated + scenario. Note: the "N tin mới" follow-up keeps the last message's link.
6. `GET /api/channels/unread` had no `starred_at`: added `google.protobuf.Timestamp starred_at = 8` to `ChannelUnread`; `store.StarredConversations` now returns `map[string]time.Time` (created_at). Domain test + REST test (present when starred, absent after unstar). Frontend: `ChannelUnread.starred_at`, `Conversation.starredAt`, `arrange()` sorts starred newest-first (no time = last), optimistic star stamps `starred_at` now. Spec chat-shortcuts updated + scenario. Tests: chat-sections, conversations, useShortcuts, channels-shortcuts route (order).

## Files
Backend
- backend/proto/messaging/messaging.proto, messaging.pb.go (generated)
- backend/services/messaging/internal/domain/push.go, push_test.go, stars.go, stars_test.go, reactions_pins_receipts.go
- backend/services/messaging/internal/store/stars.go
- backend/services/messaging/internal/rest/stars_test.go
Docs
- docs/specs/web-push/spec.md, docs/specs/chat-shortcuts/spec.md
Frontend (src)
- api/messaging.ts
- hooks/useChannelTasks.ts, useMessageLifecycle.ts, useFocusMessage.ts (+test), useConversations.ts, useShortcuts.ts (+test)
- components/chat/TaskDialog.tsx
- components/patterns/MobileNav.tsx (+test)
- components/spaces/MessageMenu.tsx (+test), MessageActionSheet.tsx (+test), SpaceView.tsx (+test)
- lib/conversations.ts (+test), lib/chat-sections.ts (+test)
- routes/channels-shortcuts.test.tsx
- routes/workspace-layout.test.tsx (new)

## Notes
- Other modified files in `git status` (ChatEditor, auth.store, drafts.store, websocket.store, lifecycle.go, quote_test.go, e2e, etc.) are not mine.
- Not run: Playwright screens; no schema change (created_at already existed), so no db-migrate.

Status: DONE
All six follow-ups are in with failing tests first (type-only item 1 verified by typecheck), and the gates pass except the known AssetsScreen failure. Backend `ChannelUnread` gained `starred_at` (Go proto regenerated; no frontend proto regen needed) and both specs are updated.
