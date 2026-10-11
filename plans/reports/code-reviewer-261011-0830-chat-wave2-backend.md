# Code review: chat wave 2 backend (attachments, rich text, message lifecycle)

Date: 2026-10-11. Branch `claude/inspiring-cray-32y4n1` @ dd805b4 plus uncommitted changes. Read-only review.

## Scope
- Migrations 038/039/040; `backend/proto/messaging/{messaging,ws}.proto` and the generated `.pb.go` files; messaging `store/`, `domain/`, `rest/`, `grpc/` files and their tests; `go.mod` (bluemonday); specs `chat-attachments`, `chat-rich-text`, `chat-message-lifecycle`; plan phases 06/07/08 (backend parts only).
- Gates run against the clean test DB, rebuilt from the branch schema:
  - `make test s=messaging`: PASS. All 5 packages ok, none skipped.
  - `make check-layering`: PASS.
  - `make check-ngac`: PASS.
  - `make build-check`: PASS.
  - `go vet`: clean.
- Re-running `init.sql`, `004`, `038`, `039` and `040` a second time succeeds. Afterwards `search_vector` is still `to_tsvector('english', COALESCE(content_text, content))`, because 004's `ADD COLUMN IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS` do nothing on the second run. The dev DB has all of the new columns and the new table.
- Behaviour was checked with throwaway overlay tests in the scratchpad. No repo files were touched.
- Note: one test-DB rebuild deadlocked on `041_chat_sections.sql`, because another session was using the same DB at the same time. A retry succeeded. This is unrelated to this change.

## Overall assessment
The core of the new attachment path is sound:
- The `ChannelDriveFiles` CTE is correct. A probe confirmed that it refuses a file under a trashed folder, a parent cycle, and a file whose row claims the channel context but sits in the workspace drive.
- Redaction is applied when rows are scanned (`scanMessages`, `ListThreads`), so every path that reads `messageCols` is covered.

However, the **legacy `drive_file` read path skips that check completely**, and the **server-side HTML filter is keyed on a format label the client chooses**. Both undercut what the specs promise. There are also several correctness gaps in the lifecycle.

---

## Critical

### C1. A legacy `linked_entity_type='drive_file'` link exposes any drive file's metadata, across channels and tenants
- **Where:**
  - `backend/services/messaging/internal/store/attachments.go:94-98`: `ListAttachmentsForMessages` joins `drive_items d ON d.id = m.linked_entity_id` with no channel, workspace or status condition.
  - `backend/services/messaging/internal/domain/messages.go:30-31,89-90`: `SendMessage` stores whatever `linked_entity_type` / `linked_entity_id` the client sends (`rest/handler.go` binds both).
  - `domain/reactions_pins_receipts.go` `EnrichMessagesWithMetadata` now runs this query for every list and thread page.
- **Failure scenario (probe-confirmed):**
  - A writer in channel A posts `{"content":"x","linked_entity_type":"drive_file","linked_entity_id":"<id of a file in channel B's drive, another workspace, or another tenant>"}`.
  - From then on, every reader of A gets that file's `file_name`, `mime_type` and `size_bytes` as an attachment of the message.
  - The probe returned the foreign file. It also returned a `trashed` file.
  - This defeats the phase 06 invariant ("mỗi `drive_file_id` đính kèm phải nằm trong drive của chính kênh đó") and the spec's "Only a confirmed file of the channel's own drive can be attached". The deny tests only cover the new `attachments` field.
  - Precondition: the attacker needs the file id. File ids are UUIDs, but they circulate in shares, URLs and former-member caches.
  - Before this change the backend never resolved the id. The frontend fetched the file through drive, where NGAC applied.
- **Fix:**
  1. In the legacy branch, apply the same parent-chain check against `m.channel_id`, and require `d.status = 'active'`. The simplest way is to reuse the CTE keyed by each message's channel.
  2. In `SendMessage`, either refuse a client-supplied `linked_entity_type = 'drive_file'`, or validate it through `ChannelDriveFiles` and turn it into an attachment.
  3. The frontend still sends this legacy shape (`frontend/src/components/spaces/SpaceView.tsx:151`), so option (b) avoids breaking it.
  4. Add a deny test: a legacy link to another channel's file must read as no attachment.

## High

### H1. The server HTML allowlist can be skipped by labelling the content `markdown` or `plain`
- **Where:**
  - `backend/services/messaging/internal/domain/richtext.go:66-73`: only `format == "html"` is sanitized.
  - `domain/lifecycle.go:52-55`: an edit can also switch the format.
  - The client renders any content that starts with `<` as HTML (`frontend/src/components/chat/MessageContent.tsx:16`).
