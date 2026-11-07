#!/usr/bin/env bash
# Benchmarks the messaging queries on a large seeded data set.
#
# Works only on the throwaway database ngac_bench inside the dev Postgres
# container: never on ngac (dev) or ngac_ci (tests), never on production.
#
#   scripts/bench/messages/run.sh [--seed] [--tenants N] [--messages N]
#                                 [--clients N] [--seconds N] [--label NAME] [--out FILE]
#
#   --seed       drop and rebuild ngac_bench (schema from data/, then seed.sql)
#   --label      which query set to run: files queries/*-<label>.sql (default: current)
#   --out        Markdown report to write (default: stdout)
set -euo pipefail

cd "$(dirname "$0")/../../.."
DB=ngac_bench
SEED=0 TENANTS=2000 MESSAGES=50000000 CLIENTS=50 SECONDS_=30 LABEL=current OUT=/dev/stdout
while [ $# -gt 0 ]; do
  case "$1" in
    --seed) SEED=1 ;;
    --tenants) TENANTS=$2; shift ;;
    --messages) MESSAGES=$2; shift ;;
    --clients) CLIENTS=$2; shift ;;
    --seconds) SECONDS_=$2; shift ;;
    --label) LABEL=$2; shift ;;
    --out) OUT=$2; shift ;;
    *) echo "unknown option $1" >&2; exit 2 ;;
  esac
  shift
done

COMPOSE="docker compose"
PG=$($COMPOSE ps -q postgres)
[ -n "$PG" ] || { echo "dev postgres is not running (make dev-infra)" >&2; exit 1; }
psql_() { docker exec -i "$PG" psql -v ON_ERROR_STOP=1 -U ngac -q "$@"; }

if [ "$SEED" = 1 ]; then
  echo "▸ Rebuilding $DB" >&2
  psql_ -d postgres -c "DROP DATABASE IF EXISTS $DB" -c "CREATE DATABASE $DB"
  psql_ -d "$DB" < data/init.sql >/dev/null
  for f in $(ls data/migrations/*.sql | sort); do
    case $(basename "$f") in
      064_directory_visibility.sql) sed '/^INSERT INTO _m064_limits VALUES/s/, [0-9]*);$/, 100000);/' "$f" ;;
      *) cat "$f" ;;
    esac | psql_ -d "$DB" >/dev/null
  done

  # Loading is far faster without the secondary indexes; they are put back after.
  psql_ -d "$DB" -At -c "SELECT indexdef || ';' FROM pg_indexes
    WHERE tablename = 'messages' AND indexname <> 'messages_pkey'" > /tmp/ngac_bench_indexes.sql
  psql_ -d "$DB" -At -c "SELECT 'DROP INDEX ' || indexname || ';' FROM pg_indexes
    WHERE tablename = 'messages' AND indexname <> 'messages_pkey'" | psql_ -d "$DB"

  echo "▸ Seeding $TENANTS tenants, $MESSAGES messages" >&2
  start=$(date +%s)
  psql_ -d "$DB" -v tenants="$TENANTS" -v messages="$MESSAGES" < scripts/bench/messages/seed.sql
  echo "▸ Rebuilding message indexes" >&2
  # Docker gives the container 64 MB of /dev/shm, too little for parallel index builds.
  { echo "SET max_parallel_maintenance_workers = 0; SET maintenance_work_mem = '1GB';"; sed 's/^CREATE \(UNIQUE \)\?INDEX /CREATE \1INDEX IF NOT EXISTS /' /tmp/ngac_bench_indexes.sql; } | psql_ -d "$DB"
  psql_ -d "$DB" -c "VACUUM (PARALLEL 0, ANALYZE)"
  echo "▸ Seeded in $(( $(date +%s) - start )) s" >&2
fi

{
  echo "## Run $(date '+%Y-%m-%d %H:%M') — query set \`$LABEL\`"
  echo
  psql_ -d "$DB" -At -F ' | ' -c "SELECT 'messages', count(*)::text FROM messages
    UNION ALL SELECT 'channels', count(*)::text FROM channels
    UNION ALL SELECT 'size', pg_size_pretty(pg_database_size('$DB'))" | sed 's/^/- /'
  echo "- server: $(psql_ -d "$DB" -At -c "SELECT current_setting('server_version') || ', shared_buffers ' || current_setting('shared_buffers')")"
  echo "- pgbench: $CLIENTS clients, ${SECONDS_} s per query"
  echo
  for q in scripts/bench/messages/queries/*-"$LABEL".sql; do
    name=$(basename "$q" .sql)
    echo "### $name"
    echo
    echo '```'
    # One plan with fixed values for the script's random variables.
    grep -v '^\\set' "$q" | sed 's/^SELECT /EXPLAIN (ANALYZE, BUFFERS, COSTS OFF) SELECT /; s/^WITH /EXPLAIN (ANALYZE, BUFFERS, COSTS OFF) WITH /' \
      | psql_ -d "$DB" -v i=500 -v d=100 -v n=123456 -v s=500000 | grep -E 'Index|Seq Scan|Bitmap|Execution Time|Planning Time|rows=' | awk 'NR <= 40'
    echo '```'
    echo
    docker exec -i "$PG" sh -c "cat > /tmp/q.sql" < "$q"
    docker exec "$PG" sh -c 'rm -rf /tmp/pgb && mkdir /tmp/pgb'
    docker exec -w /tmp/pgb "$PG" pgbench -n -U ngac -d "$DB" -f /tmp/q.sql -c "$CLIENTS" -j 4 -T "$SECONDS_" -l 2>/dev/null \
      | grep -E 'latency average|tps =' | sed 's/^/- /'
    # The per-transaction log's third column is the latency in microseconds.
    docker exec "$PG" sh -c 'cat /tmp/pgb/pgbench_log.* | awk "{print \$3}" | sort -n | awk "{a[NR]=\$1} END {if (NR) printf \"- latency p50 = %.1f ms, p95 = %.1f ms, p99 = %.1f ms\n\", a[int(NR*0.5)]/1000, a[int(NR*0.95)]/1000, a[int(NR*0.99)]/1000}"'
    echo
  done
} > "$OUT"
