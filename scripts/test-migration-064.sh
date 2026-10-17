#!/usr/bin/env bash
#
# Migration 064 gives every workspace and department the contacts-directory nodes the
# workspace service creates for new ones, leaves what release-A code already made exactly
# as it is (a hidden department stays hidden), skips a workspace with no Mgmt OA, brings
# the DeptPeers edges and tenant_users.department_id in line with the graph, and refuses
# to run on a graph it does not recognise. Builds four workspaces in a scratch copy of the
# test database, checks the state before, applies 064, checks what was made and what was
# not touched, re-applies it (a no-op), tries every guard and threshold (each must refuse
# and change nothing), dry-runs it (rollback), and finally applies the reverse SQL from the
# migration's own header and checks the database is back to where it started.
#
# Usage: scripts/test-migration-064.sh [source-db]   (default ngac_ci; needs docker compose postgres)
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${1:-ngac_ci}"
SCRATCH="ngac_scratch_064"
PG="${PG_CONTAINER:-nexus-enterprise-postgres-1}"
MIG="${MIG:-$ROOT/data/migrations/064_directory_visibility.sql}"
psql_() { docker exec -i "$PG" psql -X -q -A -t -v ON_ERROR_STOP=1 -U ngac "$@"; }
q() { psql_ -d "$SCRATCH" -c "$1"; }

psql_ -d postgres -c "DROP DATABASE IF EXISTS $SCRATCH" -c "CREATE DATABASE $SCRATCH TEMPLATE $SRC"
trap 'psql_ -d postgres -c "DROP DATABASE IF EXISTS $SCRATCH" >/dev/null' EXIT
# The source database may already have had 064 applied: start from a database that has not (no
# backup tables, no index), whatever it was, so the script does not depend on where it is run.
psql_ -d "$SCRATCH" -c "DROP TABLE IF EXISTS _reconcile_064_nodes, _reconcile_064_edges, _reconcile_064_prohibitions, _reconcile_064_tenant_users, _reconcile_064_renames" -c "DROP INDEX IF EXISTS ngac_nodes_directory_name_uniq"

fail() { echo "✗ $*" >&2; exit 1; }
want() { # want <description> <expected> <sql>
	local got
	got="$(q "$3")"
	[ "$got" = "$2" ] || fail "$1: expected '$2', got '$got'"
}
refuses() { # refuses <description> <message-fragment> <migration-text> : must fail with the fragment and change nothing
	local before after out
	before="$(snap)"
	out="$(printf '%s' "$3" | psql_ -d "$SCRATCH" 2>&1 || true)"
	echo "$out" | grep -q -- "$2" || fail "$1: did not refuse with '$2': $out"
	after="$(snap)"
	[ "$before" = "$after" ] || fail "$1: refused but changed data"
	echo "  refused: $1"
}

# A fingerprint of everything 064 may write, by node name: the ids 064 generates differ between runs.
snap() {
	q "SELECT md5(concat_ws('|',
	     (SELECT coalesce(string_agg(name || node_type || properties::text, ',' ORDER BY name, node_type), '') FROM ngac_nodes),
	     (SELECT coalesce(string_agg(c.name || '>' || p.name, ',' ORDER BY c.name, p.name), '') FROM ngac_assignments a JOIN ngac_nodes c ON c.id = a.child_id JOIN ngac_nodes p ON p.id = a.parent_id),
	     (SELECT coalesce(string_agg(u.name || '>' || o.name || array_to_string(a.operations, ','), ',' ORDER BY u.name, o.name), '') FROM ngac_associations a JOIN ngac_nodes u ON u.id = a.ua_id JOIN ngac_nodes o ON o.id = a.oa_id),
	     (SELECT coalesce(string_agg(p.name || (SELECT name FROM ngac_nodes WHERE id = p.subject_id) || array_to_string(p.operations, ',') || (SELECT string_agg(name, ',' ORDER BY name) FROM ngac_nodes WHERE id = ANY (p.target_oa_ids)) || p.intersection::text, ',' ORDER BY p.name), '') FROM ngac_prohibitions p),
	     (SELECT coalesce(string_agg(tenant_id || user_id || coalesce(department_id, '-'), ',' ORDER BY tenant_id, user_id), '') FROM tenant_users)))"
}