- **Failure scenario:**
  - A member POSTs `content_format: "markdown"` with `<img src="https://attacker/p.gif"><div style=...>`.
  - The content is stored unfiltered, and every reader's client renders it as HTML.
  - DOMPurify's defaults still allow `img`, `svg`, `table` and the like. That gives a tracking pixel (reader IP and read time), layout spoofing, and fake UI.
  - The spec's claim that "no client can store markup that runs or restyles anything" (`docs/specs/chat-rich-text/spec.md`, Purpose) does not hold. The server is not a boundary here.
- **Fix:** do both of the following.
  - Validate `content_format ∈ {markdown, plain, html}`, otherwise 400 (see M2).
  - Sanitize non-html bodies too: either run them through a policy that strips all tags, or sanitize whenever the body contains `<`.
  - Then add a test: markdown containing `<img onerror>` is stored without the tag.

### H2. A deleted root can still be replied to, which brings the hidden message back
- **Where:**
  - `backend/services/messaging/internal/domain/threads.go:24-38`: `checkReplyParent` does not check `deleted_at`.
  - `store/store.go:237-241`: the reply-count `UPDATE` has no `deleted_at IS NULL` guard.
  - `store/store.go:280-281`: the `ListMessages` filter is `deleted_at IS NULL OR reply_count > 0`.
- **Failure scenario:**
  - A root with no replies is deleted. Clients drop it (`keep_placeholder=false`).
  - Then any writer replies to its id, either via the API or a stale thread panel. Or the reply simply races the delete: both sides lock the root row, and if the delete commits first, `KeepPlaceholder` was computed as false.
  - `reply_count` becomes 1, so the "deleted" root reappears in the stream and in `ListThreads` as a placeholder that no client was told about.
  - The spec says "any other deleted message leaves the stream".
- **Fix:**
  - Refuse a reply to a deleted parent inside the `InsertMessage` transaction: `UPDATE ... WHERE id=$1 AND deleted_at IS NULL`, and fail with `ErrNotFound`/`ErrInvalidInput` when no row is updated.
  - Add a deny test.

## Medium

### M1. A message that sanitizes to nothing is accepted
- **Where:** `domain/messages.go:67` checks the raw `in.Content` before `prepareContent`.
- **Scenario:**
  - html `<script>x</script>` or `<p></p>`, with no files, stores an empty message. The probe confirmed that both sanitize to `""` / `<p></p>` with empty text.
  - This contradicts the chat-attachments spec ("one with neither is refused").
- **Fix:** test `strings.TrimSpace(contentText) == ""` after `prepareContent`, as `EditMessage` already does (`lifecycle.go:59`).

### M2. An unknown `content_format` gives a 500 instead of a 400
- **Where:** `domain/messages.go:72-75` and `domain/lifecycle.go:52`. There is no enum validation.
- **Mechanism:** migration 004 has `CHECK (content_format IN ('plain','markdown','html'))`.
- **Scenario:** `content_format: "HTML"` raises a check-constraint violation from the store, which maps to a 500 and an error-log entry. The client cannot fix its input.
- **Fix:** validate in the domain and return `ErrInvalidInput`. This also closes half of H1.

### M3. Concurrent edit/delete returns 500 where the spec says 404
- **Where:**
  - `store/lifecycle.go:60-66`: `UpdateMessageContent` turns `pgx.ErrNoRows` into a generic error.
  - `store/lifecycle.go:87-93`: `SoftDeleteMessage` does the same.
  - The domain checks liveness first (`liveMessage`), so the second request to arrive passes that check and then finds no row.
- **Scenario:**
  - A double-click on "Delete", or a moderator and the sender deleting at the same moment, gives one 200 and one 500 (`"delete message: no rows in result set"`).
  - The spec says "Deleting a deleted message is 404".
  - An edit that races a delete also returns 500.
- **Fix:** map `ErrNoRows` to `domain.ErrNotFound` in both store functions, or in the domain.

### M4. `messages.quoted_message_id` (and `deleted_by`) have foreign keys but no index
- **Where:** `data/migrations/040_message_lifecycle.sql:13-14`.
- **Scenario:**
  - `ON DELETE SET NULL` makes Postgres look up referencing rows for every deleted message.
  - Workspace teardown (`workspace/internal/store/teardown.go:75`, `DELETE FROM channels` cascading to `messages`) therefore does one sequential scan of `messages` per deleted message. That is O(n²) on a large workspace and holds locks the whole time.
  - The test DB confirmed that no index on `quoted_message_id` exists.
