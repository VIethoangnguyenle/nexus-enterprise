# Cards in thread (poll / task as thread reply)

## Request shape
- `POST /api/channels/{ch}/polls` body: existing fields + optional `"parent_message_id": "<root message id>"`
- `POST /api/channels/{ch}/tasks` body: existing fields + optional `"parent_message_id": "<root message id>"`
- Response unchanged (201 poll / task, `message_id` is the reply's id). Absent/empty parent = main-stream card as before.
- Errors: 403 no write on channel (checked first, before parent); 400 parent missing / other channel / is a reply / deleted (same `checkReplyParent` as sendMessage); no rows written on any refusal.

## Changes (backend/services/messaging)
- store/store.go: reply bookkeeping extracted from `InsertMessage` into `recordReply` (reply_count, last_reply_at, thread_participants, thread_follows, mention follows, `ErrParentGone`); no behaviour change for messages.
- store/polls_tasks.go: `InsertPollWithMessage` / `InsertTaskWithMessage` call `recordReply` in the same tx.
- domain/polls_tasks.go: `ParentMessageID` on `CreatePollInput` / `CreateTaskInput`; `checkReplyParent` after authorize+validation; `ErrParentGone` -> `ErrInvalidInput`; new `CreatePollMessage` / `CreateTaskMessage` return the announcement `*pb.Message` too (`CreatePoll` / `CreateTask` kept as wrappers).
- rest/polls_tasks.go: reads `parent_message_id`; `announceCard` -> existing `announceMessage` (PushLater + `AnnounceReply` => `ThreadReplyEvent` + follower thread states) only when the card is a reply.
- docs/specs/chat-cards/spec.md: requirement "A poll or task may be created inside a thread" + 3 scenarios.
- Tests: domain/cards_in_thread_test.go, rest/cards_in_thread_test.go (allowed, other channel, parent is reply, deleted, missing, reader/outsider/policy-deny, nothing stored, WS = thread reply event, not root broadcast).

## Gates
go vet ./... clean; go test -count=1 ./... (messaging) all pass; make check-layering ok; make build-check ok. No migration, no frontend/other-service/git-index changes, no commits.

## Notes / open
- Tests were written after the implementation (not red-first); deny cases are meaningful (without the field they would get 201).
- Main-stream card gap: fixed, see Follow-up.
- System-type replies are skipped by push (`PushForMessage` only pushes user messages), same as any system reply.

## Follow-up: main-stream cards announced live
- rest/polls_tasks.go: removed `announceCard`; CreatePoll/CreateTask call `announceMessage` always (the same path `SendMessage` uses): root -> PushLater + `BroadcastToChannel`; reply -> thread reply event + follower states.
- Test first (red: "root message was never announced"), then green: `TestCardRoutes_WithoutParentAreAnnouncedAsRootMessages` (poll and task each emit the root event, no thread event); reply tests still assert the thread-reply event and no root broadcast.
- Spec chat-cards updated (requirement text + scenario "Card created in the main stream").
- Gates re-run: go vet, go test ./... (messaging), make check-layering, make build-check all pass. Push stays a no-op for system messages (same as the normal path).

Status: DONE
Summary: Polls and tasks accept optional `parent_message_id` and become real thread replies (counted, followed, announced as the thread-reply WS event), reusing `checkReplyParent` and a shared `recordReply` store helper. All gates pass; the concern is tests came after code and main-stream cards still have no live broadcast (pre-existing).
