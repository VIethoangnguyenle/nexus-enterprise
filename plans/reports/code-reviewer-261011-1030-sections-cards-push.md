# Code review: navigator sections, poll/task rules, web push

## Scope
- e3582b3 (sections and poll/task parts only): `internal/{domain,store,rest}/sections*.go`, `polls_tasks*.go`,
  `domain/cards_test.go`, migrations 041/042, specs `chat-sections`/`chat-cards`, vite/Traefik/nginx routing.
- c2d4e3c (web push): `backend/pkg/push`, `domain/push*.go`, `store/push.go`, `rest/push.go`, `grpc/hub.go` IsViewing,
  `cmd/main.go`, migration 043, deploy/*, `docs/deployment.md`, spec `web-push`.
- Plans: phase-10, phase-11 (chat-inline-threads); phase-05 + plan.md (pwa-mobile-push).

## Gates (all run, all green)
| Check | Result |
|---|---|
| `make test s=messaging` (fresh test DB) | pass; the 26 section/poll/task/push tests ran (not skipped) |
| `go test -race -run 'Push\|Section\|Poll\|Task' ./internal/domain/` | pass |
| `go vet ./...` (messaging) | clean |
| `cd backend && go test ./pkg/push/` | pass |
| `make check-layering`, `make check-ngac`, `make build-check` | pass |
| `bash deploy/check-compose-env.sh` | ok |
| Dev DB: `chat_sections`, `chat_section_items`, `push_preferences`, `polls.closed_at/closed_by` | present (migrations applied) |

Green gates do not cover the findings below. The fake `viewing` map in `push_test.go` hides H1, and nothing tests
H2 or M1.

## High

### H1. "Viewing" means "subscribed", and the navigator subscribes to every conversation, so an open tab blocks all pushes
`backend/services/messaging/internal/grpc/hub.go:145-159`, `internal/domain/push.go:149`,
`frontend/src/hooks/useConversations.ts:56-60` (mounted by `ChatNavigator`, `HomeView` and `SpaceView`).
`IsViewing` returns true when any of the user's sessions is subscribed to the channel. `useConversations` calls
`subscribeMany` for every listed space and DM so the navigator and Home stay live.
**Scenario:** a user leaves Nexus open on a desktop browser (any chat page). A colleague DMs them or @mentions them
while they are away from the desk. `IsViewing` is true for every conversation, so their phone gets nothing. This
happens on every desktop that leaves a tab open. It breaks the spec rule "none of their live sessions has the
conversation open" and phase-05's "đang xem đúng kênh/thread". The unit test passes only because it fakes
`IsViewing`.
**Fix:** track "open" separately from "subscribed". Add a focus/open frame from `SpaceView` (or a flag on the
subscribe envelope) that sets a per-client `viewing` channel, cleared on unmount, on `visibilitychange` hidden and on
disconnect. Then make `IsViewing` read that. Add a hub-level test where a client subscribed through the navigator is
not viewing.

### H2. Any writer can reassign a task (no read check on the new assignee) and then change its status
`internal/domain/polls_tasks.go:286-326`, `internal/store/polls_tasks.go:223-237`.
`UpdateTask` checks only `write` before it writes `assignee_id`. The assignee/creator/manage gate runs only when
`Status != ""`, and it compares against the assignee stored before the update.
**Scenario 1 (status bypass):** writer W is neither assignee nor creator. They send PATCH `{assignee_id: W}` (no status,
so the gate is skipped), then PATCH `{status: "done"}`. W is now the assignee and the change succeeds. This defeats
"any other writer is refused (403)" in `docs/specs/chat-cards/spec.md`.
**Scenario 2 (assignee rule):** PATCH `{assignee_id: <user who cannot read the channel>}` succeeds. The spec says
"A task's assignee SHALL be someone who reads the channel (400 otherwise)", but only CreateTask enforces it.
**Fix:** move the assignee read check into a helper and call it from both Create and Update. Treat a reassignment like
a status change (assignee, creator or `manage`), or limit it to creator/`manage`. Add deny tests for both scenarios.

### H3. The OneSignal request does not do what the spec promises on web push
`backend/pkg/push/push.go:88-96,104-112`, `internal/domain/push.go:250`.
- `collapse_id` only works for mobile push, per the OneSignal create-notification reference ("For mobile push
  only"); web push uses `web_push_topic`. On a PWA the "N tin mới" follow-up shows up as a second notification. It
  does not replace the first, though the spec and the commit message say it does.
- `web_url` gets a relative path (`/channels/<id>`). OneSignal documents `url`/`web_url` as an `https://` URL. If
  OneSignal rejects the relative path with a 4xx, every push fails: 4xx is not retried and only a warning is logged.
  If it accepts the path, the click target is still undefined. **Not verified against the live API** (no credentials).
  It must be checked in phase 06 before the feature is called done.
**Fix:** build an absolute URL from a configured base (`APP_BASE_URL` already exists in the env set). Send
`web_push_topic` (and keep `collapse_id` for future mobile). Assert both in `TestOneSignal_SendsOneRequest…`.

## Medium

### M1. Two concurrent single-choice votes from one user both stay
`internal/store/polls_tasks.go:99-116`. The unique key is `(poll_id, option_id, user_id)` (migration 002). Under READ
COMMITTED, tx1 (option A) and tx2 (option B) each run `DELETE … option_id <> $3` before the other commits. Neither
sees the other's row, both INSERTs succeed, and the single-choice poll holds two votes from one person. A double tap
on two options, or two devices, is enough.
**Fix:** start the transaction with `SELECT 1 FROM polls WHERE id=$1 FOR UPDATE` (or
`pg_advisory_xact_lock(hashtext($poll||$user))`). The same lock also closes the vote-vs-ClosePoll race (L4).

### M2. A read-only assignee cannot complete their own task
`internal/domain/polls_tasks.go:287`. `UpdateTask` requires `write` before the assignee shortcut. CreateTask allows an
assignee who only reads, and the spec says the assignee may change status. The test `TestUpdateTask_Status…` grants
the assignee `write` before the call (`cards_test.go:180`), which hides the gap.
**Fix:** for a status-only update by the assignee or creator, require `read`; require `write` for other fields.

### M3. Retries are not idempotent, so one message can push twice
`backend/pkg/push/push.go:117-137`. A client timeout (10 s) or a 5xx after OneSignal already created the notification
gets retried, and the device receives a duplicate.
**Fix:** send OneSignal's `idempotency_key` (a UUID per `Message`, the same on every attempt).

### M4. The 30 s budget is discarded; sends run one recipient at a time with no overall bound
`internal/domain/push.go:223-225, 235, 294-296`. `PushLater` sets a 30 s deadline. `send()` then calls
`context.WithoutCancel(ctx)`, which strips that deadline, and adds 20 s for each recipient. Recipients are processed
one after another, each with its own policy check, preferences query and HTTP POST.
**Scenario:** a reply in a thread with 200 followers while OneSignal is unreachable keeps one goroutine alive for up to
200 × 20 s, about 67 minutes. A busy channel stacks one such goroutine per message.
**Fix:** keep the parent deadline in `send` (drop `WithoutCancel` there; only the `AfterFunc` path needs it). Load the
preferences of all recipients in one query. Send recipients that have no open batch in one request (`include_aliases`
accepts many ids) or through a bounded worker pool.

### M5. The client-supplied `mentions` list becomes push fan-out with no limit and no link to the text
`internal/domain/push.go:126-131`; mentions reach it unvalidated from `rest/handler.go:207,232` and
`domain/messages.go:95`.
**Scenario:** any writer posts a message with `mentions` set to every member's id (ids come from the people
endpoints). Every reader of the channel gets a "mention" push with the attacker's text. The list has no size limit
and nothing checks that the mentions appear in the content, so it costs one `PeopleByID` with N ids, N policy calls,
N preference queries and N HTTP posts.
**Fix:** cap `mentions` (for example 50), drop ids that are not referenced in the content, and dedupe.

### M6. Deny tests are missing for new decision paths (CLAUDE.md §5)
- `CreateSection` with `channel_ids` (the Undo restore path, `domain/sections.go:108-112`): no test with an unreadable
  or other-workspace channel.
- `MoveSection` with another user's section id; `UnplaceChannel` from outside the workspace.
- `UpdateTask` reassignment (H2).
- Push: no test that the 30 s follow-up respects the filters (L1 below).

## Low

- **L1. The follow-up push is not filtered again.** `domain/push.go:280-291`: "N tin mới" goes out at the end of the
  window even if the user opened the conversation, entered quiet hours, turned the kind off or lost `read` during
  those 30 s. It contains no message text, so the impact is small.
- **L2. Section positions can tie.** `store/sections.go:74-81`: two concurrent creates with no position both read
  `MAX+1`. `SwapSections` on equal positions changes nothing, and `MoveSection` still answers 200.
- **L3. Moving a DM between workspaces' sections.** `UnplaceChannel` (`store/sections.go:161`) is not scoped to the
  workspace in the URL. `UNIQUE(user_id, channel_id)` (migration 041:28) means placing a DM in workspace B's section
  silently takes it out of workspace A's. `users.ngac_node` is global, so DMs read the same from any tenant. This
  stays within the user's own data and is not a leak, but the behavior is not stated in the spec.
- **L4. Vote vs close.** `VotePoll`/`RemoveVote` check `pollClosed` and write afterwards without a lock, so a vote can
  land just after ClosePoll. The M1 lock fixes this.
- **L5. Errors that should be 400 come back as 500.** An invalid task status or an unknown `assignee_id` on update hits
  the CHECK/FK constraint and returns 500 (`store/polls_tasks.go:223`).
- **L6. PUT push-preferences echoes the request.** It returns the request body, not what was stored: an empty
  `time_zone` comes back empty while `Asia/Ho_Chi_Minh` is stored (`rest/push.go:83`). A body with keys missing turns
  every kind off.
- **L7. Lookup error ignored.** `sender, _ := s.store.DisplayName` (`domain/push.go:222`). If the lookup fails, a DM
  push goes out with an empty heading.
- **L8. The thread link does nothing yet.** The push URL `?thread=<id>` is ignored: `routes/_workspace/channels.$channelId.tsx`
  has no search schema. The URL also carries no workspace. This is frontend phase work; tracking it here.
- **L9. Quiet-hours test depends on the clock.** It uses `time.Now()`. The wrap-around branch runs only when the test
  happens to run between 22:00 and 23:59 UTC, and the time-zone branch never runs, even though `pushState.now` exists
  for injecting time.
- **L10. Undo fails for the whole section.** If one of a deleted section's channels has become unreadable, the Undo
  `CreateSection` fails entirely and the section cannot be restored. Skipping unplaceable ids on restore would keep
  the rest.

## Verified as correct
- Sections are private: every call goes through `inWorkspace` (tenant match plus `read` on the Documents OA) and
  `ownSection` (`user_id` in the WHERE), so another user's id is 404 on rename, move, delete and place. Placing a
  channel requires `read` and either the same workspace or a DM. Name uniqueness is case-insensitive through
  `uq_chat_sections_name`, so 409 under races too. Shifting positions on an Undo with a gap is correct.
- Poll results stay hidden on every path. All of GetPoll, CreatePoll, the vote/remove/close answers and pollAnswer go
  through `pollToProto(…, viewerID)`. `PollVoteEvent` carries ids only. Anonymous polls never fill `VoterIds`.
  `pollClosed` covers both `closed_at` and `ends_at`. ClosePoll requires creator + `write`, or `manage`. REST `ends_at`
  must be RFC 3339 and in the future.
- Push: `FromEnv` refuses a half configuration; the key goes only into the Authorization header; logs carry ids and
  counts, never text. Quiet-hours wrap-around logic is correct, and time zones are validated on save. `read` is
  checked again for each recipient and fails closed. Routing of `/api/me/push-preferences` is right in vite (before
  `/api/me`), Traefik (messaging priority 120 > auth 110) and nginx (`^~` beats the `/api/me` regex). The compose env
  and `check-compose-env.sh` are consistent. `chat-sections` routing is in the vite regex block, Traefik and nginx.

## Plan follow-ups
- Phase 10 (sections), backend: complete apart from M6 and L2. Its checklist (UI, mockup, IDs off screen) is not
  covered by this backend review.
- Phase 11 (cards), backend rules: H2 and M2 are against `chat-cards` spec text and must be fixed before the checkbox
  "test deny đủ các nhánh quyền".
- Phase 05 (push): don't mark done until H1 and H3 are fixed. Real-device delivery (phase 06) has to verify the
  `web_url`/`web_push_topic` behavior. The plan's assigned-task and approval events are still open, as the plan
  itself says.

## Unresolved questions
1. Does OneSignal accept a relative `web_url`? This needs a live call with the app credentials (phase 06).
2. Is moving a DM across workspace sections (L3) the intended behavior? Should it be written into
   `chat-sections/spec.md`?
3. Should reassigning a task be limited to the creator and `manage`, or also allowed to the current assignee?

Status: DONE_WITH_CONCERNS
Summary: All gates pass, but there are three High findings. Pushes are blocked for anyone with an open tab, because "viewing" is really "subscribed" and the navigator subscribes to everything. Any writer can reassign a task and so get around the status rule. The OneSignal request uses the mobile-only `collapse_id` and a relative `web_url`.
Concerns/Blockers: H3's relative-URL rejection is unverified without OneSignal credentials.