- **Fix:** add `CREATE INDEX IF NOT EXISTS idx_messages_quoted ON messages (quoted_message_id) WHERE quoted_message_id IS NOT NULL;`. Add the same for `deleted_by`, or drop that FK (the users table rarely loses rows).

### M5. Live frames lose the new fields, and the converters are duplicated
- **Where:**
  - `grpc/hub_broadcast.go:14-27` (`messageToChatMessage`) and `domain/threads.go:217-224` (`chatMessageOf`) are two copies of the same conversion.
  - `chatAttachments` is defined twice (`domain/attachments.go:76` and `grpc/hub_broadcast.go:30`).
  - Neither copy sets `content_format`, `mentions`, `linked_entity_*`, the quote or the edited flag.
- **Scenario:**
  - A reply that quotes a message arrives live without its quote block until the next refetch. This is a phase 08 UI requirement.
  - `message_edited` carries `edited_at` separately, but its `ChatMessage` has `content_format=""`. HTML renders only thanks to the client's `startsWith('<')` heuristic, the same heuristic behind H1.
  - `ChatMessage` has no `quoted` field at all.
- **Fix:**
  - Add `QuotedMessage quoted` (and `edited_at`) to `ChatMessage`.
  - Map every field in a single converter that both packages use. Regenerate both proto sides.

### M6. Plan gap: a moderator's deletion is not logged anywhere visible
- **Where:** phase-08 requirement "xoá tin người khác ghi nhật ký nhóm (ai xoá, tin của ai, lúc nào)".
- **What exists:** only the `deleted_by` column, written at `store/lifecycle.go:88`.
- **What is missing:** no read path, no event and no audit entry exposes it. `MessageDeletedEvent` does not carry it either.
- **Scenario:** a manager removes a member's message, and nobody in the group can see who did it. The spec ("which is the channel's record of the deletion") overstates this.
- **Fix:** either emit or store a group-visible audit entry (actor display name, sender, time) or narrow the plan and spec explicitly. Per CLAUDE.md, an audit entry must name the actor.

### M7. An edit does not update `mentions`
- **Where:** `domain/lifecycle.go:37-74`, `store/lifecycle.go:58-63`.
- **Scenario:**
  - The new text adds `<span data-mention="u2">`. u2 does not follow the thread and is not notified.
  - A removed mention stays in `mentions`. That user keeps following implicitly (`threadFollowingExpr`, `ThreadFollowers`) and the client still gets the stale id.
- **Fix:**
  - Accept `mentions` on `EditMessageRequest` and the PATCH body, update the column, and apply the reply-follow rule.
  - Alternatively, document that an edit never changes mentions.

### M8. Tests promised by the plan or spec are missing
- **Search after delete.** The phase-08 risk section says "lọc `deleted_at IS NULL` trong `SearchMessages`; có test". The filter exists, but no test covers it.
- **Last reply time.** No test shows that `last_reply_at` moves back to the newest remaining reply.
- **Thread list.** No test covers redaction in `ListThreads`, or a deleted root that drops out once its last reply is deleted.
- **Attachments.** There is no deny test for a file under a trashed folder (the probe shows the behaviour is correct), and none for the legacy link (C1).
- **Mark unread.** The deny side of `MarkMessageUnread` is covered (outsider). Nothing covers the case where the caller marks their *own* reply unread, which is excluded from thread unread (`sender_id <> $1`) and so gives 0 unread.

## Low

1. **mailto links.** They get `rel="noreferrer"` only: no `target=_blank`, no `noopener` (probe: `<a href="mailto:a@b.c" rel="noreferrer">`). The spec says every link opens in a new tab with noopener noreferrer. Either fix the spec wording or the policy. The client hook adds both attributes anyway.
2. **`039_message_plain_text.sql` backfill.**
   - It decodes `&amp;` first, so `&amp;lt;` becomes `<` (double decode).
   - It ignores `&quot;` / `&#39;`.
   - It joins blocks with a space, while Go `plainText` joins them with newlines.
   - Only search is affected.
3. **039 table rewrite.** 039 rewrites `messages` under ACCESS EXCLUSIVE (DROP plus ADD of a STORED generated column, plus a full-table UPDATE). Check the production row count before deploying.
4. **Legacy HTML rows are not re-sanitized.** HTML stored before this change was never filtered by the server, and 039 does not filter it. The client's DOMPurify is the only layer for those rows.
5. **`EditMessage` reload.**
   - It reloads through `GetThread` (`lifecycle.go:65`), so editing a root loads the entire thread.
   - When `err == nil && len == 0`, it wraps nil (`"%!w(<nil>)"`).
   - Use a single-message read instead.
