#!/usr/bin/env bash
#
# Migration 061 removes what the old AddMember let into a group (a person of another
# workspace, someone detached from the workspace, a UA) and the cache rows that go with
# it, and leaves everyone else alone. Builds a small graph in a scratch copy of the test
# database, checks the bad state is there, applies 061, checks who was removed and who
# stayed, then re-applies it (a no-op) and applies it once more with a lowered threshold
# (which must refuse and change nothing).
#
# Usage: scripts/test-migration-061.sh [source-db]   (default ngac_ci; needs docker compose postgres)
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${1:-ngac_ci}"
SCRATCH="ngac_scratch_061"
PG="${PG_CONTAINER:-nexus-enterprise-postgres-1}"
MIG="${MIG:-$ROOT/data/migrations/061_channel_membership_reconcile.sql}"
psql_() { docker exec -i "$PG" psql -X -q -A -t -v ON_ERROR_STOP=1 -U ngac "$@"; }
q() { psql_ -d "$SCRATCH" -c "$1"; }

psql_ -d postgres -c "DROP DATABASE IF EXISTS $SCRATCH" -c "CREATE DATABASE $SCRATCH TEMPLATE $SRC"
trap 'psql_ -d postgres -c "DROP DATABASE IF EXISTS $SCRATCH" >/dev/null' EXIT

fail() { echo "✗ $*" >&2; exit 1; }
want() { # want <description> <expected> <sql>
	local got
	got="$(q "$3")"
	[ "$got" = "$2" ] || fail "$1: expected $2, got $got"
}

# --- Fixture ---------------------------------------------------------------
# Workspace a (the one under test) and workspace b (another tenant). In a: member M,
# owner O (under Owners, which is under Members), detached D (in no UA of a), foreign F
# (a member of b). Groups: #general (type workspace) and p (private), each with a
# Members UA; a DM with D in it.
psql_ -d "$SCRATCH" <<'SQL'
INSERT INTO ngac_nodes (id, name, node_type) VALUES
 ('t-pc-a','PC_t061a','PC'),('t-mem-a','t061a_Members','UA'),('t-own-a','t061a_Owners','UA'),('t-chn-a','t061a_Channels','OA'),
 ('t-pc-b','PC_t061b','PC'),('t-mem-b','t061b_Members','UA'),('t-own-b','t061b_Owners','UA'),('t-chn-b','t061b_Channels','OA'),
 ('t-M','U_t061_M','U'),('t-O','U_t061_O','U'),('t-D','U_t061_D','U'),('t-F','U_t061_F','U'),
 ('t-gua','Ch_t061g_Members','UA'),('t-goa','Ch_t061g_Content','OA'),
 ('t-pua','Ch_t061p_Members','UA'),('t-poa','Ch_t061p_Content','OA'),
 ('t-dua','Ch_t061d_Members','UA'),('t-doa','Ch_t061d_Content','OA');
INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES
 ('a1','t-mem-a','t-pc-a'),('a2','t-own-a','t-pc-a'),('a3','t-chn-a','t-pc-a'),('a4','t-own-a','t-mem-a'),
 ('a5','t-mem-b','t-pc-b'),('a6','t-own-b','t-pc-b'),('a7','t-chn-b','t-pc-b'),('a8','t-own-b','t-mem-b'),
 ('a9','t-M','t-mem-a'),('a10','t-O','t-own-a'),('a11','t-F','t-mem-b'),
 ('a12','t-gua','t-pc-a'),('a13','t-pua','t-pc-a'),('a14','t-dua','t-pc-a'),
 -- group edges: M and O are members of a; D was detached; F belongs to b; a UA of b sits under p
 ('e1','t-M','t-gua'),('e2','t-D','t-gua'),('e3','t-F','t-gua'),
 ('e4','t-M','t-pua'),('e5','t-O','t-pua'),('e6','t-D','t-pua'),('e7','t-F','t-pua'),('e8','t-mem-b','t-pua'),
 ('e9','t-D','t-dua');
INSERT INTO users (id, username, password, ngac_node, display_name) VALUES
 ('t-uO','t061_O','','t-O','O'),('t-uM','t061_M','','t-M','M'),('t-uD','t061_D','','t-D','D'),('t-uF','t061_F','','t-F','F');
INSERT INTO workspaces (id, name, owner_id, ngac_pc_id, type) VALUES
 ('t061a','t061a','t-uO','t-pc-a','organization'),('t061b','t061b','t-uF','t-pc-b','organization');
INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, ngac_ua_id, created_by) VALUES
 ('t-cg','general','workspace','t061a','t-goa','t-gua','t-uO'),
 ('t-cp','team','private','t061a','t-poa','t-pua','t-uO'),
 ('t-cd','dm','dm','t061a','t-doa','t-dua','t-uO');
INSERT INTO channel_members (channel_id, ngac_node_id) VALUES
 ('t-cg','t-M'),('t-cg','t-D'),('t-cg','t-F'),
 ('t-cp','t-M'),('t-cp','t-O'),('t-cp','t-D'),('t-cp','t-F'),
 ('t-cp','t-orphan'),   -- a cache row with no edge in the graph
 ('t-cd','t-D');
SQL

# The guard: a workspace without its ID-keyed nodes (059 not run) stops the migration.
psql_ -d "$SCRATCH" -c "INSERT INTO workspaces (id, name, owner_id, ngac_pc_id, type) VALUES ('t061-legacy', 'legacy', 't-uO', 't-pc-a', 'organization')" >/dev/null
out="$(psql_ -d "$SCRATCH" <"$MIG" 2>&1 || true)"
echo "$out" | grep -q "lack PC/Members/Owners/Channels nodes" || fail "061 ran with a workspace that lacks its nodes: $out"
echo "  guard: refused while a workspace lacks its nodes"
# Test leftovers in the copied database that the guard would also stop on are not this
# test's business: give those workspaces their nodes (no edges, so no group is affected).
psql_ -d "$SCRATCH" -c "DELETE FROM workspaces WHERE id = 't061-legacy'" >/dev/null
psql_ -d "$SCRATCH" <<'SQL' >/dev/null
INSERT INTO ngac_nodes (id, name, node_type)
SELECT gen_random_uuid()::text, x.name, x.t FROM workspaces w
CROSS JOIN LATERAL (VALUES ('PC_' || w.id, 'PC'), (w.id || '_Members', 'UA'), (w.id || '_Owners', 'UA'), (w.id || '_Channels', 'OA')) AS x(name, t)
WHERE NOT EXISTS (SELECT 1 FROM ngac_nodes n WHERE n.name = x.name AND n.node_type = x.t);
SQL

# The guard also wants the Members UA to sit under the PC: a node that only carries the
# name does not count (policyclient.MembersUA refuses it too).
psql_ -d "$SCRATCH" <<'SQL' >/dev/null
INSERT INTO ngac_nodes (id, name, node_type) VALUES
 ('t-pc-c','PC_t061c','PC'),('t-mem-c','t061c_Members','UA'),('t-own-c','t061c_Owners','UA'),('t-chn-c','t061c_Channels','OA');
INSERT INTO workspaces (id, name, owner_id, ngac_pc_id, type) VALUES ('t061c', 't061c', 't-uO', 't-pc-c', 'organization');
SQL
out="$(psql_ -d "$SCRATCH" <"$MIG" 2>&1 || true)"
echo "$out" | grep -q "lack PC/Members/Owners/Channels nodes.*t061c" || fail "061 ran with a Members UA that is not under its PC: $out"
echo "  guard: refused while a Members UA is not under its PC"
psql_ -d "$SCRATCH" -c "DELETE FROM workspaces WHERE id = 't061c'" >/dev/null

# The workspace-membership test the services now use, as SQL: who is under <ws>_Members.
under_members() {
	q "WITH RECURSIVE b(n) AS (SELECT s.child_id FROM ngac_assignments s WHERE s.parent_id = 't-mem-a' UNION SELECT s.child_id FROM b JOIN ngac_assignments s ON s.parent_id = b.n) SELECT count(*) FROM b WHERE n = '$1'"
}
edge() { q "SELECT count(*) FROM ngac_assignments WHERE child_id = '$1' AND parent_id = '$2'"; }
cache() { q "SELECT count(*) FROM channel_members WHERE channel_id = '$1' AND ngac_node_id = '$2'"; }

