#!/usr/bin/env bash
#
# Migration 067 makes channels.workspace_id NOT NULL. It applies on clean data, refuses
# (changing nothing) while a channel with no workspace exists, is a no-op the second time,
# leaves the foreign key in place, and reverses from its own header.
# Works in a scratch copy of the test database.
#
# Usage: scripts/test-migration-067.sh [source-db]   (default ngac_ci; needs the compose postgres)
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${1:-ngac_ci}"
SCRATCH="ngac_scratch_067"
PG="${PG_CONTAINER:-nexus-enterprise-postgres-1}"
MIG="${MIG:-$ROOT/data/migrations/067_channels_workspace_not_null.sql}"
psql_() { docker exec -i "$PG" psql -X -q -A -t -v ON_ERROR_STOP=1 -U ngac "$@"; }
q() { psql_ -d "$SCRATCH" -c "$1"; }

psql_ -d postgres -c "DROP DATABASE IF EXISTS $SCRATCH" -c "CREATE DATABASE $SCRATCH TEMPLATE $SRC"
trap 'psql_ -d postgres -c "DROP DATABASE IF EXISTS $SCRATCH" >/dev/null' EXIT

fail() { echo "✗ $*" >&2; exit 1; }
want() { local got; got="$(q "$3")"; [ "$got" = "$2" ] || fail "$1: expected '$2', got '$got'"; }
nullable() { q "SELECT is_nullable FROM information_schema.columns WHERE table_name = 'channels' AND column_name = 'workspace_id'"; }
snap() { q "SELECT md5(coalesce(string_agg(id || ':' || channel_type || ':' || coalesce(workspace_id, '-'), ',' ORDER BY id), '')) FROM channels"; }

# Start from the state before 067, with no channels of its own to get in the way.
q "ALTER TABLE channels ALTER COLUMN workspace_id DROP NOT NULL"
q "DELETE FROM channel_members WHERE channel_id IN (SELECT id FROM channels WHERE workspace_id IS NULL)"
q "DELETE FROM channels WHERE workspace_id IS NULL"
want "the fixture starts nullable" YES "SELECT is_nullable FROM information_schema.columns WHERE table_name = 'channels' AND column_name = 'workspace_id'"

# --- A NULL-workspace channel: refused, nothing changes -----------------------------------
q "INSERT INTO channels (id, name, channel_type, workspace_id) VALUES ('m067-dm1', 'dm', 'dm', NULL), ('m067-dm2', 'dm', 'dm', NULL), ('m067-g1', 'g', 'private', NULL)"
BEFORE="$(snap)"
out="$(psql_ -d "$SCRATCH" <"$MIG" 2>&1 || true)"
echo "$out" | grep -q "3 channel(s) have no workspace (dm=2, private=1)" || fail "did not refuse with the counts by type: $out"
[ "$(nullable)" = YES ] || fail "refused but changed the column"
[ "$BEFORE" = "$(snap)" ] || fail "refused but changed data"
echo "  refused: 3 channels with no workspace (dm=2, private=1)"
q "DELETE FROM channels WHERE id LIKE 'm067-%'"

# --- Clean data: applies -------------------------------------------------------------------
out="$(psql_ -d "$SCRATCH" <"$MIG" 2>&1)" || fail "067 failed: $out"
echo "$out" | grep -q "migration 067: channels.workspace_id is NOT NULL" || fail "no NOTICE: $out"
want "the column is NOT NULL" NO "SELECT is_nullable FROM information_schema.columns WHERE table_name = 'channels' AND column_name = 'workspace_id'"
want "the foreign key to workspaces is still there" 1 \
	"SELECT count(*) FROM information_schema.table_constraints c JOIN information_schema.key_column_usage k USING (constraint_name, table_schema) WHERE c.table_name = 'channels' AND c.constraint_type = 'FOREIGN KEY' AND k.column_name = 'workspace_id'"
if q "INSERT INTO channels (id, name, channel_type, workspace_id) VALUES ('m067-x', 'x', 'dm', NULL)" 2>/dev/null; then fail "the database accepted a channel with no workspace"; fi
echo "  applied: NOT NULL, a channel without a workspace is refused by the database"

# --- Second run: a no-op -------------------------------------------------------------------
AFTER="$(snap)"
psql_ -d "$SCRATCH" <"$MIG" >/dev/null || fail "the second run failed"
[ "$AFTER" = "$(snap)" ] || fail "a second run changed data"
want "still NOT NULL after a second run" NO "SELECT is_nullable FROM information_schema.columns WHERE table_name = 'channels' AND column_name = 'workspace_id'"
echo "  second run: no-op"

# --- Reverse: the SQL in the header --------------------------------------------------------
REVERSE="$(awk '/^-- Reverse:$/{on=1;next} on && /^--   /{l=$0; sub(/^--   /, "", l); print l; next} on{exit}' "$MIG")"
case "$REVERSE" in ALTER\ TABLE*DROP\ NOT\ NULL\;) ;; *) fail "could not extract the reverse SQL: '$REVERSE'" ;; esac
printf '%s\n' "$REVERSE" | psql_ -d "$SCRATCH" >/dev/null
want "after the reverse the column is nullable" YES "SELECT is_nullable FROM information_schema.columns WHERE table_name = 'channels' AND column_name = 'workspace_id'"
psql_ -d "$SCRATCH" <"$MIG" >/dev/null
want "and 067 applies again" NO "SELECT is_nullable FROM information_schema.columns WHERE table_name = 'channels' AND column_name = 'workspace_id'"
echo "  reverse (from the header): nullable again; re-applies"

echo "✓ 067 makes channels.workspace_id NOT NULL, refuses while a channel has no workspace, is a no-op the second time, and reverses cleanly"
