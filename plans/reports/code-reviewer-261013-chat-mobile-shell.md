# Code review: phone Chat shell (607deb3..314b8a7)

Date: 2026-10-13 · Branch `claude/inspiring-cray-32y4n1` · Read-only review, no services started.

Commits: df6da45 (activity API), 75ae6e2 (search API and migration 051), 4e5196f (invitation notice),
0950abd (phone shell), 4e679b1 (four Chat tabs), 314b8a7 (search screen).

## Checks run

| Check | Result |
|---|---|
| `go vet` messaging domain/rest/store | clean |
| `go test` messaging domain/rest/events, workspace domain/events | pass |
| `make check-layering`, `make check-ngac` | pass |
| vitest: conversations, useChatOverlay, useMessageSearch, ChatSearchScreen, PhoneThreadsScreen, websocket.store | 126/126 pass |
| eslint on the touched shell files | clean |
| Migrations 050/051 on the running dev DB (read-only query) | applied: `search_vector` uses `'simple'` with the tag strip; `idx_messages_latest_main` and `idx_messages_latest_reply` exist |
| `tsc --noEmit -p .` | 6 errors, all in `src/generated/` and older than this range (last touched 202b543). Not caused by these commits. |

Store tests that need a database were not run, because another agent is using the stack.

## 1. Authorization: no leak found (high confidence)

All three endpoints get their channel set from `readableChannelIDs`
(`backend/services/messaging/internal/domain/reactions_pins_receipts.go:202-223`):

- It starts from the channels of `claims.TenantID` plus DMs.
- It keeps only the channels where an NGAC `BatchCheck` grants `read` on the channel's OA.
- It fails closed: a policy error returns no channels (`domain/dms.go:38-44`).

How each concern holds:

- **Tenant and workspace scope.** `workspaceID` comes from the JWT, never from the request (`rest/activity.go:21`, `rest/search.go:26`, `rest/threads.go`).
- **Removed members.** Access is decided by the NGAC assignment, not by `channel_members`.
- **Deleted messages.** Every query filters `deleted_at IS NULL`:
  - activity: `store/activity.go:24`
  - search: `store/search.go:91`
  - followed threads: `store/threads.go` (FollowedThreads WHERE clause)
- **Replies.** A reply is in the same channel as its root, so the channel check covers it.
- **Quoted text.** Quotes are only resolved when the quoted message is in the same channel (`domain/lifecycle.go:208`), so a search hit cannot pull in quoted text from a channel the caller cannot read.
- **Deny tests exist:**
  - `TestConversationActivity_FollowsReadAccess`
  - `TestFollowedThreads_AcrossConversationsAndFollowsAccess`
  - `TestSearchAll_NeverReturnsAConversationTheCallerCannotRead`
  - `TestConversationActivityRoute_NamesNothingWithoutRead`
  - `TestSearchAllRoute_BadInputAndNoReadAccess`

**tsquery builder** (`store/search.go:18-51`): no injection path.
- The head is a bound parameter passed to `plainto_tsquery`.
- The tail can only contain letters, digits and marks, so it cannot carry tsquery syntax.
- The tail is also a bound parameter to `to_tsquery`.
- An empty head or tail gives an empty query, which `&&` drops.

**Routes:** `/api/channels`, `/api/messages` and `/api/threads` are already in `frontend/vite.config.js:17,74,76` and in the Traefik rule (`docker-compose.yml:343`). Echo matches static paths before `:chId`, so `/channels/activity` is not caught by the channel route.

## Findings

### High

**H1. The activity endpoint now runs a whole-system DM scan, and edits or deletes trigger it for every subscribed client.** Confidence: high for the code path. The load size is inferred.
- `readableChannelIDs` calls `ListAllDMs` (`store/store.go:105-113`, `SELECT ... WHERE channel_type='dm'`). This returns every DM in every tenant, and all of them go into one policy `BatchCheck`. This code existed before.
- This range puts it on hot paths:
  - Every list mount and window focus fetches `/api/channels/activity` (`hooks/useConversationActivity.ts:13-19`, default staleTime 0).
  - Every debounced search keystroke runs it again (`domain/search.go:49`).
  - Every `messageEdited` or `messageDeleted` frame in any subscribed conversation invalidates the activity query for every connected client (`stores/websocket.store.ts:502-509`).
