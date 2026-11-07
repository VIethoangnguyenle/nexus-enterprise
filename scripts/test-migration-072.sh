#!/usr/bin/env bash
#
# Migration 072 numbers messages. On data from before it, main messages are numbered 1..n in
# each channel by time (then id), replies 1..k in each thread, the channel and the root keep the
# last number, and read positions become numbers from read times. After it, the trigger numbers
# what is stored next. A second run changes nothing, and it reverses from its own header.
# Works in a scratch copy of the test database.
#
# Usage: scripts/test-migration-072.sh [source-db]   (default ngac_ci; needs the compose postgres)
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${1:-ngac_ci}"
SCRATCH="ngac_scratch_072"
PG="${PG_CONTAINER:-nexus-enterprise-postgres-1}"
MIG="${MIG:-$ROOT/data/migrations/072_message_sequences.sql}"
psql_() { docker exec -i "$PG" psql -X -q -A -t -v ON_ERROR_STOP=1 -U ngac "$@"; }
q() { psql_ -d "$SCRATCH" -c "$1"; }

psql_ -d postgres -c "DROP DATABASE IF EXISTS $SCRATCH" -c "CREATE DATABASE $SCRATCH TEMPLATE $SRC"
trap 'psql_ -d postgres -c "DROP DATABASE IF EXISTS $SCRATCH" >/dev/null' EXIT

fail() { echo "✗ $*" >&2; exit 1; }
want() { local got; got="$(q "$3")"; [ "$got" = "$2" ] || fail "$1: expected '$2', got '$got'"; }
reverse_sql() { awk '/^-- Reverse/{on=1;next} on && /^--   /{l=$0; sub(/^--   /, "", l); print l; next} on{exit}' "$MIG"; }
snap() { q "SELECT md5(concat_ws('|', (SELECT string_agg(id || coalesce(seq::text,'') || coalesce(reply_seq::text,'') || last_reply_seq, ',' ORDER BY id) FROM messages),
  (SELECT string_agg(id || last_seq, ',' ORDER BY id) FROM channels),
  (SELECT string_agg(user_id || channel_id || last_read_seq, ',' ORDER BY user_id, channel_id) FROM read_receipts)))"; }

# Start from the state before 072 (the source database has it applied).
reverse_sql | psql_ -d "$SCRATCH" >/dev/null
want "the fixture starts without numbers" 0 "SELECT count(*) FROM information_schema.columns WHERE table_name = 'messages' AND column_name = 'seq'"

# One channel; ids deliberately out of time order; a thread of two replies; a read time between
# the second and third main message.
q "INSERT INTO users (id, username) VALUES ('m072-a', 'm072-a'), ('m072-b', 'm072-b')"
q "INSERT INTO workspaces (id, name, owner_id, type) VALUES ('m072-w', 'w', 'm072-a', 'organization')"
q "INSERT INTO channels (id, name, channel_type, workspace_id) VALUES ('m072-c', 'c', 'private', 'm072-w')"
q "INSERT INTO messages (id, channel_id, sender_id, content, created_at, parent_message_id) VALUES
   ('m072-z', 'm072-c', 'm072-a', 'one',   '2026-01-01 10:00', NULL),
   ('m072-x', 'm072-c', 'm072-b', 'two',   '2026-01-01 10:01', NULL),
   ('m072-y', 'm072-c', 'm072-a', 'three', '2026-01-01 10:03', NULL),
   ('m072-r2','m072-c', 'm072-b', 'r-one', '2026-01-01 10:02', 'm072-z'),
   ('m072-r1','m072-c', 'm072-a', 'r-two', '2026-01-01 10:04', 'm072-z')"
q "INSERT INTO read_receipts (user_id, channel_id, last_read_at) VALUES ('m072-b', 'm072-c', '2026-01-01 10:02')"

# --- Applies: numbers by time, read positions by time ----------------------------------------
out="$(psql_ -d "$SCRATCH" <"$MIG" 2>&1)" || fail "072 failed: $out"
want "main by time, replies by time in their thread" "m072-r1=-/2,m072-r2=-/1,m072-x=2/-,m072-y=3/-,m072-z=1/-" "SELECT string_agg(id || '=' || coalesce(seq::text, '-') || '/' || coalesce(reply_seq::text, '-'), ',' ORDER BY id) FROM messages WHERE channel_id = 'm072-c'"
want "the channel keeps its last number" 3 "SELECT last_seq FROM channels WHERE id = 'm072-c'"
want "the root keeps its thread's last number" 2 "SELECT last_reply_seq FROM messages WHERE id = 'm072-z'"
want "read at 10:02 is read up to the second message" 2 "SELECT last_read_seq FROM read_receipts WHERE user_id = 'm072-b' AND channel_id = 'm072-c'"
echo "  applied: numbered by time, read positions from read times"

