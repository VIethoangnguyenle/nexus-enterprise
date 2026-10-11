# Edit-added mentions are unread until read past

## Design
- New table `message_mention_times(message_id, user_id, mentioned_at)` (migration `data/migrations/049_edit_mention_times.sql`). Rows exist ONLY for people an edit added; a person mentioned at send time has no row, so their mention dates from `messages.created_at` (no backfill needed or possible: history cannot tell who an edit added).
- Effective mention time = `GREATEST(m.created_at, mt.mentioned_at)` (NULL ignored). Unread = that time > read position. Same position rules as before: main message vs channel `read_receipts.last_read_at`; reply vs thread `last_read_at` (root.created_at if never read). Channel `mention_count` stays main-only.
- `UpdateMessageContent` (same tx, row locked): after computing `added = new - old`, `recordMentionTimes` deletes rows of anyone no longer in the list, upserts `now()` for the added (non-user ids dropped by `SELECT FROM users`). Removal stops counting (array no longer contains the user + row deleted); re-adding re-dates it. Edit with `mentions` absent (nil) touches nothing.
- Deleted message: queries already require `deleted_at IS NULL`; FK `ON DELETE CASCADE` on message and user. Muted/off: unchanged (mentions always count). Access: counts/feed only over caller-readable channels (domain), unchanged; no-read test added.
- `GetUnreadCounts`: `mention_count` moved from a FILTER over the unread-main-messages join (created_at based) to a `mention_unread` CTE using the effective time. `unread_count` untouched (an edit does not make a message count as unread, only the mention).

## Files
- data/migrations/049_edit_mention_times.sql (new)
- backend/services/messaging/internal/store/{lifecycle.go, stars.go, reactions_pins_receipts.go}
- tests: store/edit_mention_unread_test.go (new, 6: add->unread->read clears; removal stops + re-add; no-op edits; deleted; reply/thread position; sender self), domain/stars_test.go (+2: end-to-end via EditMessage incl. muted, deny without read, read-past, remove, delete; unmentioned gets none), rest/stars_test.go (+1: unread-count, feed `unread`, `/channels/unread` mention_count, deny, read clears)
- docs/specs/{chat-shortcuts, chat-message-lifecycle, notifications}/spec.md (rule + scenarios)
- No frontend, no other services, no proto, nothing committed, index untouched, nothing restarted.

## TDD
Store tests written first and failed (Not equal) before the SQL change; green after.

## Migration / DB
- Backup: `~/backups/nexus/ngac-before-049-20261011120949.sql.gz` (662 KB -> 670 KB file listing shows 670866 B).
- `make db-migrate` applied `049_edit_mention_times.sql` on dev DB `ngac`; also applied by hand to test DB `ngac_chat_mine` (harness does not migrate; same container, port 5433).
- `\d message_mention_times` on dev: columns message_id/user_id TEXT NOT NULL, mentioned_at timestamptz NOT NULL default now(); PK (message_id,user_id); idx_message_mention_times_user(user_id); FKs to messages and users ON DELETE CASCADE. 0 rows (no backfill).
- Rolled-back tx on dev DB: inserted 1 row, deleting its message cascaded it to 0, rollback left 0 rows.

## Gates
`go vet ./...` ok; `go test ./...` in messaging all ok (domain, events, grpc, rest, store); `make check-layering` ok; `make build-check` ok; gofmt clean.

## Notes
- Running dev messaging (8183) not restarted: it needs a restart to serve the new queries (table already exists, old binary just ignores it).
- Frontend: no request change; `mention_count`, `unread` and `count` now simply include edit-added mentions. Feed order is still by message `created_at`, so an edit-added mention on an old message sits in its old position in the feed (badge/unread flag is correct).

Status: DONE
Summary: A per-person `message_mention_times` row (set by edits that add someone, dropped on removal, cascaded on delete) now dates those mentions from the edit, and the unread badge, feed `unread` and channel `mention_count` use it against the existing read positions. Migration 049 applied and verified on dev and test DBs, tests added at store/domain/REST levels, specs updated, all gates green.