# --- Fixture ---------------------------------------------------------------
# t064a  organization, no directory yet. Departments: A (A1 below it), H (H1 below it), B.
#        People: oa (an owner), pA (directly in A), pA1 (in A1), pH1 (in H1), mm (member, none).
#        Drift: pA's tenant_users row says no department; pB's says A while the graph puts pB in none.
# t064b  organization made by release-A code: its directory exists, BX has its nodes, HX is
#        hidden (Dir and DirPeer directly under Directory, its UA reads its Dir), CX (below BX)
#        was created while the workspace had no nodes for BX's parent... and has none.
#        A person who left BX keeps a stale DeptPeers edge.
# t064c  legacy: no Mgmt OA, so it is left alone.
# t064p  personal: gets the five nodes and no department.
psql_ -d "$SCRATCH" <<'SQL'
-- workspaces' own nodes
INSERT INTO ngac_nodes (id, name, node_type) VALUES
 ('a-pc','PC_t064a','PC'),('a-mem','t064a_Members','UA'),('a-own','t064a_Owners','UA'),('a-mgmt','t064a_Mgmt','OA'),
 ('b-pc','PC_t064b','PC'),('b-mem','t064b_Members','UA'),('b-own','t064b_Owners','UA'),('b-mgmt','t064b_Mgmt','OA'),
 ('c-pc','PC_t064c','PC'),('c-mem','t064c_Members','UA'),('c-own','t064c_Owners','UA'),
 ('p-pc','PC_t064p','PC'),('p-mem','t064p_Members','UA'),('p-own','t064p_Owners','UA'),('p-mgmt','t064p_Mgmt','OA');
INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES
 ('s01','a-mem','a-pc'),('s02','a-own','a-pc'),('s03','a-mgmt','a-pc'),('s04','a-own','a-mem'),
 ('s05','b-mem','b-pc'),('s06','b-own','b-pc'),('s07','b-mgmt','b-pc'),('s08','b-own','b-mem'),
 ('s09','c-mem','c-pc'),('s10','c-own','c-pc'),('s11','c-own','c-mem'),
 ('s12','p-mem','p-pc'),('s13','p-own','p-pc'),('s14','p-mgmt','p-pc'),('s15','p-own','p-mem');

-- t064a: departments (UA nodes) and people
INSERT INTO ngac_nodes (id, name, node_type) VALUES
 ('a-uA','Dept_t064a-A','UA'),('a-uA1','Dept_t064a-A1','UA'),('a-uH','Dept_t064a-H','UA'),('a-uH1','Dept_t064a-H1','UA'),('a-uB','Dept_t064a-B','UA'),
 ('a-oa','U_t064a_oa','U'),('a-pA','U_t064a_pA','U'),('a-pA1','U_t064a_pA1','U'),('a-pH1','U_t064a_pH1','U'),('a-mm','U_t064a_mm','U'),('a-pB','U_t064a_pB','U');
INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES
 ('d01','a-uA','a-pc'),('d02','a-uA1','a-uA'),('d03','a-uH','a-pc'),('d04','a-uH1','a-uH'),('d05','a-uB','a-pc'),
 ('d06','a-oa','a-own'),('d07','a-pA','a-mem'),('d08','a-pA','a-uA'),('d09','a-pA1','a-mem'),('d10','a-pA1','a-uA1'),
 ('d11','a-pH1','a-mem'),('d12','a-pH1','a-uH1'),('d13','a-mm','a-mem'),('d14','a-pB','a-mem');
INSERT INTO users (id, username, password, ngac_node, display_name) VALUES
 ('u-oa','t064_oa','','a-oa','oa'),('u-pA','t064_pA','','a-pA','pA'),('u-pA1','t064_pA1','','a-pA1','pA1'),
 ('u-pH1','t064_pH1','','a-pH1','pH1'),('u-mm','t064_mm','','a-mm','mm'),('u-pB','t064_pB','','a-pB','pB');
INSERT INTO workspaces (id, name, owner_id, ngac_pc_id, type) VALUES
 ('t064a','t064a','u-oa','a-pc','organization'),('t064b','t064b','u-oa','b-pc','organization'),
 ('t064c','t064c','u-oa','c-pc','organization'),('t064p','t064p','u-oa','p-pc','personal');
INSERT INTO departments (id, workspace_id, name, parent_id, ngac_ua_id) VALUES
 ('t064a-A','t064a','A',NULL,'a-uA'),('t064a-A1','t064a','A1','t064a-A','a-uA1'),
 ('t064a-H','t064a','H',NULL,'a-uH'),('t064a-H1','t064a','H1','t064a-H','a-uH1'),('t064a-B','t064a','B',NULL,'a-uB');
INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id, department_id) VALUES
 ('t064a','u-oa','owner','active','a-oa',NULL),
 ('t064a','u-pA','member','active','a-pA',NULL),                -- drift: the graph says A
 ('t064a','u-pA1','member','active','a-pA1','t064a-A1'),        -- in step
 ('t064a','u-pH1','member','active','a-pH1','t064a-H1'),
 ('t064a','u-mm','member','active','a-mm',NULL),
 ('t064a','u-pB','member','active','a-pB','t064a-A');           -- drift: the graph says none

-- t064b: made by release-A code
INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES
 ('b-dir','Directory_t064b','OA','{"type":"directory","workspace_id":"t064b"}'),
 ('b-open','DirectoryOpen_t064b','OA','{"type":"directory","workspace_id":"t064b"}'),
 ('b-nodept','DirectoryNoDept_t064b','OA','{"type":"directory","workspace_id":"t064b"}'),
 ('b-owners','DirectoryOwners_t064b','OA','{"type":"directory","workspace_id":"t064b"}'),
 ('b-lim','DirectoryLimited_t064b','UA','{"type":"directory","workspace_id":"t064b"}'),
 ('b-uBX','Dept_t064b-BX','UA','{}'),('b-uHX','Dept_t064b-HX','UA','{}'),('b-uCX','Dept_t064b-CX','UA','{}'),
 ('b-dBX','Dir_t064b-BX','OA','{"type":"directory","workspace_id":"t064b"}'),
 ('b-pBX','DirPeer_t064b-BX','OA','{"type":"directory","workspace_id":"t064b"}'),
 ('b-sBX','DeptPeers_t064b-BX','UA','{"type":"directory","workspace_id":"t064b"}'),
 ('b-dHX','Dir_t064b-HX','OA','{"type":"directory","workspace_id":"t064b"}'),
 ('b-pHX','DirPeer_t064b-HX','OA','{"type":"directory","workspace_id":"t064b"}'),
 ('b-sHX','DeptPeers_t064b-HX','UA','{"type":"directory","workspace_id":"t064b"}'),
 ('b-pBXu','U_t064b_pBX','U','{}'),('b-leaver','U_t064b_leaver','U','{}'),('b-pHXu','U_t064b_pHX','U','{}');
INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES
 ('x01','b-dir','b-mgmt'),('x02','b-open','b-dir'),('x03','b-nodept','b-open'),('x04','b-owners','b-dir'),('x05','b-lim','b-pc'),
 ('x06','b-uBX','b-pc'),('x07','b-uHX','b-pc'),('x08','b-uCX','b-uBX'),
 ('x09','b-dBX','b-open'),('x10','b-pBX','b-dir'),('x11','b-sBX','b-pc'),
 ('x12','b-dHX','b-dir'),('x13','b-pHX','b-dir'),('x14','b-sHX','b-pc'),     -- HX: hidden, both directly under Directory
 ('x15','b-pBXu','b-mem'),('x16','b-pBXu','b-uBX'),('x17','b-pBXu','b-sBX'),
 ('x18','b-leaver','b-mem'),('x19','b-leaver','b-sBX'),                       -- left BX; the peers edge is stale
 ('x20','b-pHXu','b-mem'),('x21','b-pHXu','b-uHX'),('x22','b-pHXu','b-sHX');
INSERT INTO ngac_associations (id, ua_id, oa_id, operations) VALUES
 ('y01','b-mem','b-open','{read}'),('y02','b-mem','b-owners','{read}'),
 ('y03','b-sBX','b-pBX','{read}'),('y04','b-sHX','b-pHX','{read}'),('y05','b-uHX','b-dHX','{read}');
INSERT INTO ngac_prohibitions (name, subject_id, operations, target_oa_ids, intersection)
VALUES ('DirectoryLimitedRule_t064b','b-lim','{read}','{b-open}',false);
INSERT INTO users (id, username, password, ngac_node, display_name) VALUES
 ('u-bBX','t064_bBX','','b-pBXu','bBX'),('u-bL','t064_bL','','b-leaver','bL'),('u-bHX','t064_bHX','','b-pHXu','bHX');
INSERT INTO departments (id, workspace_id, name, parent_id, ngac_ua_id) VALUES
 ('t064b-BX','t064b','BX',NULL,'b-uBX'),('t064b-HX','t064b','HX',NULL,'b-uHX'),('t064b-CX','t064b','CX','t064b-BX','b-uCX');
INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id, department_id) VALUES
 ('t064b','u-bBX','member','active','b-pBXu','t064b-BX'),
 ('t064b','u-bL','member','active','b-leaver',NULL),
 ('t064b','u-bHX','member','active','b-pHXu','t064b-HX');
SQL

# A re-run of migration 023 (`make db-migrate` re-applies the whole chain on a dev database) renames
# any OA below a workspace PC and any UA directly under it that carries no platform name, which
# is what the directory's nodes look like to it: t064b's Owners OA and its Limited UA were renamed
# that way and 064 has to give them their names back. An ordinary role renamed by 023 (not a
# directory node) must stay as 023 left it.
psql_ -d "$SCRATCH" <<'SQL'
UPDATE ngac_nodes SET name = 'Folder_b-owners' WHERE id = 'b-owners';
UPDATE ngac_nodes SET name = 'Role_b-lim' WHERE id = 'b-lim';
INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ('b-role', 'Role_b-role', 'UA', '{"type":"role","display_name":"Editors"}');
INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ('x30','b-role','b-pc');
INSERT INTO ngac_node_renames (node_id, old_name, new_name, props_patch, reason) VALUES
 ('b-owners', 'DirectoryOwners_t064b', 'Folder_b-owners', '{}', 'workspace folder'),
 ('b-lim', 'DirectoryLimited_t064b', 'Role_b-lim', '{}', 'role'),
 ('b-role', 'Editors', 'Role_b-role', '{}', 'role');
SQL

dept_of() { q "SELECT coalesce(department_id, '-') FROM tenant_users WHERE tenant_id = '$1' AND user_id = '$2'"; }

# Red: before 064 the fixture has no directory for a, c, p and a gap in b.
want "fixture: t064a has no directory"    0 "SELECT count(*) FROM ngac_nodes WHERE properties->>'type' = 'directory' AND properties->>'workspace_id' = 't064a'"
want "fixture: CX has no nodes"           0 "SELECT count(*) FROM ngac_nodes WHERE name LIKE '%t064b-CX'  AND name <> 'Dept_t064b-CX'"
want "fixture: HX is hidden"              "Directory_t064b" "SELECT string_agg(p.name, ',') FROM ngac_assignments a JOIN ngac_nodes p ON p.id = a.parent_id WHERE a.child_id = 'b-dHX'"
[ "$(dept_of t064a u-pA)" = "-" ] && [ "$(dept_of t064a u-pB)" = "t064a-A" ] || fail "fixture: the drifted rows must be drifted"
BEFORE="$(snap)"

# --- Guards and thresholds: each refuses and changes nothing -----------------
# (the file as is, with one condition injected into the data or one limit lowered)
MIGTXT="$(cat "$MIG")"
BASE="90, 130, 5, 16, 10, 2, 0"   # nodes, edges, removed, rules, rows, skipped, restored (the file's, set from production)
grep -q "INSERT INTO _m064_limits VALUES ($BASE);" "$MIG" || fail "BASE does not match the limits in $MIG"
# The fixture is not production: it restores renamed nodes (a dev-only case) and may outgrow the production
# limits, so the runs that must succeed use the file with roomy limits. The thresholds are tried on the file as is.
RUNMIG="$(mktemp)"; trap 'rm -f "$RUNMIG"; psql_ -d postgres -c "DROP DATABASE IF EXISTS $SCRATCH" >/dev/null' EXIT
sed "s/INSERT INTO _m064_limits VALUES ($BASE);/INSERT INTO _m064_limits VALUES (2000, 3000, 100, 400, 200, 10, 2000);/" "$MIG" >"$RUNMIG"
RUNTXT="$(cat "$RUNMIG")"

q "INSERT INTO workspaces (id, name, owner_id, ngac_pc_id, type) VALUES ('t064bad','bad','u-oa','a-pc','organization')"
refuses "a workspace with no PC_<id> or Members under it" "lack PC_<id> or <id>_Members" "$MIGTXT"
q "DELETE FROM workspaces WHERE id = 't064bad'"

lowered() { echo "$BASE" | awk -F', ' -v i="$1" -v v="$2" 'BEGIN{OFS=", "} {$i=v; print}'; }
for limit in "1:1:nodes would be created (limit 1)" \
             "2:1:edges would be created (limit 1)" \
             "3:0:stale edges would be removed (limit 0)" \
             "4:1:prohibitions would be created (limit 1)" \
             "5:1:department_id values would change (limit 1)" \
             "6:0:workspaces skipped for want of Mgmt (limit 0)" \
             "7:1:renamed directory nodes would be restored (limit 1)"; do
	IFS=: read -r idx val msg <<<"$limit"
	refuses "threshold: $msg" "$msg" "$(sed "s/INSERT INTO _m064_limits VALUES ($BASE);/INSERT INTO _m064_limits VALUES ($(lowered "$idx" "$val"));/" "$MIG")"
