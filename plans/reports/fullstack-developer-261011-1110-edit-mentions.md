# Edit adds @mentions: notify only the newly mentioned

## Behaviour
- `PATCH /api/messages/:id` accepts optional `mentions` (user ids). Absent/null = unchanged; `[]` = clear; list = replace (blank/duplicate ids dropped).
- Store (one tx, row locked): reads old mentions, replaces them, `added = new - old`; for a reply, added users follow the thread (shared `followMentioned`, also used by InsertMessage). Root: implicit follow comes from the stored array (test).
- Push: `pushNewMentions` async (`later`, shared with PushLater), recipients via `recipients(..., mentionsOnly=true)`: no DM/thread-follower/"all" audience. Same checks: read on OA, prefs, quiet hours, level (off passes group mention, silences DM), not viewing. Cap 20.
- gRPC EditMessage passes nil (unchanged); proto not touched (no `make proto` / proto:gen needed).

## Files
backend/services/messaging/internal/{store/lifecycle.go, store/store.go, domain/lifecycle.go, domain/push.go, rest/lifecycle.go, grpc/server.go}
tests: domain/edit_mentions_test.go (8: add/same/remove/nil/clear, deny no-read, pref off, level off group vs DM, edit w/o mentions in DM pushes nothing, viewing, cap 20, root follow, reply follow); existing callers got `, nil`.
docs/specs/chat-message-lifecycle/spec.md, docs/specs/web-push/spec.md updated.

## Gates
go vet ok; go test ./... in messaging ok; make check-layering, check-ngac ok; dev messaging restarted (8183).

## Frontend (not edited) - request shape
`PATCH /api/messages/{id}` body: `{"content": "...", "content_format": "...", "mentions": ["<user id>", ...]}`.
`mentions` = full list of user ids mentioned in the edited text (same ids the composer sends on create). MUST be sent
whenever the edit composer can change mentions; omitting it keeps old mentions (does not remove them). Send `[]` if all removed.

## Limitation
In-app unread-mention badge/feed derive from `messages.mentions` + `created_at` vs read position. A newly mentioned
person on a message older than their read position appears in the mentions feed but is not counted unread (would need a
mentioned_at column; not done - out of scope). Web push is the notification for that case.

## Unresolved
- Want edit-added mentions to bump the unread badge? Needs schema.
