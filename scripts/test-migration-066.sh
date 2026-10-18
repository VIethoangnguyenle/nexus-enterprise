#!/usr/bin/env bash
#
# Migration 066 gives every existing direct message the workspace it belongs to: the one
# workspace in which both its people are members; with several, an organization before a
# personal workspace and then the oldest; with none, it stays unplaced. It never moves a DM
# that has a workspace, leaves groups alone, ignores a person who was removed, refuses to
# run past its thresholds, is a no-op the second time, and reverses from its own header.
# Builds the cases in a scratch copy of the test database, applies the file, checks each,
# re-applies it, tries the thresholds, dry-runs it, and applies the header's reverse SQL.
#
# Usage: scripts/test-migration-066.sh [source-db]   (default ngac_ci; needs the compose postgres)
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${1:-ngac_ci}"
SCRATCH="ngac_scratch_066"
PG="${PG_CONTAINER:-nexus-enterprise-postgres-1}"
MIG="${MIG:-$ROOT/data/migrations/066_dm_workspace.sql}"
psql_() { docker exec -i "$PG" psql -X -q -A -t -v ON_ERROR_STOP=1 -U ngac "$@"; }
q() { psql_ -d "$SCRATCH" -c "$1"; }

psql_ -d postgres -c "DROP DATABASE IF EXISTS $SCRATCH" -c "CREATE DATABASE $SCRATCH TEMPLATE $SRC"
trap 'psql_ -d postgres -c "DROP DATABASE IF EXISTS $SCRATCH" >/dev/null' EXIT
# The source may already have had 066 applied: start from a database that has not.
psql_ -d "$SCRATCH" -c "DROP TABLE IF EXISTS _reconcile_066_dms"
# ...and with no DMs of its own, so the counts below are the fixture's alone.
psql_ -d "$SCRATCH" -c "DELETE FROM channel_members WHERE channel_id IN (SELECT id FROM channels WHERE channel_type = 'dm')" -c "DELETE FROM channels WHERE channel_type = 'dm'"

fail() { echo "✗ $*" >&2; exit 1; }
want() { # want <description> <expected> <sql>
	local got
	got="$(q "$3")"
	[ "$got" = "$2" ] || fail "$1: expected '$2', got '$got'"
}
snap() { q "SELECT md5(coalesce(string_agg(id || ':' || channel_type || ':' || coalesce(workspace_id, '-'), ',' ORDER BY id), '')) FROM channels"; }
refuses() { # refuses <description> <message-fragment> <migration-text>: must fail with the fragment and change nothing
	local before after out
	before="$(snap)"
	out="$(printf '%s' "$3" | psql_ -d "$SCRATCH" 2>&1 || true)"
	echo "$out" | grep -q -- "$2" || fail "$1: did not refuse with '$2': $out"
	after="$(snap)"
	[ "$before" = "$after" ] || fail "$1: refused but changed data"
	echo "  refused: $1"
}
ws_of() { q "SELECT coalesce(workspace_id, '-') FROM channels WHERE id = '$1'"; }

# --- Fixture ---------------------------------------------------------------
# Workspaces: o1 organization (2020), o2 organization (2021), o3 organization (2022),
#             pOld personal (2019, the oldest of all), pNew personal (2023).
# People (all active unless said): pairs below. Nodes: users.ngac_node is "n-<user>";
# tenant_users.ngac_node_id is "t-<user>-<ws>" unless NULL.
psql_ -d "$SCRATCH" <<'SQL'
INSERT INTO ngac_nodes (id, name, node_type)
SELECT 'n-' || u, 'n-' || u, 'U' FROM unnest(ARRAY['a1','a2','b1','b2','c1','c2','d1','d2','e1','e2','f1','f2','g1','g2','h1','h2','i1','j1','k1','k2']) u;
INSERT INTO ngac_nodes (id, name, node_type)
SELECT 't-' || u || '-' || w, 't-' || u || '-' || w, 'U'
FROM unnest(ARRAY['a1','a2','b1','b2','c1','c2','d1','d2','e1','e2','f1','f2','g1','g2','h1','h2','k1','k2']) u, unnest(ARRAY['o1','o2','o3','pOld','pNew']) w;