- So one edit in a 500-member channel causes about 500 full DM scans plus 500 batch policy checks. Each check is sized by the total number of DMs in the system.
- Fix:
  1. Select only the caller's DMs: `channel_type='dm' AND id IN (SELECT channel_id FROM channel_members WHERE ngac_node_id=$1)`. NGAC still makes the final decision, so the denormalized cache only narrows the candidates.
  2. On the client, update the one affected entry in the activity cache instead of invalidating the whole query. Or debounce the invalidation and set `staleTime` to about 30s.

### Medium

**M1. A deleted message stays as the list preview for the rest of the session.** Confidence: high.
- `messageDeleted` invalidates the server activity query (`stores/websocket.store.ts:506-509`) but leaves `lastMessages[channelId]` alone. `applyMessageDeleted` (`lib/chat-cache.ts:169-199`) does not clear it either.
- `mergeLines` (`lib/conversations.ts:193-221`) prefers a live entry whenever it is strictly newer than everything the server returned.
- After the delete, the server's newest message is older than the deleted one. So the deleted text, or its reply line with a working "open thread" link, keeps showing in Home, Trực tiếp and Nhóm.
- This also matters when a moderator removes sensitive content.
- Fix: in the `messageDeleted` handler, drop `lastMessages[channelId]` when its `id === messageId`. Add a `conversations.test.ts` case for it.

**M2. A live reply hides a newer live main-stream message, and "Đánh dấu đã đọc" then stops short.** Confidence: high.
- `lastMessages` keeps one entry per channel (`websocket.store.ts:402,478`).
- Example sequence:
  1. Live main message M2 arrives, newer than the server's M1.
  2. Live reply R1 arrives and overwrites M2 in the same slot.
  3. `mergeLines` keeps the server's stale M1 as `main` and adds R1 as `reply`.
- Results:
  - `FeedRow` (`components/spaces/FeedRow.tsx:113-117`) shows M1 as the conversation's newest message.
  - `rowTargetOf` (`components/spaces/SectionOverlays.tsx:35`) sends `markRead` for `c.main.id` = M1, so M2 stays unread after "Đánh dấu đã đọc".
- Fix: keep two live slots per channel, `lastMain` and `lastReply`, and merge each against its server counterpart. Or invalidate the activity query when a live reply lands on top of a live main.

**M2b. The thread-reply `@` badge ignores mentions in the main line.** Confidence: high, low impact.
- `FeedRow.tsx:61-64` checks only `reply?.mentionsMe`.
- When there is no newer reply and the main message names the reader, no `@` is shown, although the comment on line 60 says it should be.
- Fix: `(reply ?? main)?.mentionsMe`.

**M3. A one-letter last word becomes a prefix search over the whole GIN index.** Confidence: high for the logic. Cost on production data was not measured.
- The 2-character minimum is checked on the whole text (`domain/search.go:43`), but the last word becomes `'x':*` whatever its length (`store/search.go:46-51`).
- Typing "a b" or "ok t" sends `'b':*` or `'t':*`, which matches a large share of all lexemes. It runs every 250 ms while the person types.
- Fix: in `searchTerms`, use the prefix only when the tail has at least 2 runes, and otherwise put it in the head. Also cap `q`, for example at 200 runes, in `SearchAll`.

**M4. Search and followed threads still enrich per channel, one channel at a time.** Confidence: high.
- `messageFeed` → `EnrichMessagesWithMetadata` runs once per distinct channel (`domain/stars.go:150-157`, and `enrichAcrossChannels` at `domain/threads.go:335-347`). Each run does reactions, pins and attachments queries.
- A 25-hit search page spread over 25 conversations makes about 75 extra queries per request, on every keystroke pause.
- Fix: batch reactions and attachments across all message ids in one pass. Load pins with `channel_id = ANY($1)`.

**M5. `FollowedThreads` computes unread counts and participants for every followed thread before applying `LIMIT`.** Confidence: medium (depends on the plan).
- The CTE (`store/threads.go`, FollowedThreads) evaluates `threadUnreadExpr` and `threadParticipantsExpr` for every followed root in every readable channel. Only after that does it sort and take `limit+1`.
- It also joins `messages` to itself a second time.
- Fix: in an inner query, select the ids of the next `limit+1` followed roots (filter on `threadFollowingExpr`, order, limit). Then compute the counts and participants only for those rows.

**M6. The deploy note for 051 understates the lock.** Confidence: high.
- `docs/deployment.md` (the 051 bullet) says the table is "locked against writes".
- `ALTER TABLE ... DROP/ADD COLUMN ... STORED` takes an ACCESS EXCLUSIVE lock, so reads are blocked too, and the lock is held through the index build inside the DO block.
- 050 runs a plain `CREATE INDEX` twice on `messages`, which blocks writes for both builds, and the deploy note does not mention it.
- Fix: correct the wording to "blocks reads and writes". Mention 050, and use `CREATE INDEX CONCURRENTLY` if the runner allows statements outside a transaction.

