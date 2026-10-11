# Code review: chat-shortcuts backend (stars, starred feed, mentions feed)

Scope: backend/services/messaging (store/stars.go, domain/stars.go, rest/stars.go, handler.go, reactions_pins_receipts.go,
messages.go, models.go, tests), backend/proto/messaging, data/migrations/048_chat_stars.sql, docs/specs/chat-shortcuts.
Uncommitted, read-only review.

Gates run: `go vet ./...` clean; `go test ./... -count=1` 252 passed (6 packages); `make check-layering check-ngac` pass.

## Critical
None.

## High
None.

## Medium

1. **Cascade deletes with no index on the child key** — `data/migrations/048_chat_stars.sql:5-20`.
   PKs are `(user_id, channel_id)` and `(user_id, message_id)`; the only extra index is `(user_id, created_at DESC)`.
   `ON DELETE CASCADE` from `channels(id)` / `messages(id)` must find rows by `channel_id` / `message_id`, which no
   index leads with. Scenario: workspace teardown (`backend/services/workspace/internal/store/teardown.go:75`,
   `DELETE FROM channels WHERE workspace_id = $1`) cascades to every message of the workspace, and each deleted
   message runs a sequential scan of `starred_messages`. Cost is messages x star rows inside one transaction, which
   grows with adoption. Fix: add `CREATE INDEX ... ON starred_messages (message_id)` and
   `... ON starred_conversations (channel_id)`.

2. **Feed enrichment runs once per distinct channel** — `domain/stars.go:133-142`.
   `EnrichMessagesWithMetadata` runs reactions, pins, attachments and quotes queries for each channel group, so a
   50-item page spread over 50 conversations makes about 200 round trips, plus `readableChannelIDs`
   (`ListChannelsByWorkspace` + `ListAllDMs` + a batch policy check) on every call. Mentions are naturally spread
   across many channels, so this is the common case, not the worst case. Reactions, attachments and quotes are
   message-keyed and can be batched for the whole page. Only pins are per channel, and they can be loaded with one
   `channel_id = ANY` query. This is not a blocker at current scale, but it is the first thing to fix if the feeds get slow.

## Low

3. **The report says something false about indexes** — `plans/reports/fullstack-developer-261011-1050-chat-shortcuts-backend.md:36`
   says "No SQL index on `messages.mentions`". `idx_messages_mentions` (GIN, full table) exists since
   `data/migrations/004_messages_enhancements.sql:19`, and `idx_messages_thread_mentions` (partial) since 037. The
   `mentions @> ARRAY[$1]` filter in `store/stars.go:130` can use it. Correct the note so no one adds a duplicate GIN index.

4. **The page cursor depends on the client keeping full precision** — `domain/stars.go:86-97`, `store/stars.go:94-95,132-133`.
   A raw `timestamptz` + id is correct when the client echoes microseconds back exactly (Postgres stores microseconds,
   and timestamppb nanos are multiples of 1000). It is not correct for a JS client that rounds to milliseconds
   through `Date`: a client-built cursor truncated to ms is earlier than the real value, so rows in the gap
   (T_ms, T_us) are skipped silently. The report hands the frontend a manual `padStart(9)` recipe to avoid this. An
   opaque `next_cursor` string in the response would remove the trap entirely. If you keep the current design, the
   frontend must follow the recipe exactly.

5. **403 vs 404 shows whether a message exists** — `domain/stars.go:53-63`. An unknown id gets 404 and an existing
   message in an unreadable channel gets 403. This matches the existing `authorizeMessage`/`authorizeChannel` pattern
   (`domain/authz.go:46-78`), and ids are opaque, so it is not a regression. The order is correct for deleted
   status: `facts.Deleted` is checked only after read passes, so a deleted message's state does not leak to
   non-readers.

6. **A removed member cannot clear stale stars** — `domain/stars.go:61`. Unstar needs read, so the rows stay until
   the channel/message cascade. The spec allows this (spec.md:12-15), and the rows are invisible because the feed
   re-checks the graph. This is informational only.

7. **A policy outage returns an empty feed instead of an error** — `readableChannelIDs` -> `filterAccessible`
   (`domain/reactions_pins_receipts.go:197-219`) fails closed by dropping channels, so the feeds return 200 `[]`
   while policy is down. This is the same behaviour as the sidebar unread counts. It is safe, but it looks to the
   user like "no mentions". This is informational only.

8. **Thread-reply unread rule differs from the "all" level unread count** — `store/stars.go:123` falls back to
   `root.created_at` when there is no `thread_follows` row. `GetUnreadCounts`' `all_thread_unread`
   (`store/reactions_pins_receipts.go:287`) falls back to the channel read position first. The mentions rule matches
   the spec (spec.md:46) and `threadUnreadExpr` (`store/threads.go:41`), so it is consistent with thread badges.
   Note only: a reply mention stays `unread` after the user reads the channel, until they open the thread.

## Verified OK
- Authorization: star and unstar on both conversations and messages call `authorizeChannel(..., OpRead)` before any
  write, and messages are checked on their channel's OA, not as a node. Both feeds filter by
  `readableChannelIDs` (graph re-check on every call, workspace from the token `claims.TenantID` plus DMs), so a
  removed member or another workspace sees nothing. Tests cover this (`domain/stars_test.go:105-162`,
  `rest/stars_test.go:29`).
- Deny tests: 403 on all four star routes without read (domain and REST). 401 without a session on the production
  router. Removed member and other workspace give empty feeds.
- Mentions scope: `sender_id <> $1`, `deleted_at IS NULL`, roots and replies, muted level ignored. Unread for roots
  uses `read_receipts.last_read_at` (set to `NOW()` on mark-read), and for replies `thread_follows.last_read_at`,
  else the root's creation time.
- Pagination tie-break: `(created_at, id) < (before, before_id)` with matching `ORDER BY ... DESC, id DESC`, fetching
  `limit+1` rows for `has_more`. Test pages one row at a time through a tie (`stars_test.go:105`, `:277`).
- Starred feed index: `(user_id, created_at DESC)` matches the filter and order.
- Route order: static `/messages/starred` and `/messages/mentions` against `/messages/:msgId` is covered by the
  production-router test. The existing `/api/messages` and `/api/channels` proxy rules cover the new routes.
- Proto: additive fields only (24-26, 12, 7), no renumbering. `messaging.pb.go` was regenerated. The frontend
  generates only `ws.proto` (`frontend/src/generated/proto/messaging/ws.ts`), so `proto:gen` correctly produces no
  diff. CLAUDE.md §5 is satisfied.
- No new NGAC strings, graph writes or SQL in transports (check-ngac and check-layering pass).
- The schema change was applied to the dev DB and the test DB (per the dev report). Not re-verified here: psql is not
  on PATH.

## Recommended actions
1. Add `starred_messages(message_id)` and `starred_conversations(channel_id)` indexes to 048 before it ships. The
   migration is unapplied in production, so edit it in place.
2. Correct the GIN index note in the dev report.
3. Optional: batch the feed enrichment across channels, and consider an opaque `next_cursor`.

Status: DONE_WITH_CONCERNS
Summary: Authorization, deny tests, unread semantics, tie-break paging and the proto change all hold, and all gates pass. Concerns: migration 048 lacks child-key indexes for its cascades (slow workspace teardown), feed enrichment is per channel, and the dev report wrongly says there is no mentions index.