INSERT INTO users (id, username, ngac_node, email)
SELECT 'u-' || u, 'm066-' || u, 'n-' || u, 'm066-' || u || '@example.test'
FROM unnest(ARRAY['a1','a2','b1','b2','c1','c2','d1','d2','e1','e2','f1','f2','g1','g2','h1','h2','i1','j1','k1','k2']) u;

INSERT INTO workspaces (id, name, owner_id, type, created_at) VALUES
 ('o1', 'm066 o1', 'u-a1', 'organization', '2020-01-01'),
 ('o2', 'm066 o2', 'u-a1', 'organization', '2021-01-01'),
 ('o3', 'm066 o3', 'u-a1', 'organization', '2022-01-01'),
 ('pOld', 'm066 pOld', 'u-a1', 'personal', '2019-01-01'),
 ('pNew', 'm066 pNew', 'u-a1', 'personal', '2023-01-01');

-- membership: user -> workspaces (status active unless the status column below says otherwise)
CREATE TEMP TABLE m (u text, w text, st text DEFAULT 'active', own_node boolean DEFAULT true);
INSERT INTO m (u, w) VALUES
 ('a1','o1'),('a2','o1'),                                  -- A: one shared (o1)
 ('b1','o1'),('b1','pOld'),('b2','o1'),('b2','pOld'),       -- B: org o1 + personal pOld (pOld is older): org wins
 ('c1','o2'),('c1','o3'),('c2','o2'),('c2','o3'),           -- C: two organizations: the older, o2
 ('d1','o1'),('d2','o2'),                                  -- D: none shared
 ('e1','o1'),('e2','o1'),('e2','o3'),('e1','o3'),           -- E: o1 and o3; e2 is removed in o1: only o3 counts
 ('f1','pOld'),('f2','pOld'),('f1','pNew'),('f2','pNew'),   -- F: two personal: the older, pOld
 ('g1','o1'),('g2','o1'),                                  -- G: recorded by the users.ngac_node
 ('h1','o1'),('h2','o1'),                                  -- H: already has a workspace (o2): untouched
 ('k1','o2'),('k2','o2');                                  -- K: a group, untouched
UPDATE m SET st = 'removed' WHERE u = 'e2' AND w = 'o1';
INSERT INTO tenant_users (tenant_id, user_id, status, ngac_node_id)
SELECT w, 'u-' || u, st, 't-' || u || '-' || w FROM m;

-- channels
INSERT INTO channels (id, name, channel_type, workspace_id) VALUES
 ('dm-A','dm','dm',NULL),('dm-B','dm','dm',NULL),('dm-C','dm','dm',NULL),('dm-D','dm','dm',NULL),
 ('dm-E','dm','dm',NULL),('dm-F','dm','dm',NULL),('dm-G','dm','dm',NULL),('dm-H','dm','dm','o2'),
 ('dm-half','dm','dm',NULL),('dm-empty','dm','dm',NULL),
 ('grp-K','group','private',NULL);
-- A person is recorded by the node they acted as: their workspace node (a tenant_users row,
-- which names the one workspace) or their own users.ngac_node (which names every workspace
-- they are a member of). A by workspace nodes, D by one of each; B, C, E, F, G by their own.
INSERT INTO channel_members (channel_id, ngac_node_id) VALUES
 ('dm-A','t-a1-o1'),('dm-A','t-a2-o1'),
 ('dm-B','n-b1'),('dm-B','n-b2'),
 ('dm-C','n-c1'),('dm-C','n-c2'),
 ('dm-D','n-d1'),('dm-D','t-d2-o2'),
 ('dm-E','n-e1'),('dm-E','n-e2'),
 ('dm-F','n-f1'),('dm-F','n-f2'),
 ('dm-G','n-g1'),('dm-G','n-g2'),
 ('dm-H','t-h1-o1'),('dm-H','t-h2-o1'),
 ('dm-half','t-i1-o1'),
 ('grp-K','t-k1-o2'),('grp-K','t-k2-o2');