### Low

- **L1.** 051 also strips `<...>` from `content_text` (`data/migrations/051_message_search_simple.sql:36`). That column is already plain text, so a literal message like "x<y and y>z" loses those words from the index. Fix: `COALESCE(content_text, regexp_replace(content, '<[^>]+>', ' ', 'g'))`.
- **L2.** `FollowedThreads` filters `m.last_reply_at < $3` (`store/threads.go`). A root with `reply_count > 0` and a NULL `last_reply_at` drops out after page 1. Fix: `COALESCE(m.last_reply_at, m.created_at)` in both the WHERE and the ORDER BY.
- **L3.** The topic name `"workspace.invitation"` and the event struct are written out separately in `workspace/internal/events/invitation_producer.go:19-31` and `messaging/internal/events/consumer.go:316-326`. A rename in one breaks the other silently. Fix: move both into a shared `backend/pkg`.
- **L4.** `VerifiedUserIDsByEmail` queries `lower(email) = $1` (`workspace/internal/store/invitation_store.go`). It needs an index on `lower(email)`, otherwise every invite does a sequential scan of `users`. Check the existing indexes.
- **L5.** After an offer is revoked or has expired, the invitation notification still links to `/workspace-select`, where it no longer appears (`lib/notification-copy.ts`). Acceptable, but worth a line in the notifications spec.
- **L6.** Recent searches are stored in plain text in `localStorage`, keyed by user id (`lib/recent-searches.ts`), and are not cleared on sign-out. On a shared device the next person can read them from devtools. Fix: clear the `ngac-chat-recent-searches:*` keys on sign-out.
- **L7.** `events.Connect()` can return a nil `*Producer`, which is stored as a non-nil interface (`workspace/cmd/main.go`). `notifyInvitee` then still starts a goroutine and runs a DB query for nothing. It does not break anything, because `InvitationOffered` handles a nil receiver.

### Simplification

- **S1.** `useChatOverlay` (`hooks/useChatOverlay.ts`) is sound:
  - It reads the history-state flag correctly.
  - TanStack does not push a second entry for an identical href plus state, so a double tap is harmless.
  - It falls back to `replace` correctly on a deep link.

  Only one behaviour is worth noting: going from the search layer to the threads layer pushes a second entry, so Back returns to search instead of the list. Decide whether that is intended.
- **S2.** `usePersistedFlag` and `recent-searches` wrap every storage access in try/catch. `usePersistedFlag`'s set-state-during-render on a key change is a correct React pattern. No action needed.

### UI rule (no raw identifiers): no violation found (high confidence)

I checked these files:

- `ChatSearchScreen.tsx`
- `PhoneThreadsScreen.tsx`
- `FeedRow.tsx`
- `MainMenuDrawer.tsx`
- `PhoneSearchBar.tsx`
- `PhoneSectionsList.tsx`
- `NotificationScreen.tsx`
- the invitation notification copy

Ids appear only in `key`, `params` and `hueKey`. Names fall back through `displayName` to the username or "unknown person", never to an id. The invitation notification body is written by the server from names only.

## Ranked fix list

1. **H1:** select only the caller's DMs in `readableChannelIDs`. Patch the activity cache instead of invalidating it on edit or delete.
2. **M1:** clear `lastMessages[ch]` when its message is deleted.
3. **M2:** keep separate live slots for the main stream and replies, so the preview and mark-read target the real newest main message.
4. **M3:** prefix-match only a tail of at least 2 runes, and cap the query length.
5. **M4:** batch the enrichment across channels.
6. **M5:** apply `LIMIT` before the per-thread subqueries.
7. **M6:** correct the lock wording for 051 and document 050.
8. **M2b, L1–L7:** as listed above.

## Unresolved questions

- Should Back from the threads layer return to the search layer when both were opened in sequence (S1)?
- Is the migration runner transactional? This decides whether `CREATE INDEX CONCURRENTLY` can be used for 050.

Status: DONE_WITH_CONCERNS
Summary: No authorization leak or SQL injection found in the three new endpoints; deny tests exist and every check passes. One high finding (whole-system DM scan on hot paths, multiplied by edit and delete events) and two medium preview-correctness bugs (a deleted message stays as the preview; a live reply hides a newer main message, so mark-read stops short) should be fixed before shipping.