done

q "INSERT INTO ngac_nodes (id, name, node_type) VALUES ('spoof','Directory_t064a','OA')"
refuses "a node named like a directory node that is not one" "is named like a directory node of workspace t064a" "$MIGTXT"
q "DELETE FROM ngac_nodes WHERE id = 'spoof'"

q "INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ('spoof','Dir_t064a-A','OA','{\"type\":\"directory\",\"workspace_id\":\"t064b\"}')"
refuses "a directory node marked for another workspace" "is named like a directory node of workspace t064a" "$MIGTXT"
q "DELETE FROM ngac_nodes WHERE id = 'spoof'"

q "UPDATE departments SET parent_id = 't064a-A1' WHERE id = 't064a-A'"
refuses "departments in a cycle" "not reachable from a root" "$MIGTXT"
q "UPDATE departments SET parent_id = NULL WHERE id = 't064a-A'"

q "UPDATE departments SET parent_id = 't064b-BX' WHERE id = 't064a-B'"
refuses "a parent in another workspace" "not reachable from a root" "$MIGTXT"
q "UPDATE departments SET parent_id = NULL WHERE id = 't064a-B'"

q "INSERT INTO departments (id, workspace_id, name, parent_id, ngac_ua_id) VALUES ('t064a-X','t064a','X',NULL,'no-such-ua')"
refuses "a department with no UA node" "have no UA node" "$MIGTXT"
q "DELETE FROM departments WHERE id = 't064a-X'"

q "INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ('amb','a-pA','a-uB')"
refuses "a person directly under two departments" "more than one department" "$MIGTXT"
q "DELETE FROM ngac_assignments WHERE id = 'amb'"

q "INSERT INTO ngac_nodes (id, name, node_type) VALUES ('taken','DirectoryOwners_t064b','OA')"
refuses "a renamed directory node whose name is taken by another node" "cannot give back its name" "$MIGTXT"
q "DELETE FROM ngac_nodes WHERE id = 'taken'"

q "INSERT INTO ngac_prohibitions (name, subject_id, operations, target_oa_ids) VALUES ('DirectoryLimitedRule_t064b','a-mem','{read}','{b-open}') ON CONFLICT (name) DO UPDATE SET subject_id = 'a-mem'"
refuses "a prohibition of that name that is not the rule" "is not the directory rule of workspace t064b" "$MIGTXT"
q "UPDATE ngac_prohibitions SET subject_id = 'b-lim' WHERE name = 'DirectoryLimitedRule_t064b'"

[ "$BEFORE" = "$(snap)" ] || fail "the fixture changed while guards were tried"

