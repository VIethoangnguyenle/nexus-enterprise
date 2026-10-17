#!/usr/bin/env bash
#
# Runs the tests behind the `realpolicy` build tag: tests that talk to the real
# policy service (a process started by backend/testutil, see policy_process.go)
# instead of a fake.
#
# The policy binary is built once, here, so compiling it is not charged to the
# per-package test timeout. Each test binary then starts one policy process
# against TEST_DATABASE_URL, with an unreachable Redis and Kafka.
#
# A skipped test is a failure, as in go-test-strict.sh: asking for the real policy
# and not getting it must never read as green.
#
# Usage: TEST_DATABASE_URL=postgres://... scripts/run-realpolicy-tests.sh
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"

if [ -z "${TEST_DATABASE_URL:-}" ]; then
	echo "✗ TEST_DATABASE_URL is not set: the policy process needs a database (never the dev 'ngac' one — use ngac_ci)." >&2
	exit 1
fi

BIN="$ROOT/.dev-logs/bin/policy"
mkdir -p "$(dirname "$BIN")"
echo "▸ Building policy → $BIN"
(cd "$ROOT/backend/services/policy" && go build -o "$BIN" ./cmd/) || exit 1
export NEXUS_POLICY_BIN="$BIN"

OUT="$(mktemp)"
trap 'rm -f "$OUT"' EXIT

FAIL=0
RAN=0
for mod in "$ROOT/backend" "$ROOT"/backend/services/*; do
	[ -f "$mod/go.mod" ] || continue
	pkgs="$(cd "$mod" && grep -rl --include='*_test.go' '^//go:build .*realpolicy' . 2>/dev/null | xargs -r -n1 dirname | sort -u)"
	[ -n "$pkgs" ] || continue
	RAN=1
	echo "=== ${mod#"$ROOT"/} ==="
	# shellcheck disable=SC2086 # package list is whitespace-separated paths without spaces
	(cd "$mod" && go test -tags realpolicy -count=1 -timeout 180s -v $pkgs) 2>&1 | tee "$OUT"
	status=${PIPESTATUS[0]}
	if [ "$status" -ne 0 ]; then
		FAIL=1
	elif grep -q -- '--- SKIP' "$OUT"; then
		echo "✗ a realpolicy test was skipped — a skipped test has not passed" >&2
		FAIL=1
	fi
done

if [ "$RAN" -eq 0 ]; then
	echo "✗ no test carries the realpolicy tag" >&2
	exit 1
fi
if [ "$FAIL" -ne 0 ]; then
	echo "✗ realpolicy tests failed" >&2
	exit 1
fi
echo "✓ realpolicy tests pass"
