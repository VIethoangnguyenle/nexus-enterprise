#!/usr/bin/env bash
#
# Migration 060 must never retype the real #general on a re-run, even after its name
# changed. Copies the test database into a scratch one, renames a #general there,
# re-applies 060 and fails if any channel changed type.
#
# Usage: scripts/test-migration-060-rerun.sh [source-db]   (default ngac_ci; needs docker compose postgres)
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${1:-ngac_ci}"
SCRATCH="ngac_scratch_060"
PG="${PG_CONTAINER:-nexus-enterprise-postgres-1}"
psql_() { docker exec -i "$PG" psql -X -q -A -t -v ON_ERROR_STOP=1 -U ngac "$@"; }

psql_ -d postgres -c "DROP DATABASE IF EXISTS $SCRATCH" -c "CREATE DATABASE $SCRATCH TEMPLATE $SRC"
trap 'psql_ -d postgres -c "DROP DATABASE IF EXISTS $SCRATCH" >/dev/null' EXIT

# The source must already carry 060 (index present); the first apply is the normal path.
psql_ -d "$SCRATCH" <"$ROOT/data/migrations/060_private_groups.sql" >/dev/null

target="$(psql_ -d "$SCRATCH" -c "SELECT id FROM channels WHERE channel_type = 'workspace' ORDER BY created_at LIMIT 1")"
[ -n "$target" ] || { echo "✗ no workspace-wide channel to rename in $SRC" >&2; exit 1; }
psql_ -d "$SCRATCH" -c "UPDATE channels SET name = 'Company-wide renamed' WHERE id = '$target'"

before="$(psql_ -d "$SCRATCH" -c "SELECT id || ':' || channel_type FROM channels ORDER BY id" | md5sum)"
psql_ -d "$SCRATCH" <"$ROOT/data/migrations/060_private_groups.sql" >/dev/null
after="$(psql_ -d "$SCRATCH" -c "SELECT id || ':' || channel_type FROM channels ORDER BY id" | md5sum)"
type="$(psql_ -d "$SCRATCH" -c "SELECT channel_type FROM channels WHERE id = '$target'")"

if [ "$before" != "$after" ] || [ "$type" != "workspace" ]; then
	echo "✗ re-running 060 changed channel types (renamed #general is now '$type')" >&2
	exit 1
fi
echo "✓ 060 re-run left every channel type alone, the renamed #general included"
