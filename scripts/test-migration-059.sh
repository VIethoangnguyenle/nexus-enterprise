#!/usr/bin/env bash
#
# Tests data/migrations/059_ngac_workspace_id_keyed_repair.sql on a scratch
# database built from init.sql and migrations 001-058, never on a shared one.
#
# Shapes covered: a workspace whose PC has no children (dev's test-ws-*), one with
# legacy-named Members/Owners/Channels, one already ID-keyed (must not change), and
# an ambiguous one (two Members candidates: the migration must refuse). Each run is
# applied twice; the second must change nothing.
#
# Needs the dev Postgres container (make dev-infra). Override with PG_CONTAINER.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PG="${PG_CONTAINER:-nexus-enterprise-postgres-1}"
DB="m059_$$"
MIG="$ROOT/data/migrations/059_ngac_workspace_id_keyed_repair.sql"

psql_admin() { docker exec -i "$PG" psql -U ngac -d postgres -v ON_ERROR_STOP=1 -q "$@"; }
psql_db() { docker exec -i "$PG" psql -U ngac -d "$DB" -v ON_ERROR_STOP=1 -q -A -t "$@"; }
fail() { echo "✗ $*" >&2; exit 1; }
expect() { # description, expected, actual
	[ "$2" = "$3" ] || fail "$1: want '$2', got '$3'"
	echo "  ✓ $1"
}

psql_admin -c "CREATE DATABASE $DB" || exit 1
trap 'psql_admin -c "DROP DATABASE IF EXISTS $DB" >/dev/null 2>&1' EXIT