SQL
BEFORE="$(snap)"
want "the fixture has no backup table" 0 "SELECT count(*) FROM pg_tables WHERE tablename = '_reconcile_066_dms'"
want "9 DMs wait for a workspace" 9 "SELECT count(*) FROM channels WHERE channel_type = 'dm' AND workspace_id IS NULL AND id LIKE 'dm-%'"

RUNMIG="$(cat "$MIG")"
with_limits() { printf '%s' "$RUNMIG" | sed "/^INSERT INTO _m066_limits VALUES/s/VALUES (.*);\$/VALUES ($1, $2);/"; }
LIMITS_LINE="$(grep -c '^INSERT INTO _m066_limits VALUES (10, 2);$' "$MIG")"
[ "$LIMITS_LINE" = 1 ] || fail "the thresholds line is not 'INSERT INTO _m066_limits VALUES (10, 2);'"
ALLOWED="$(with_limits 10 3)"
[ "$ALLOWED" != "$RUNMIG" ] || fail "could not set the thresholds"

# --- Guards: thresholds refuse, changing nothing ---------------------------------------
refuses "the production threshold on unplaced DMs (2) is below the fixture's 3" "DMs cannot be placed, more than the threshold 2" "$RUNMIG"
refuses "more DMs would change than the threshold" "DMs would change, more than the threshold 5" "$(with_limits 5 100)"
[ "$BEFORE" = "$(snap)" ] || fail "the guards changed data"

