# Code review: chat phase 02, thread data model and API

Branch `claude/inspiring-cray-32y4n1`, uncommitted changes. Plan: `plans/261010-1559-chat-inline-threads/phase-02-thread-data-model-and-api.md`.

## Scope
- Files: migration 037, `store/{threads,store,reactions_pins_receipts,models}.go`, `domain/{threads,messages,reactions_pins_receipts}.go`, `rest/{threads,handler,reactions_pins_receipts}.go`, `grpc/server.go`, `messaging.proto` plus the generated Go, `docs/specs/chat-threads/spec.md`, `docs/specs/README.md`, and the tests.
- Commands run:
  - `go build`, `go vet` and `go test -count=1 ./...` in messaging: 65 passed, 0 skipped.
  - `go vet` in every service: clean. Auth's `MessagingServiceClient` fakes still compile.
  - `make check-docs`, `make check-ngac` and `make check-layering`: green.
  - EXPLAIN VERBOSE of the GetUnreadCounts SQL on the dev DB.
  - Migration 037's backfill replayed inside a rolled-back transaction on the dev DB.

## Overall
The layering, the NGAC usage and the deny coverage are sound. There are no trust-boundary holes in the new endpoints. The SQL scoping is correct. One High issue needs a product decision before merge: the GetUnreadCounts fix closes a real cross-tenant leak, but it also takes unread badges away from most channels a member can see. There are two Medium performance and data issues and a few Low ones.

## Critical
None.

## High

**H1. GetUnreadCounts now covers only `channel_members` rows, which is a much smaller set than the channels a user can read.** `store/reactions_pins_receipts.go:257`
- **The fix itself is right.** The old subquery `(SELECT ngac_node_id FROM users …)` resolved to the outer `cm.ngac_node_id`, so every caller received every channel in every tenant. That leaked channel ids, unread counts and last-read ids across tenants. `claims.NGACNodeID` is the correct value: auth's `SwitchTenant` re-issues the token with the same `users.ngac_node` (`auth/internal/domain/tenants.go:160`), and that same node is what `channel_members` stores. The only callers are REST `GetUnreadCounts` and the domain tests. The gRPC `GetUnreadCounts` RPC has no server implementation, and that was already true before this change.
- **The side effect is the problem.** `TenantMember_*` holds `read` on the workspace Channels OA, and every channel's Content OA sits under that OA (`domain/channels.go:158`). So every workspace member can read, and sees in the sidebar (`ListChannels` → `filterAccessible`), every workspace channel. But `channel_members` only holds the creator and explicit invitees. On the dev DB that is 60 workspace channels and 59 rows, one per channel.
- **Failure scenario:** member B opens a workspace channel created by A, and B was never invited. Before this change B got an unread badge, by accident of the bug. Now `useConversations` (`frontend/src/hooks/useConversations.ts:34`) finds no entry for that channel, so the badge is 0 forever and `thread_unread_count` is never reported either.
- **Status:** the spec (`chat-threads/spec.md` "Unread counts cover the caller's own channels") records "member of" deliberately. It is still a user-visible regression in the most common case, and the commit should not land it silently.
- **Options:**
  - (a) Accept it. Badges then appear only for joined spaces, which needs a "join" UX in phase 03+.
  - (b) Scope the query to `channels.workspace_id = claims.TenantID` and keep only the channels that pass a BatchCheck `read`. This uses the same filter `ListChannels` uses, so the badge set equals the sidebar set.
  - (c) Use the union of `channel_members` and the result of option (b).
- **This needs the user's decision.**

## Medium

**M1. The GetUnreadCounts thread subquery scans every thread root of every member channel, and the frontend refetches it on every incoming message.** `store/reactions_pins_receipts.go:248`
- **Cost:** EXPLAIN shows SubPlan 2 running once per member channel. Each run filters all roots with `reply_count > 0` in that channel. For each root it evaluates the follow expression, and for each followed root it runs a count subquery.
- **How often it runs:** `websocket.store.ts:375` invalidates `unreadCounts` on every `newMessage`. Every connected client in a channel therefore re-runs the whole thing on every message.
- **Cost estimate:** a user in M channels with T threads each pays O(M·T) per message, multiplied by the number of online members.
- **The plan already flagged this.** Its Risk section says "đo trước/sau; nếu > 2x…" (measure before and after). There is no evidence a measurement was taken.
- **Fix options:**
  - Drive the sum from the rows that can be non-zero. These are the explicit `thread_follows` rows with `following`, plus implicit roots where `sender_id = $1` or `$1 = ANY(mentions)`; the latter can use the existing GIN index on `mentions`.
  - Or add a partial index `messages(channel_id, last_reply_at DESC) WHERE parent_message_id IS NULL AND reply_count > 0`. ListThreads would also benefit, because it currently filters through `idx_messages_channel_time` and then sorts.
  - Measure the cost before merging either way.