# --- Dry run: the whole file with COMMIT turned into ROLLBACK ---------------
out="$(printf '%s' "${RUNTXT//$'\nCOMMIT;'/$'\nROLLBACK;'}" | psql_ -d "$SCRATCH" 2>&1)"
echo "$out" | grep -q "migration 064: created" || fail "the dry run did not report its counts: $out"
[ "$BEFORE" = "$(snap)" ] || fail "the dry run changed data"
echo "  dry run: $(echo "$out" | grep 'migration 064: created' | sed 's/.*migration 064: //')"

# --- Apply -----------------------------------------------------------------
out="$(psql_ -d "$SCRATCH" <"$RUNMIG" 2>&1)" || fail "064 failed: $out"
echo "$out" | grep -q "workspace(s) have no <id>_Mgmt under their policy class and are left alone: .*t064c" || fail "the skipped workspace was not reported: $out"
AFTER="$(snap)"
[ "$AFTER" != "$BEFORE" ] || fail "064 changed nothing"

check_applied() {
	local ws id
	# t064a and t064p: the five nodes, marked, in the right place.
	for ws in t064a t064p; do
		want "$ws: Directory under Mgmt"       "${ws}_Mgmt"            "$(sorted_parents_sql Directory_$ws)"
		want "$ws: Open under Directory"       "Directory_$ws"         "$(sorted_parents_sql DirectoryOpen_$ws)"
		want "$ws: NoDept under Open"          "DirectoryOpen_$ws"     "$(sorted_parents_sql DirectoryNoDept_$ws)"
		want "$ws: Owners beside Open"         "Directory_$ws"         "$(sorted_parents_sql DirectoryOwners_$ws)"
		want "$ws: Limited under the class only" "PC_$ws"              "$(sorted_parents_sql DirectoryLimited_$ws)"
		want "$ws: marked"  "{\"type\": \"directory\", \"workspace_id\": \"$ws\"}" "SELECT properties::text FROM ngac_nodes WHERE name = 'Directory_$ws'"
		want "$ws: Members read Open and Owners" "DirectoryOpen_$ws:read,DirectoryOwners_$ws:read" \
			"SELECT string_agg(o.name || ':' || array_to_string(a.operations, ','), ',' ORDER BY o.name) FROM ngac_associations a JOIN ngac_nodes u ON u.id = a.ua_id JOIN ngac_nodes o ON o.id = a.oa_id WHERE u.name = '${ws}_Members' AND o.name LIKE 'Directory%'"
		want "$ws: Limited grants nothing"     0 "SELECT count(*) FROM ngac_associations a JOIN ngac_nodes u ON u.id = a.ua_id WHERE u.name = 'DirectoryLimited_$ws'"
		want "$ws: the rule" "DirectoryLimited_$ws|{read}|{DirectoryOpen_$ws}|false" \
			"SELECT s.name || '|' || p.operations::text || '|' || (SELECT '{' || string_agg(o.name, ',') || '}' FROM ngac_nodes o WHERE o.id = ANY (p.target_oa_ids)) || '|' || p.intersection::text FROM ngac_prohibitions p JOIN ngac_nodes s ON s.id = p.subject_id WHERE p.name = 'DirectoryLimitedRule_$ws'"
		want "$ws: nothing directory under Members" 0 \
			"WITH RECURSIVE below(n) AS (SELECT a.child_id FROM ngac_assignments a JOIN ngac_nodes m ON m.id = a.parent_id AND m.name = '${ws}_Members' UNION SELECT a.child_id FROM below b JOIN ngac_assignments a ON a.parent_id = b.n) SELECT count(*) FROM below b JOIN ngac_nodes n ON n.id = b.n WHERE n.properties->>'type' = 'directory'"
	done

	# t064a: the department tree is mirrored, parents first.
	want "A: Dir under Open"        "DirectoryOpen_t064a"  "$(sorted_parents_sql Dir_t064a-A)"
	want "A1: Dir under A's"        "Dir_t064a-A"          "$(sorted_parents_sql Dir_t064a-A1)"
	want "A: DirPeer under Directory" "Directory_t064a"    "$(sorted_parents_sql DirPeer_t064a-A)"
	want "A1: DirPeer under A's"    "DirPeer_t064a-A"      "$(sorted_parents_sql DirPeer_t064a-A1)"
	want "H1: Dir under H's"        "Dir_t064a-H"          "$(sorted_parents_sql Dir_t064a-H1)"
	want "A1: peers UA flat"        "PC_t064a"             "$(sorted_parents_sql DeptPeers_t064a-A1)"
	want "A1: peers read DirPeer"   "DirPeer_t064a-A1:read" \
		"SELECT string_agg(o.name || ':' || array_to_string(a.operations, ','), ',') FROM ngac_associations a JOIN ngac_nodes u ON u.id = a.ua_id JOIN ngac_nodes o ON o.id = a.oa_id WHERE u.name = 'DeptPeers_t064a-A1'"
	want "B: three nodes"           3 "SELECT count(*) FROM ngac_nodes WHERE name IN ('Dir_t064a-B','DirPeer_t064a-B','DeptPeers_t064a-B')"
	# People directly in a department are its peers; others are not.
	want "peers of A"   "U_t064a_pA"  "SELECT string_agg(c.name, ',') FROM ngac_assignments a JOIN ngac_nodes c ON c.id = a.child_id JOIN ngac_nodes p ON p.id = a.parent_id WHERE p.name = 'DeptPeers_t064a-A'"
	want "peers of A1"  "U_t064a_pA1" "SELECT string_agg(c.name, ',') FROM ngac_assignments a JOIN ngac_nodes c ON c.id = a.child_id JOIN ngac_nodes p ON p.id = a.parent_id WHERE p.name = 'DeptPeers_t064a-A1'"
	want "peers of H1"  "U_t064a_pH1" "SELECT string_agg(c.name, ',') FROM ngac_assignments a JOIN ngac_nodes c ON c.id = a.child_id JOIN ngac_nodes p ON p.id = a.parent_id WHERE p.name = 'DeptPeers_t064a-H1'"
	want "a person with no department sits under no peers UA" 0 \
		"SELECT count(*) FROM ngac_assignments a JOIN ngac_nodes p ON p.id = a.parent_id JOIN ngac_nodes c ON c.id = a.child_id WHERE c.name IN ('U_t064a_mm','U_t064a_oa') AND p.name LIKE 'DeptPeers_%'"
	want "an ancestor's peers are not inherited" 0 \
		"SELECT count(*) FROM ngac_assignments a JOIN ngac_nodes p ON p.id = a.parent_id JOIN ngac_nodes c ON c.id = a.child_id WHERE c.name = 'U_t064a_pA1' AND p.name = 'DeptPeers_t064a-A'"

	# Rows follow the graph.
	want "pA's row"  "t064a-A" "SELECT coalesce(department_id, '-') FROM tenant_users WHERE user_id = 'u-pA'"
	want "pB's row"  "-"       "SELECT coalesce(department_id, '-') FROM tenant_users WHERE user_id = 'u-pB'"
	want "pA1's row is as it was" "t064a-A1" "SELECT coalesce(department_id, '-') FROM tenant_users WHERE user_id = 'u-pA1'"
	want "backup of the two rows" "u-pA:-:t064a-A,u-pB:t064a-A:-" \
		"SELECT string_agg(user_id || ':' || coalesce(old_department_id, '-') || ':' || coalesce(new_department_id, '-'), ',' ORDER BY user_id) FROM _reconcile_064_tenant_users WHERE tenant_id = 't064a'"

	# t064b: what release-A code made is as it was; the gap is filled; the stale edge is gone.
	want "b: the hidden Dir stays directly under Directory"     "Directory_t064b" "$(sorted_parents_sql Dir_t064b-HX)"
	want "b: the hidden DirPeer stays directly under Directory" "Directory_t064b" "$(sorted_parents_sql DirPeer_t064b-HX)"
	want "b: the hidden department's UA still reads its Dir" 1 "SELECT count(*) FROM ngac_associations WHERE id = 'y05'"
	want "b: BX's nodes keep their ids" "b-dBX,b-pBX,b-sBX" "SELECT string_agg(id, ',' ORDER BY id) FROM ngac_nodes WHERE name IN ('Dir_t064b-BX','DirPeer_t064b-BX','DeptPeers_t064b-BX')"
	want "b: the directory keeps its ids" "b-dir,b-lim,b-nodept,b-open,b-owners" "SELECT string_agg(id, ',' ORDER BY id) FROM ngac_nodes WHERE properties->>'type' = 'directory' AND properties->>'workspace_id' = 't064b' AND name !~ '^(Dir_|DirPeer_|DeptPeers_)'"
	want "b: CX's Dir below BX's"      "Dir_t064b-BX"     "$(sorted_parents_sql Dir_t064b-CX)"
	want "b: CX's DirPeer below BX's"  "DirPeer_t064b-BX" "$(sorted_parents_sql DirPeer_t064b-CX)"
	want "b: CX's peers UA is flat"    "PC_t064b"         "$(sorted_parents_sql DeptPeers_t064b-CX)"
	want "b: the stale peers edge is removed" 0 "SELECT count(*) FROM ngac_assignments WHERE id = 'x19'"
	want "b: and recorded" "U_t064b_leaver" "SELECT string_agg(c.name, ',') FROM _reconcile_064_edges e JOIN ngac_nodes c ON c.id = e.child_or_ua WHERE e.action = 'removed'"
	want "b: BX's own person keeps the peers edge" 1 "SELECT count(*) FROM ngac_assignments WHERE id = 'x17'"
	want "b: the hidden department's person keeps theirs" 1 "SELECT count(*) FROM ngac_assignments WHERE id = 'x22'"
	want "b: nothing new for the directory it already had" 0 "SELECT count(*) FROM _reconcile_064_nodes WHERE workspace_id = 't064b' AND name !~ 'CX'"

	# What a re-run of 023 renamed comes back under its name; a role it renamed does not.
	want "b: Owners restored"  "b-owners" "SELECT id FROM ngac_nodes WHERE name = 'DirectoryOwners_t064b'"
	want "b: Limited restored" "b-lim"    "SELECT id FROM ngac_nodes WHERE name = 'DirectoryLimited_t064b'"
	want "b: the role stays renamed" "Role_b-role" "SELECT name FROM ngac_nodes WHERE id = 'b-role'"
	want "b: no directory node is left under a name 023 gave it" 0 "SELECT count(*) FROM ngac_nodes WHERE properties->>'type' = 'directory' AND name ~ '^(Folder_|Role_)'"

	# t064c: left alone.
	want "c: no directory"  0 "SELECT count(*) FROM ngac_nodes WHERE name LIKE '%t064c%' AND properties->>'type' = 'directory'"
	want "c: no rule"       0 "SELECT count(*) FROM ngac_prohibitions WHERE name = 'DirectoryLimitedRule_t064c'"

	# The unique index, and the totals.
	want "the unique index" 1 "SELECT count(*) FROM pg_indexes WHERE indexname = 'ngac_nodes_directory_name_uniq'"
	want "nodes made for the fixtures" 28 "SELECT count(*) FROM _reconcile_064_nodes WHERE workspace_id LIKE 't064%'"
	want "every created node is marked" 0 "SELECT count(*) FROM _reconcile_064_nodes b JOIN ngac_nodes n ON n.id = b.node_id WHERE n.properties->>'type' IS DISTINCT FROM 'directory'"
}
sorted_parents_sql() { echo "SELECT coalesce(string_agg(p.name, ',' ORDER BY p.name), '') FROM ngac_assignments a JOIN ngac_nodes c ON c.id = a.child_id JOIN ngac_nodes p ON p.id = a.parent_id WHERE c.name = '$1'"; }
check_applied
echo "  applied: directory, departments, peers, rows; release-A nodes (hidden HX included) untouched; stale edge removed; t064c skipped"

# --- Second run: nothing to do ------------------------------------------------
BACKUPS="$(q "SELECT (SELECT count(*) FROM _reconcile_064_nodes) || ',' || (SELECT count(*) FROM _reconcile_064_edges) || ',' || (SELECT count(*) FROM _reconcile_064_prohibitions) || ',' || (SELECT count(*) FROM _reconcile_064_tenant_users) || ',' || (SELECT count(*) FROM _reconcile_064_renames)")"
out="$(psql_ -d "$SCRATCH" <"$RUNMIG" 2>&1)" || fail "the second run failed: $out"
echo "$out" | grep -q "created 0 nodes, 0 edges, 0 prohibitions; removed 0 stale peers edges; changed 0 department_id rows; skipped 1 workspaces; restored 0 names" || fail "the second run was not a no-op: $out"
[ "$AFTER" = "$(snap)" ] || fail "a second run changed data"
[ "$BACKUPS" = "$(q "SELECT (SELECT count(*) FROM _reconcile_064_nodes) || ',' || (SELECT count(*) FROM _reconcile_064_edges) || ',' || (SELECT count(*) FROM _reconcile_064_prohibitions) || ',' || (SELECT count(*) FROM _reconcile_064_tenant_users) || ',' || (SELECT count(*) FROM _reconcile_064_renames)")" ] || fail "a second run added backup rows"
check_applied
echo "  second run: 0 rows, no backup rows"

# --- Reverse: the SQL in the migration's header restores where we started -----------
REVERSE="$(awk '/^--   BEGIN;$/{on=1} on{l=$0; sub(/^--   ?/, "", l); print l} /^--   COMMIT;$/{on=0}' "$MIG")"
case "$REVERSE" in BEGIN\;*COMMIT\;) ;; *) fail "could not extract the reverse SQL from the header" ;; esac
printf '%s\n' "$REVERSE" | psql_ -d "$SCRATCH" >/dev/null
[ "$BEFORE" = "$(snap)" ] || fail "the reverse did not restore the original rows"
want "the unique index is gone after the reverse" 0 "SELECT count(*) FROM pg_indexes WHERE indexname = 'ngac_nodes_directory_name_uniq'"
echo "  reverse (from the header): nodes, edges, rules and department_id are as before"

# --- Applying it again after the reverse works (the backups are kept, the rows are new) --
q "DROP TABLE _reconcile_064_nodes, _reconcile_064_edges, _reconcile_064_prohibitions, _reconcile_064_tenant_users, _reconcile_064_renames"
psql_ -d "$SCRATCH" <"$RUNMIG" >/dev/null
[ "$AFTER" = "$(snap)" ] || fail "applying again after the reverse did not give the same result"
echo "✓ 064 builds the directory for workspaces and departments, leaves release-A nodes and hidden departments alone, refuses on every guard and threshold, is a no-op the second time, and reverses cleanly"