# --- Dry run: COMMIT turned into ROLLBACK ------------------------------------------------
out="$(printf '%s' "${ALLOWED//$'\nCOMMIT;'/$'\nROLLBACK;'}" | psql_ -d "$SCRATCH" 2>&1)"
echo "$out" | grep -q "migration 066: 9 DMs without a workspace" || fail "the dry run did not report its counts: $out"
[ "$BEFORE" = "$(snap)" ] || fail "the dry run changed data"
want "the dry run left no backup table" 0 "SELECT count(*) FROM pg_tables WHERE tablename = '_reconcile_066_dms'"
echo "  dry run: $(echo "$out" | grep 'migration 066: 9 DMs' | sed 's/.*migration 066: //')"

# --- Apply -----------------------------------------------------------------------------
out="$(printf '%s' "$ALLOWED" | psql_ -d "$SCRATCH" 2>&1)" || fail "066 failed: $out"
echo "$out" | grep -q "9 DMs without a workspace; 6 to place (3 in exactly one shared workspace, 3 in several); 3 left unplaced" || fail "unexpected counts: $out"
echo "$out" | grep -q "left without a workspace (they appear in no workspace): dm-D, dm-empty, dm-half" || fail "the unplaced DMs were not reported: $out"
AFTER="$(snap)"
[ "$AFTER" != "$BEFORE" ] || fail "066 changed nothing"

check_applied() {
	want "A: the one shared workspace"                 o1   "SELECT workspace_id FROM channels WHERE id = 'dm-A'"
	want "B: an organization before an older personal" o1   "SELECT workspace_id FROM channels WHERE id = 'dm-B'"
	want "C: of two organizations, the older"          o2   "SELECT workspace_id FROM channels WHERE id = 'dm-C'"
	want "D: no shared workspace stays unplaced"       -    "SELECT coalesce(workspace_id, '-') FROM channels WHERE id = 'dm-D'"
	want "E: a removed member does not count"          o3   "SELECT workspace_id FROM channels WHERE id = 'dm-E'"
	want "F: of two personal workspaces, the older"    pOld "SELECT workspace_id FROM channels WHERE id = 'dm-F'"
	want "G: people recorded by their own node"        o1   "SELECT workspace_id FROM channels WHERE id = 'dm-G'"
	want "H: a DM that has a workspace is not moved"   o2   "SELECT workspace_id FROM channels WHERE id = 'dm-H'"
	want "a DM with one person stays unplaced"         -    "SELECT coalesce(workspace_id, '-') FROM channels WHERE id = 'dm-half'"
	want "a DM with nobody stays unplaced"             -    "SELECT coalesce(workspace_id, '-') FROM channels WHERE id = 'dm-empty'"
	want "a group is left alone"                       -    "SELECT coalesce(workspace_id, '-') FROM channels WHERE id = 'grp-K'"
	want "the backup holds the 6 changed rows" "dm-A:-:o1,dm-B:-:o1,dm-C:-:o2,dm-E:-:o3,dm-F:-:pOld,dm-G:-:o1" \
		"SELECT string_agg(channel_id || ':' || coalesce(old_workspace_id, '-') || ':' || new_workspace_id, ',' ORDER BY channel_id) FROM _reconcile_066_dms"
}
check_applied
echo "  applied: one shared -> it; several -> organization before personal, then oldest; removed member ignored; none/one/empty unplaced; assigned DM and group untouched"

# --- Second run: nothing to place -------------------------------------------------------
BACKUPS="$(q "SELECT count(*) FROM _reconcile_066_dms")"
out="$(printf '%s' "$ALLOWED" | psql_ -d "$SCRATCH" 2>&1)" || fail "the second run failed: $out"
echo "$out" | grep -q "3 DMs without a workspace; 0 to place (0 in exactly one shared workspace, 0 in several); 3 left unplaced" || fail "the second run was not a no-op: $out"
[ "$AFTER" = "$(snap)" ] || fail "a second run changed data"
[ "$BACKUPS" = "$(q "SELECT count(*) FROM _reconcile_066_dms")" ] || fail "a second run added backup rows"
check_applied
echo "  second run: 0 rows, no backup rows"

# --- Reverse: the SQL in the migration's header restores where we started ----------------
REVERSE="$(awk '/^--   BEGIN;$/{on=1} on{l=$0; sub(/^--   ?/, "", l); print l} /^--   COMMIT;$/{on=0}' "$MIG")"
case "$REVERSE" in BEGIN\;*COMMIT\;) ;; *) fail "could not extract the reverse SQL from the header" ;; esac
printf '%s\n' "$REVERSE" | psql_ -d "$SCRATCH" >/dev/null
want "after the reverse every DM is unplaced again, as the fixture had them, H aside" 9 \
	"SELECT count(*) FROM channels WHERE channel_type = 'dm' AND workspace_id IS NULL AND id LIKE 'dm-%'"
want "H keeps its workspace after the reverse" o2 "SELECT workspace_id FROM channels WHERE id = 'dm-H'"
[ "$BEFORE" = "$(snap)" ] || fail "the reverse did not restore the original rows"
echo "  reverse (from the header): every DM is back to no workspace"

# --- Applying again after the reverse works -----------------------------------------------
printf '%s' "$ALLOWED" | psql_ -d "$SCRATCH" >/dev/null
check_applied
# --- A DM that becomes placeable later is placed on the next run ------------------------
q "INSERT INTO tenant_users (tenant_id, user_id, status, ngac_node_id) VALUES ('o2', 'u-d1', 'active', 't-d1-o2')"
printf '%s' "$ALLOWED" | psql_ -d "$SCRATCH" >/dev/null
want "D: placed once both are in a workspace" o2 "SELECT workspace_id FROM channels WHERE id = 'dm-D'"

echo "✓ 066 gives each DM the workspace both its people are in (organization before personal, then oldest), leaves what it cannot place and what already has a workspace alone, refuses past its thresholds, is a no-op the second time, and reverses cleanly"