# --- The trigger numbers what comes next ------------------------------------------------------
q "INSERT INTO messages (id, channel_id, sender_id, content) VALUES ('m072-n', 'm072-c', 'm072-a', 'four')"
q "INSERT INTO messages (id, channel_id, sender_id, content, parent_message_id) VALUES ('m072-r3', 'm072-c', 'm072-b', 'r-three', 'm072-z')"
want "a new main message is next in its channel" 4 "SELECT seq FROM messages WHERE id = 'm072-n'"
want "a new reply is next in its thread" 3 "SELECT reply_seq FROM messages WHERE id = 'm072-r3'"
if q "UPDATE messages SET seq = NULL WHERE id = 'm072-n'" 2>/dev/null; then fail "a main message without a number was accepted"; fi
echo "  trigger: new messages take the next numbers; a main message cannot lose its number"

# --- Code that knows only read times, and ON CONFLICT, still leave the numbers right -----------
q "UPDATE read_receipts SET last_read_at = '2026-01-01 10:01:30' WHERE user_id = 'm072-b' AND channel_id = 'm072-c'"
want "a read time moved back alone brings the read number with it" 2 "SELECT last_read_seq FROM read_receipts WHERE user_id = 'm072-b' AND channel_id = 'm072-c'"
q "UPDATE read_receipts SET last_read_at = now() WHERE user_id = 'm072-b' AND channel_id = 'm072-c'"
want "and forward to now, to the last message" 4 "SELECT last_read_seq FROM read_receipts WHERE user_id = 'm072-b' AND channel_id = 'm072-c'"
q "INSERT INTO read_receipts (user_id, channel_id, last_read_at) VALUES ('m072-a', 'm072-c', '2026-01-01 10:00:30')"
want "a receipt stored with a time only gets its number" 1 "SELECT last_read_seq FROM read_receipts WHERE user_id = 'm072-a' AND channel_id = 'm072-c'"
q "INSERT INTO messages (id, channel_id, sender_id, content) VALUES ('m072-n', 'm072-c', 'm072-a', 'again') ON CONFLICT (id) DO NOTHING"
want "a message stored twice takes no number" 4 "SELECT last_seq FROM channels WHERE id = 'm072-c'"
echo "  old writers: read times bring read numbers; a repeated message leaves no gap"

# --- A sender caught up has read their own message -------------------------------------------
q "INSERT INTO messages (id, channel_id, sender_id, content) VALUES ('m072-o', 'm072-c', 'm072-b', 'mine')"
want "the caught-up sender has read it" 5 "SELECT last_read_seq FROM read_receipts WHERE user_id = 'm072-b' AND channel_id = 'm072-c'"
want "a sender behind has not" 1 "SELECT last_read_seq FROM read_receipts WHERE user_id = 'm072-a' AND channel_id = 'm072-c'"
echo "  own messages: read by a sender who was caught up"

# --- Second run: a no-op ---------------------------------------------------------------------
BEFORE="$(snap)"
psql_ -d "$SCRATCH" <"$MIG" >/dev/null 2>&1 || fail "the second run failed"
[ "$BEFORE" = "$(snap)" ] || fail "a second run changed data"
echo "  second run: no-op"

# --- Reverse: the SQL in the header ----------------------------------------------------------
reverse_sql | psql_ -d "$SCRATCH" >/dev/null
want "after the reverse there are no numbers" 0 "SELECT count(*) FROM information_schema.columns WHERE table_name = 'messages' AND column_name IN ('seq', 'reply_seq', 'last_reply_seq')"
want "and no triggers" 0 "SELECT count(*) FROM pg_trigger WHERE tgname IN ('messages_number', 'read_receipts_number')"
psql_ -d "$SCRATCH" <"$MIG" >/dev/null 2>&1
want "and 072 applies again" 4 "SELECT seq FROM messages WHERE id = 'm072-n'"
echo "  reverse (from the header): columns and trigger gone; re-applies"

echo "✓ 072 numbers messages and read positions, numbers new messages, is a no-op the second time, and reverses cleanly"