assert_cleaned() {
	# Removed: the person of another workspace, the detached person, the foreign UA; their cache rows; the orphan.
	want "F kept in #general"        0 "SELECT count(*) FROM ngac_assignments WHERE id IN ('e3')"
	want "D kept in #general"        0 "SELECT count(*) FROM ngac_assignments WHERE id IN ('e2')"
	want "F kept in team"            0 "SELECT count(*) FROM ngac_assignments WHERE id IN ('e7')"
	want "D kept in team"            0 "SELECT count(*) FROM ngac_assignments WHERE id IN ('e6')"
	want "b's Members UA kept in team" 0 "SELECT count(*) FROM ngac_assignments WHERE id IN ('e8')"
	want "cache D@general"           0 "SELECT count(*) FROM channel_members WHERE channel_id='t-cg' AND ngac_node_id IN ('t-D','t-F')"
	want "cache D,F@team + orphan"   0 "SELECT count(*) FROM channel_members WHERE channel_id='t-cp' AND ngac_node_id IN ('t-D','t-F','t-orphan')"
	# Kept: members (an owner included), and the DM, which is not this migration's to touch.
	want "M lost #general"           1 "SELECT count(*) FROM ngac_assignments WHERE id = 'e1'"
	want "M lost team"               1 "SELECT count(*) FROM ngac_assignments WHERE id = 'e4'"
	want "O lost team"               1 "SELECT count(*) FROM ngac_assignments WHERE id = 'e5'"
	want "DM edge changed"           1 "SELECT count(*) FROM ngac_assignments WHERE id = 'e9'"
	want "cache M@general"           1 "SELECT count(*) FROM channel_members WHERE channel_id='t-cg' AND ngac_node_id='t-M'"
	want "cache M,O@team"            2 "SELECT count(*) FROM channel_members WHERE channel_id='t-cp' AND ngac_node_id IN ('t-M','t-O')"
	want "cache DM"                  1 "SELECT count(*) FROM channel_members WHERE channel_id='t-cd'"
}

# Red: before the migration the bad state is there, and the assertions notice it.
[ "$(under_members t-M)" = 1 ] && [ "$(under_members t-O)" = 1 ] || fail "fixture: M and O must be under t061a_Members"
[ "$(under_members t-D)" = 0 ] && [ "$(under_members t-F)" = 0 ] || fail "fixture: D and F must not be under t061a_Members"
[ "$(edge t-D t-gua)" = 1 ] && [ "$(edge t-F t-gua)" = 1 ] && [ "$(edge t-mem-b t-pua)" = 1 ] || fail "fixture: the bad edges must exist before 061"
if (assert_cleaned) >/dev/null 2>&1; then fail "the assertions pass before 061 ran: they prove nothing"; fi
echo "  before 061: D and F sit in #general and team, b's Members UA sits under team (assertions fail as they should)"

# A lowered threshold refuses and changes nothing.
snap() { q "SELECT md5(string_agg(id, ',' ORDER BY id)) FROM ngac_assignments" ; q "SELECT md5(string_agg(channel_id || ngac_node_id, ',' ORDER BY channel_id, ngac_node_id)) FROM channel_members"; }
before="$(snap)"
out="$(sed 's/VALUES (50, 200)/VALUES (1, 200)/' "$MIG" | psql_ -d "$SCRATCH" 2>&1 || true)"
echo "$out" | grep -q "group edges would be removed (limit 1)" || fail "061 did not refuse on its threshold: $out"
[ "$before" = "$(snap)" ] || fail "061 changed data although its threshold refused it"
echo "  threshold: refused with a limit of 1 edge, nothing changed"

# Green: apply.
psql_ -d "$SCRATCH" <"$MIG" >/dev/null
assert_cleaned
want "assignment backups" 5 "SELECT count(*) FROM _reconcile_061_assignments"
want "cache backups"      5 "SELECT count(*) FROM _reconcile_061_channel_members"
echo "  applied: 5 edges and 5 cache rows removed and backed up; members, owners and the DM untouched"

# Second run: nothing to do.
after="$(snap)"
psql_ -d "$SCRATCH" <"$MIG" >/dev/null
[ "$after" = "$(snap)" ] || fail "a second run changed data"
want "backups after the second run" 5 "SELECT count(*) FROM _reconcile_061_assignments"
assert_cleaned
echo "  second run: no change"

# Reverse restores exactly what was removed.
q "INSERT INTO ngac_assignments SELECT b.* FROM _reconcile_061_assignments b WHERE EXISTS (SELECT 1 FROM ngac_nodes WHERE id = b.child_id) AND EXISTS (SELECT 1 FROM ngac_nodes WHERE id = b.parent_id) ON CONFLICT DO NOTHING; INSERT INTO channel_members SELECT b.* FROM _reconcile_061_channel_members b WHERE EXISTS (SELECT 1 FROM channels WHERE id = b.channel_id) ON CONFLICT DO NOTHING" >/dev/null
[ "$before" = "$(snap)" ] || fail "the reverse did not restore the original rows"
echo "✓ 061 removes the injected edges and cache rows, spares members and DMs, is a no-op the second time, and reverses cleanly"