echo "▸ scratch database $DB: init.sql + migrations below 059"
psql_db <"$ROOT/data/init.sql" >/dev/null || fail "init.sql"
for f in $(ls "$ROOT"/data/migrations/*.sql | sort); do
	case "$(basename "$f")" in 059_*) break ;; esac
	psql_db <"$f" >/dev/null 2>&1 || fail "$(basename "$f")"
done

# Fixtures. Foreign keys to users are not what is under test.
psql_db >/dev/null <<'SQL'
SET session_replication_role = replica;
INSERT INTO ngac_nodes (id, name, node_type) VALUES
  ('pcA', 'pc-wsA', 'PC'),
  ('pcB', 'Acme PC', 'PC'), ('bM', 'Acme_Members', 'UA'), ('bO', 'Acme_Owners', 'UA'), ('bC', 'Acme_Channels', 'OA'),
  ('bTM', 'TenantMember_wsB', 'UA'), ('bTO', 'TenantOwner_wsB', 'UA'),
  ('pcD', 'PC_wsD', 'PC'), ('dM', 'wsD_Members', 'UA'), ('dO', 'wsD_Owners', 'UA'), ('dC', 'wsD_Channels', 'OA');
UPDATE ngac_nodes SET properties = '{"workspace_id":"wsD","tenant_id":"wsD","scope":"tenant"}' WHERE id = 'pcD';
INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES
  ('a1','bM','pcB'), ('a2','bO','pcB'), ('a3','bC','pcB'), ('a4','bTM','pcB'), ('a5','bTO','pcB'),
  ('d1','dM','pcD'), ('d2','dO','pcD'), ('d3','dC','pcD'), ('d4','dO','dM');
INSERT INTO ngac_associations (id, ua_id, oa_id, operations) VALUES ('x1','bM','bC','{read}');
INSERT INTO workspaces (id, name, owner_id, ngac_pc_id, type) VALUES
  ('wsA', 'A', 'u', 'pcA', 'organization'), ('wsB', 'B', 'u', 'pcB', 'organization'), ('wsD', 'D', 'u', 'pcD', 'organization');
SQL

snapshot() { psql_db -c "SELECT (SELECT count(*) FROM ngac_nodes), (SELECT count(*) FROM ngac_assignments), (SELECT count(*) FROM ngac_associations), (SELECT coalesce(md5(string_agg(id||name||properties::text, ',' ORDER BY id)),'') FROM ngac_nodes)"; }
healthy_before="$(psql_db -c "SELECT md5(string_agg(n.id||n.name||n.properties::text, ',' ORDER BY n.id)) FROM ngac_nodes n WHERE n.id IN ('pcD','dM','dO','dC')")"

echo "▸ first run"
psql_db <"$MIG" >/dev/null || fail "059 failed on the first run"

has() { psql_db -c "SELECT count(*) FROM ngac_nodes WHERE name = '$1' AND node_type = '$2'"; }
edge() { psql_db -c "SELECT count(*) FROM ngac_assignments a JOIN ngac_nodes c ON c.id=a.child_id JOIN ngac_nodes p ON p.id=a.parent_id WHERE c.name='$1' AND p.name='$2'"; }

expect "A: PC renamed" 1 "$(has PC_wsA PC)"
expect "A: Members created" 1 "$(has wsA_Members UA)"
expect "A: Owners created" 1 "$(has wsA_Owners UA)"
expect "A: Channels created" 1 "$(has wsA_Channels OA)"
expect "A: Owners under Members" 1 "$(edge wsA_Owners wsA_Members)"
expect "A: Channels under PC" 1 "$(edge wsA_Channels PC_wsA)"
expect "A: PC carries workspace_id" wsA "$(psql_db -c "SELECT properties->>'workspace_id' FROM ngac_nodes WHERE name='PC_wsA'")"
expect "A: owners and members associated with Channels" 2 "$(psql_db -c "SELECT count(*) FROM ngac_associations s JOIN ngac_nodes o ON o.id=s.oa_id WHERE o.name='wsA_Channels'")"
expect "A: owners hold manage on Channels" 1 "$(psql_db -c "SELECT count(*) FROM ngac_associations s JOIN ngac_nodes u ON u.id=s.ua_id JOIN ngac_nodes o ON o.id=s.oa_id WHERE u.name='wsA_Owners' AND o.name='wsA_Channels' AND 'manage' = ANY(s.operations)")"

expect "B: PC renamed in place" pcB "$(psql_db -c "SELECT id FROM ngac_nodes WHERE name='PC_wsB'")"
expect "B: Members renamed in place (same id)" bM "$(psql_db -c "SELECT id FROM ngac_nodes WHERE name='wsB_Members'")"
expect "B: Owners renamed in place" bO "$(psql_db -c "SELECT id FROM ngac_nodes WHERE name='wsB_Owners'")"
expect "B: Channels renamed in place" bC "$(psql_db -c "SELECT id FROM ngac_nodes WHERE name='wsB_Channels'")"
expect "B: legacy names are gone" 0 "$(psql_db -c "SELECT count(*) FROM ngac_nodes WHERE name LIKE 'Acme%'")"
expect "B: TenantMember under Members" 1 "$(edge TenantMember_wsB wsB_Members)"
expect "B: TenantOwner under Owners" 1 "$(edge TenantOwner_wsB wsB_Owners)"
expect "B: Owners under Members" 1 "$(edge wsB_Owners wsB_Members)"
expect "B: associations it already had are untouched" read "$(psql_db -c "SELECT array_to_string(operations, ',') FROM ngac_associations WHERE id='x1'")"
expect "B: no association was added to a renamed node" 1 "$(psql_db -c "SELECT count(*) FROM ngac_associations WHERE oa_id='bC'")"

healthy_after="$(psql_db -c "SELECT md5(string_agg(n.id||n.name||n.properties::text, ',' ORDER BY n.id)) FROM ngac_nodes n WHERE n.id IN ('pcD','dM','dO','dC')")"
expect "D: an already ID-keyed workspace is not touched" "$healthy_before" "$healthy_after"
expect "every workspace has Members" 0 "$(psql_db -c "SELECT count(*) FROM workspaces w WHERE NOT EXISTS (SELECT 1 FROM ngac_nodes n WHERE n.name = w.id||'_Members' AND n.node_type='UA')")"

echo "▸ second run changes nothing"
first="$(snapshot)"
psql_db <"$MIG" >/dev/null || fail "059 failed on the second run"
expect "second run is a no-op" "$first" "$(snapshot)"

echo "▸ ambiguous workspace is refused"
psql_db >/dev/null <<'SQL'
SET session_replication_role = replica;
INSERT INTO ngac_nodes (id, name, node_type) VALUES ('pcE','PC_E','PC'), ('e1','One_Members','UA'), ('e2','Two_Members','UA');
INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ('e-a1','e1','pcE'), ('e-a2','e2','pcE');
INSERT INTO workspaces (id, name, owner_id, ngac_pc_id, type) VALUES ('wsE','E','u','pcE','organization');
SQL
if psql_db <"$MIG" >/dev/null 2>&1; then fail "059 accepted two Members candidates"; fi
expect "refusal rolled everything back (no wsE_Members)" 0 "$(has wsE_Members UA)"

psql_db >/dev/null <<'SQL'
SET session_replication_role = replica;
DELETE FROM workspaces WHERE id = 'wsE';
DELETE FROM ngac_assignments WHERE child_id IN ('e1','e2');
DELETE FROM ngac_nodes WHERE id IN ('pcE','e1','e2');
SQL
psql_db <"$MIG" >/dev/null || fail "059 does not run cleanly again once the ambiguous workspace is gone"

# Each refusal below is its own fixture on top of the repaired database, removed afterwards.
refuses() { # description, fixture sql, workspace id whose repair must not happen
	psql_db >/dev/null <<SQL
SET session_replication_role = replica;
$2
SQL
	if psql_db <"$MIG" >/dev/null 2>&1; then fail "059 accepted: $1"; fi
	expect "refused: $1" "${4:-0}" "$(has "$3_Members" UA)"
	psql_db >/dev/null <<SQL
SET session_replication_role = replica;
DELETE FROM ngac_associations WHERE ua_id LIKE 'z%' OR oa_id LIKE 'z%';
DELETE FROM ngac_assignments WHERE child_id LIKE 'z%' OR parent_id LIKE 'z%';
DELETE FROM workspaces WHERE id LIKE 'wsZ%';
DELETE FROM ngac_nodes WHERE id LIKE 'z%';
SQL
}
refuses "a candidate associated with another workspace's OA" "
INSERT INTO ngac_nodes (id,name,node_type) VALUES ('zpc','PC_wsZ1','PC'),('zm','Staff_Members','UA'),('zoa','Secret_Vault','OA');
INSERT INTO ngac_assignments (id,child_id,parent_id) VALUES ('za1','zm','zpc'),('za2','zoa','zpc');
INSERT INTO ngac_associations (id,ua_id,oa_id,operations) VALUES ('zs1','zm','zoa','{read}');
INSERT INTO workspaces (id,name,owner_id,ngac_pc_id,type) VALUES ('wsZ1','Z','u','zpc','organization');" wsZ1
refuses "a Members candidate that holds this workspace's Mgmt" "
INSERT INTO ngac_nodes (id,name,node_type) VALUES ('zpc','PC_wsZ3','PC'),('zm','Board_Members','UA'),('zoa','wsZ3_Mgmt','OA');
INSERT INTO ngac_assignments (id,child_id,parent_id) VALUES ('za1','zm','zpc'),('za2','zoa','zpc');
INSERT INTO ngac_associations (id,ua_id,oa_id,operations) VALUES ('zs1','zm','zoa','{manage}');
INSERT INTO workspaces (id,name,owner_id,ngac_pc_id,type) VALUES ('wsZ3','Z','u','zpc','organization');" wsZ3
psql_db >/dev/null <<'SQL'
SET session_replication_role = replica;
INSERT INTO ngac_nodes (id,name,node_type) VALUES ('zpc','PC_wsZ2','PC'),('zr','Role_x_Members','UA');
INSERT INTO ngac_assignments (id,child_id,parent_id) VALUES ('za1','zr','zpc');
INSERT INTO workspaces (id,name,owner_id,ngac_pc_id,type) VALUES ('wsZ2','Z','u','zpc','organization');
SQL
psql_db <"$MIG" >/dev/null || fail "059 failed on a workspace with a Role_ node under its PC"
expect "a Role_..._Members node is not renamed into Members" zr "$(psql_db -c "SELECT id FROM ngac_nodes WHERE name='Role_x_Members'")"
expect "a fresh Members was created beside it" 1 "$(has wsZ2_Members UA)"
psql_db >/dev/null <<'SQL'
SET session_replication_role = replica;
DELETE FROM ngac_associations WHERE ua_id LIKE 'z%' OR oa_id LIKE 'z%' OR ua_id IN (SELECT id FROM ngac_nodes WHERE name LIKE 'wsZ2\_%') OR oa_id IN (SELECT id FROM ngac_nodes WHERE name LIKE 'wsZ2\_%');
DELETE FROM ngac_assignments WHERE child_id LIKE 'z%' OR parent_id LIKE 'z%' OR child_id IN (SELECT id FROM ngac_nodes WHERE name LIKE 'wsZ2\_%') OR parent_id IN (SELECT id FROM ngac_nodes WHERE name LIKE 'wsZ2\_%');
DELETE FROM workspaces WHERE id = 'wsZ2';
DELETE FROM ngac_nodes WHERE id LIKE 'z%' OR name LIKE 'wsZ2\_%';
SQL
refuses "a PC claiming another workspace" "
INSERT INTO ngac_nodes (id,name,node_type,properties) VALUES ('zpc','PC_wsZ3','PC','{\"workspace_id\":\"someone-else\"}');
INSERT INTO workspaces (id,name,owner_id,ngac_pc_id,type) VALUES ('wsZ3','Z','u','zpc','organization');" wsZ3
# Duplicate target names can only be built with the index out of the way.
psql_db -c "DROP INDEX ngac_nodes_workspace_role_name_uniq" >/dev/null
refuses "two nodes already named wsZ4_Members" "
INSERT INTO ngac_nodes (id,name,node_type) VALUES ('zpc','PC_wsZ4','PC'),('zm1','wsZ4_Members','UA'),('zm2','wsZ4_Members','UA');
INSERT INTO workspaces (id,name,owner_id,ngac_pc_id,type) VALUES ('wsZ4','Z','u','zpc','organization');" wsZ4 2
psql_db <"$MIG" >/dev/null || fail "059 did not recreate its index"

echo "▸ two nodes with a workspace role name cannot exist"
if psql_db -c "INSERT INTO ngac_nodes (id,name,node_type) VALUES ('dup1','wsB_Members','UA')" >/dev/null 2>&1; then fail "unique index missing: a second wsB_Members was accepted"; fi
echo "  ✓ unique index holds"

echo "✓ migration 059 behaves"