**M2. Migration 037's follow backfill is not safe to re-apply, and `make db-migrate` re-applies every migration on every run.** `data/migrations/037_thread_follows.sql:45-49`; `Makefile:556-565`
- Prod is safe: `deploy/migrate.sh` keeps a ledger, so the file runs once.
- **The dev problem:** the dev target re-runs all files. A thread started after the first run gives its starter no row, because the starter follows implicitly with the read position at the root. On the next `make db-migrate`, the first INSERT creates a row for that starter with `last_read_at = NOW()`. That silently marks every unread reply in their threads as read.
- **Failure scenario:** A starts a thread, B replies twice, A's badge shows 2. Any developer runs `make db-migrate`, and A's badge is now 0.
- **What I could and could not show:** the dev DB currently has no such thread, so the rolled-back replay inserted 0 rows. The logic is deterministic all the same.
- **Fix:** limit the backfill to threads created before this migration, for example with a guard on the root's `created_at`. Or skip it when `thread_follows` already has rows, for example `WHERE NOT EXISTS (SELECT 1 FROM thread_follows)`.

## Low

**L1. ListThreads pages by a timestamp-only cursor while sorting by `(last_reply_at DESC, id DESC)`.** `store/threads.go:123,129`
- Threads with the same `last_reply_at` at a page boundary are skipped.
- The REST response serialises `last_reply_at` as `{seconds,nanos}` (proto struct through Echo's JSON), while `before` must be RFC 3339. A client that converts through a JS `Date` truncates to milliseconds. It will then skip any thread whose last reply falls in [ms floor, cursor).
- `GetMessages` already pages the same way by `created_at`, so this follows an existing pattern. It is still cheap to fix now: use a `(last_reply_at, id)` cursor, or document microsecond RFC 3339 for phase 03.

**L2. Message times come from two different clocks.** Message `created_at` and `last_reply_at` use the Go clock (`domain/messages.go:56`, `store/store.go:231,244`). The thread read position uses the DB clock (`store/threads.go:83,96,99`).
- If clocks skew, or a reply commits after a concurrent MarkThreadRead whose `NOW()` is later than that reply's `created_at`, the reply is never counted unread for that reader.
- The window is small: it lasts from `time.Now()` to the commit. Channel receipts have the same property. Note it in the spec, or set the read position to the newest reply's `created_at` instead of `NOW()`.

**L3. `TestThreadStates_IsOneQueryForAPage` does not count queries.** `store/threads_test.go:182`
- It asserts results only. The acceptance criterion ("1 truy vấn tóm tắt — kiểm bằng test đếm query hoặc EXPLAIN", i.e. one summary query, checked by a query-count test or EXPLAIN) is met by construction, but the test name claims more than the test proves.
- Either rename the test, or wrap the store's pool with a counting tracer (`pgx` `QueryTracer`) and assert exactly one query.

**L4. `following=false` on a `Message` is ambiguous.** `SearchMessages`, `FindByEntity` and pinned messages do not call `applyThreadStates`, so their root messages carry `following=false` and `thread_unread_count=0` even when the caller follows the thread. The frontend cannot tell "not computed" from "not following". Either document on proto fields 18/19 that they are filled only by GetMessages, GetThread and ListThreads, or apply the states on those paths too.

**L5. Untested new branches:**
- `GetUnreadCounts` returning `ErrInvalidInput` on an empty user or node (`domain/reactions_pins_receipts.go:164`).
- The four new gRPC delegations (`grpc/server.go:139-173`), which have no gRPC-layer test.

They are thin, but CLAUDE.md §5 asks for coverage of new branches.

**L6. Index cleanup.** `idx_messages_thread_time (parent_message_id, created_at)` supersedes the existing `idx_messages_thread (parent_message_id)` (`data/init.sql:124`). The old index is now pure write overhead.

**L7. `checkReplyParent` gives different 400 messages for "unknown parent" and "parent in another channel"** (`domain/threads.go:28-30`). A channel writer can use this to tell whether a message id exists elsewhere. The ids are UUIDs and the check runs after the write check, so the impact is negligible. One combined message would remove it.

## Verified correct (asked explicitly)
- **(d) SQL scoping:**
  - Every query that embeds the shared expressions binds `$1` = user id: ThreadStates, ListThreads, MarkThreadRead and GetUnreadCounts.
  - In GetUnreadCounts the inner `FROM messages m` shadows the outer `m`. EXPLAIN shows the subplan on `m_1`, and the `SUM` aggregates at the inner level because its argument references only inner columns and `$1`.
  - `cm.channel_id` correlates correctly.
  - The `LEFT JOIN thread_follows tf ON FALSE` in MarkThreadRead makes `tf.*` NULL, so the inserted `following` is the implicit value. On conflict only `last_read_at` changes. That is correct.
  - `GREATEST(COALESCE(last_reply_at,$2),$2)` is correct and monotonic.
  - The reply upsert's `GREATEST(thread_follows.last_read_at, EXCLUDED.last_read_at)` never moves a read position backwards.
- **(b) Callers of the changed signatures:** every caller of GetMessages, GetThread and GetUnreadCounts (REST, gRPC, tests) is updated. Every query that scans through `scanMessages` uses `messageCols` (ListMessages, GetThread, FindByEntity, SearchMessages), and ListThreads scans the same 14 columns plus 3. There is no other writer of messages, since WS sends go through `domain.SendMessage`.
- **InsertMessage transaction:** the count, `last_reply_at`, the participant and both follow upserts are in one `BeginFunc`. Concurrent replies serialise on the parent row's UPDATE. Neither the FK KEY SHARE locks nor the ON CONFLICT upserts can deadlock here.
- **(c) Contract changes:** proto fields 16-19 on `Message`, field 4 on `ChannelUnread`, two new messages and four RPCs are all additive. The generated header versions are unchanged. The frontend generates only `ws.proto`, so `npm run proto:gen` is not required. The new routes fall under the already-proxied `/api/messages` and `/api/channels` (8183), and `/channels/unread` (static) still wins over `/channels/:chId/threads`.
- **(e) Migration:** the schema statements are idempotent (IF NOT EXISTS, guarded UPDATE, ON CONFLICT). It is applied on the dev DB; columns and indexes were checked. Data idempotency is covered under M2.
- **(f) Security:**
  - All four endpoints check `read` on the channel Content OA (`ngac.OpRead`) before any write.
  - The deny tests prove that no row is written. REST deny returns 403 on all four routes.
  - Thread state only reaches the caller after a channel read check (GetMessages, GetThread, ListThreads), or for the caller's own `channel_members` rows (unread counts).
  - Users who are mentioned without access get a follow row that they can never read through any path.
  - No NGAC strings are added outside `backend/ngac`.
- **(a) Acceptance:**
  - Migration applied: yes. The backup cannot be verified from here.
  - Tests green with 0 skips: yes, and there is a deny test for every new endpoint and for the new SendMessage parent checks.
  - One query per page: true by construction (see L3).
  - Spec and index updated: yes. `check-docs` is green.
  - The plan's "measure GetUnreadCounts before/after" step: **not evidenced** (M1).

## Recommended actions
1. Decide H1 with the user: accept joined-only badges, or scope unread counts to the channels the user can read in the tenant.
2. Fix M2: guard the backfill so that `make db-migrate` cannot reset read positions.
3. Measure GetUnreadCounts on a realistic dataset (M1), and add the partial index or rewrite the subquery.
4. Optional: add a composite cursor (L1), add query counting (L3) and document proto fields 18/19 (L4).

## Unresolved questions
- H1: should unread badges cover every readable channel or only joined ones? The spec says joined, but the sidebar shows every readable channel.
- Was the DB backed up before 037 was applied, as the plan requires?

Status: DONE_WITH_CONCERNS
Summary: The thread model, API, authorization and SQL are correct and the tests are green with no skips. The GetUnreadCounts fix is right for tenancy, but it drops unread badges for every readable channel not cached in channel_members (H1). Re-running migration 037 in dev silently marks starters' unread replies as read (M2), and the new unread subquery runs on every incoming message without having been measured (M1).
Concerns/Blockers: H1 needs a product decision from the user before merge.
