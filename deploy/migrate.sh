#!/bin/sh
# Database migration runner with a ledger. Runs as the one-shot `migrate` service of
# docker-compose.prod.yml (postgres:16-alpine, so psql / pg_dump match the server).
#
#   /db/init.sql          base schema, applied once (ledger name "init.sql")
#   /db/migrations/*.sql  numbered chain, applied in sorted order, each once
#   /backups              pg_dump target; the newest BACKUP_KEEP dumps are kept
#
# Ledger: schema_migrations(filename, checksum, applied_at). A recorded file whose checksum
# changed is a hard error — fix forward with a new migration, never edit an applied one.
#
# Transactions: a file runs with its ledger row in one transaction (psql --single-transaction)
# unless it cannot: it manages its own BEGIN/COMMIT, uses a statement that is illegal inside a
# transaction (CREATE [UNIQUE] INDEX CONCURRENTLY, ALTER TYPE ... ADD VALUE, VACUUM,
# CREATE DATABASE), or carries the marker line `-- migrate:no-transaction`. Those run as-is and
# the ledger row is written after, so a crash in between re-runs the file on the next deploy:
# such files MUST be idempotent (IF NOT EXISTS, guarded DO blocks, ...).
#
# When anything was applied, /backups/.migrations-applied is created; deploy.sh uses it to
# restart the policy services, which hold the authorization graph in memory.
#
# Connection comes from the standard PG* variables (PGHOST, PGUSER, PGPASSWORD, PGDATABASE).
set -eu

DB_DIR=${DB_DIR:-/db}
BACKUP_DIR=${BACKUP_DIR:-/backups}
BACKUP_KEEP=${BACKUP_KEEP:-14}
WAIT_SECONDS=${WAIT_SECONDS:-60}

log() { printf '[migrate] %s\n' "$*"; }
die() { printf "[migrate] ERROR: %s\n" "$1" >&2; exit "${2:-1}"; }

# -X: ignore psqlrc. -q: no notices chatter. -A -t: bare values for scripting.
psqlq() { psql -X -q -A -t -v ON_ERROR_STOP=1 "$@"; }

[ -f "$DB_DIR/init.sql" ] || die "missing $DB_DIR/init.sql"
[ -d "$DB_DIR/migrations" ] || die "missing $DB_DIR/migrations"
case "$BACKUP_KEEP" in '' | *[!0-9]*) die "BACKUP_KEEP must be a number" ;; esac

# Wait for the server (compose already gates on the healthcheck; this covers manual runs).
waited=0
until pg_isready -q; do
  [ "$waited" -lt "$WAIT_SECONDS" ] || die "database not ready after ${WAIT_SECONDS}s"
  sleep 2
  waited=$((waited + 2))
done

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# --- what the ledger already knows ----------------------------------------------------------
: >"$work/recorded"
if [ "$(psqlq -c "SELECT to_regclass('public.schema_migrations') IS NOT NULL")" = "t" ]; then
  psqlq -F ' ' -c "SELECT filename, checksum FROM schema_migrations ORDER BY filename" >"$work/recorded"
fi

# --- classify every file: verify recorded checksums, collect the pending ---------------------
: >"$work/pending"
bad=0
check_file() { # $1 = ledger name, $2 = path
  name=$1
  path=$2
  case "$name" in
    *[!A-Za-z0-9._-]*) die "unsafe migration file name: $name" ;;
  esac
  sum=$(sha256sum "$path" | cut -d' ' -f1)
  rec=$(awk -v n="$name" '$1 == n { print $2 }' "$work/recorded")
  if [ -z "$rec" ]; then
    printf '%s %s %s\n' "$name" "$sum" "$path" >>"$work/pending"
  elif [ "$rec" != "$sum" ]; then
    log "CHECKSUM CHANGED: $name (recorded $(printf %s "$rec" | cut -c1-12)..., file $(printf %s "$sum" | cut -c1-12)...)" >&2
    bad=1
  fi
}

check_file init.sql "$DB_DIR/init.sql"
for f in $(ls "$DB_DIR"/migrations/*.sql | sort); do
  check_file "$(basename "$f")" "$f"
done

# Recorded files that vanished from the repo are suspicious but not fatal.
while read -r name _; do
  [ -n "$name" ] || continue
  if [ "$name" != init.sql ] && [ ! -f "$DB_DIR/migrations/$name" ]; then
    log "warning: ledger lists $name but the file is no longer shipped"
  fi
done <"$work/recorded"

[ "$bad" -eq 0 ] || die "an applied migration was edited; refusing to continue. Add a new migration instead." 3

if [ ! -s "$work/pending" ]; then
  log "up to date: nothing to apply"
  exit 0
fi
log "pending: $(awk '{ printf "%s ", $1 }' "$work/pending")"

# --- backup before touching anything ---------------------------------------------------------
tables=$(psqlq -c "SELECT count(*) FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema')")
if [ "$tables" -gt 0 ]; then
  mkdir -p "$BACKUP_DIR"
  stamp=$(date -u +%Y%m%dT%H%M%SZ)
  dump="$BACKUP_DIR/nexus-$stamp.dump"
  log "backing up to $dump"
  if ! pg_dump -Fc -f "$dump.partial"; then
    rm -f "$dump.partial"
    die "pg_dump failed; not migrating"
  fi
  mv "$dump.partial" "$dump"
  # Keep the newest BACKUP_KEEP dumps (names sort chronologically).
  ls -1 "$BACKUP_DIR"/nexus-*.dump 2>/dev/null | sort -r | tail -n +"$((BACKUP_KEEP + 1))" | while read -r old; do
    log "pruning $old"
    rm -f "$old"
  done
else
  log "empty database: nothing to back up"
fi

# --- ledger ---------------------------------------------------------------------------------
psqlq -c "CREATE TABLE IF NOT EXISTS schema_migrations (
  filename   TEXT        PRIMARY KEY,
  checksum   TEXT        NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)"

needs_own_transaction() {
  grep -qiE '^[[:space:]]*(begin|start[[:space:]]+transaction)[[:space:]]*;' "$1" ||
    grep -qiE 'create[[:space:]]+(unique[[:space:]]+)?index[[:space:]]+concurrently|alter[[:space:]]+type[^;]*add[[:space:]]+value|^[[:space:]]*vacuum|create[[:space:]]+database|^-- migrate:no-transaction' "$1"
}

while read -r name sum path; do
  record="INSERT INTO schema_migrations (filename, checksum) VALUES ('$name', '$sum')"
  if needs_own_transaction "$path"; then
    log "applying $name (own transaction handling)"
    psqlq -f "$path"
    psqlq -c "$record"
  else
    log "applying $name"
    psqlq --single-transaction -f "$path" -c "$record"
  fi
done <"$work/pending"

mkdir -p "$BACKUP_DIR"
: >"$BACKUP_DIR/.migrations-applied"
log "done"
