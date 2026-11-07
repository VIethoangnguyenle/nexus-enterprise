# Code review — feat/message-unread-seq (2026-11-06)

Reviewer: code-reviewer subagent, read-only. No message text reaches a frame. What was found, and what was done.

| # | Finding | Done |
|---|---------|------|
| H1 | Code of the previous release (serving during deploy, or after a rollback) moves only `last_read_at`, leaving `last_read_seq` stale; its `ON CONFLICT DO NOTHING` leaves a gap | Fixed in 072: trigger `read_receipts_number` derives the number from the time when only the time moves; `messages_number` skips an id already stored before taking a number. Covered by `scripts/test-migration-072.sh` |
| M1 | 072 holds `messages` exclusively for its backfill with no `lock_timeout`; the extra `LOCK` was redundant | Fixed: `SET LOCAL lock_timeout = '5s'`, `LOCK` removed, header states the lock |
| M2 | A failed `CREATE INDEX CONCURRENTLY` leaves an INVALID index that `IF NOT EXISTS` then skips, and the runner records 073 | Fixed: 073 drops invalid copies first and ends with a check that raises unless all three are valid |
| M3 | Frames trust the `channel_members` cache, which can outlive the NGAC membership | Accepted as push does; stated in `docs/specs/chat-unread-counts` (frames carry ids only) |
| M4 | A member list read before a removal and stored after it brings the removed person back for 30 s | Fixed: removal epoch; such a read is used once, not kept. Test `TestActivity_AListReadBeforeARemovalIsNotKept` |
| M5 | Every instance reads member lists for every workspace; publish reads the DB on the send request | Fixed: delivery returns early without local sessions of the workspace; publishing is queued to the activity loop |
| M6 | Client ignores the reader's own messages, server counted them | Fixed on the server: a caught-up sender has read their own message (trigger). Test `TestUnreadMain_OwnMessageIsReadByASenderWhoWasCaughtUp` |
| M7 | Mentions compared times while the main count compares numbers | Fixed: a mention is unread when numbered after the read number, or added by an edit after the read time |
| L1 | Unsynchronised writes in `SetChannelDirectory` | Fixed: state made in `NewHub`, directory set under the lock |
| L2 | A failed member read is not remembered | Not changed: `channels.workspace_id` is NOT NULL since 067 |
| L3 | Live messages had no `seq`, so a trimmed list of live messages had no cursor | Fixed: `ChatMessage.seq`, `reply_seq` |
| L4 | Gap detection skipped when `last_seq` is unknown | Fixed: a number above `known + 1` is a gap whatever `known` is |
| L5 | Out-of-order frames from several instances cause a refetch | Not changed: one instance in production; noted for multi-instance |
| L6 | `getWholeThread` reads pages one after another; an old bundle sees only 200 replies | Not changed: same memory as before; the update prompt replaces old bundles |
| L7 | `UpsertReadReceipt` is a silent no-op for a missing channel | Not changed: the caller checks the channel's access first |