6. **Pages that are not enriched.** `ListPins`, `SearchMessages` and `FindThreadsByEntity` skip enrichment, so a pinned message that carries only files shows empty, with no attachments.
7. **Reactions and pins on deleted messages.** Both are still accepted, and the placeholder still returns its reactions.
8. **Inconsistent sender names.** `QuotesByID` uses `display_name` with a fallback to `username`, but message rows use `username`. The same person can appear under two names.
9. **Existence oracle.** `liveMessage` runs before authorization, so any authenticated user can tell whether a message id exists (404 vs 403). This is negligible with UUIDs.

## Edge cases checked and found correct
- **`ChannelDriveFiles`:**
  - A trashed or deleted intermediate folder stops the chain (`p.status='active'`).
  - Cycles stop at depth 64.
  - A `drive_context` claimed on the file's own row is ignored, and the root must be `is_root`, `active`, `channel` and keyed to this channel.
  - DMs also use `drive_context='channel'` (`drive/internal/domain/channel_drive.go:88`), so attachments in DMs work.
  - The workspace drive root uses context id = workspace id, which cannot collide with a channel id.
- **Quotes:**
  - Checked on write (`checkQuote`: same channel, live).
  - Re-checked on read (`attachQuotes` compares `q.ChannelID == m.ChannelID`).
  - The text of a deleted quote is blanked in SQL.
  - A deleted message's own `QuotedMessageID` is cleared by `redactIfDeleted`.
- **Redaction by read path:**
  - List, thread, thread list, `ListPins` (via `GetThread`), `FindByEntity` and quotes: redacted when scanned.
  - Search: `deleted_at IS NULL`.
  - Unread counts (channel and thread) and the counts in `ThreadFollowers`: deleted rows filtered.
  - Thread participants and the thread-state fan-out: deleted rows filtered.
  - The `message_deleted` frame carries ids only.
- **Reply count:** `reply_count` and `last_reply_at` are recomputed in the same transaction. The row lock on the root serializes this against a concurrent reply insert.
- **`MarkMessageUnread`:** moves the read position back by 1 µs (pgx and Postgres both use µs precision). Following state is kept, because there is no follow row and the implicit value is computed.
- **Callers of changed functions:**
  - gRPC `SendMessage` has no callers outside messaging. Auth's messaging client only calls `CreateChannel`. Passing `ContentFormat`/`Mentions` therefore changes nothing for existing callers.
  - Polls and tasks insert through `store.insertMessage` directly as `system` messages, so the empty-message rule does not affect them. `content_text` falls back to `content`.
  - No events consumer calls `SendMessage`.
- **Routing:** new routes under `/api/messages` are covered by the Vite proxy (`vite.config.js:16`), the Traefik rule and nginx.
- **Generated code:** `ws.ts` is regenerated and contains the new events and `ChatAttachment`. The Go `.pb.go` files contain the new messages.

## Recommended actions (priority order)
1. C1: bound the legacy `drive_file` read to the message's channel drive and active files, and validate or convert `linked_entity_*` on send. Add deny tests.
2. H1 and M2: validate `content_format`, and stop letting a non-html label carry markup.
3. H2: refuse replies to deleted roots atomically in `InsertMessage`.
4. M1 and M3: check for emptiness after sanitizing, and map `ErrNoRows` to `ErrNotFound`.
5. M4: index `quoted_message_id`.
6. M5–M8: complete the live-frame fields with one shared converter, settle the moderator-deletion log, fix mention handling on edit, and add the missing tests. Update the specs where the code is narrower than the plan (moderation log, mailto behaviour).

## Plan follow-ups (backend)
- Phase 06: the backend is mostly done. It is blocked on C1, because the deny invariant does not hold on the legacy path.
- Phase 07: the backend is done except H1, M1 and M2. The spec claim needs H1 fixed.
- Phase 08: the backend is done except H2, M3 and M6 (moderation log), plus the search-after-delete test the plan promises (M8).

## Unresolved questions
- Should the legacy `drive_file` send shape keep being accepted? The frontend still uses it (`SpaceView.tsx:151`). If not, it should be converted to `attachments` before C1 is fixed.
- Moderation log: is the `deleted_by` column considered enough, or is a visible group audit entry required, as the plan says?
- Should an edit be able to change mentions?

Status: DONE_WITH_CONCERNS
Summary: All gates pass and the new attachment CTE, quote checks and redaction paths are correct. However, the legacy `drive_file` link path exposes any drive file's metadata across channels and tenants (probe-confirmed), and the server HTML filter can be skipped by labelling content as markdown.
Concerns/Blockers: C1 and H1 should be fixed before merge. H2 and M1–M4 are correctness gaps that are cheap to fix now.
