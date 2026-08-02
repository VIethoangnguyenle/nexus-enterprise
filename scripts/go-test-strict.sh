#!/usr/bin/env bash
#
# Runs `go test` in one module and treats a skipped test as a failure.
#
# Why: most of this repo's suites reach for a live Postgres and call t.Skipf
# when it is absent. `go test` reports a package whose every test skipped as
# `ok`, so with the database down the drive service printed "ok" while all 34
# of its tests had been skipped — a green suite that ran nothing. That is how
# the approval store suite stayed broken for weeks without anyone noticing.
#
# A test that cannot run is not a test that passed. If a test genuinely should
# not run in this configuration, put it behind a build tag (see the
# `integration` suites) rather than skipping at runtime, so its absence is
# declared instead of discovered.
#
# Packages with no test files are reported by `go test` as a package-level skip.
# Those are not skipped tests and are ignored here.
set -uo pipefail

DIR="${1:?usage: go-test-strict.sh <module-dir> [go test args...]}"
shift

cd "$DIR" || exit 1

RAW=$(mktemp)
trap 'rm -f "$RAW"' EXIT

go test -json "$@" ./... >"$RAW" 2>/dev/null
GO_STATUS=$?

# Human-readable output, reconstructed from the JSON stream.
python3 - "$RAW" <<'PY'
import json, sys

path = sys.argv[1]
skipped, failed = [], []
with open(path) as fh:
    for line in fh:
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            continue
        action, test = event.get("Action"), event.get("Test")
        if action == "output":
            sys.stdout.write(event.get("Output", ""))
        elif action == "skip" and test:
            skipped.append((event.get("Package", "?"), test))
        elif action == "fail" and test:
            failed.append((event.get("Package", "?"), test))

if skipped:
    print()
    print(f"✗ {len(skipped)} test(s) skipped — a skipped test has not passed:")
    for pkg, test in sorted(set(skipped)):
        print(f"    {pkg.split('/')[-1]}: {test}")
    print()
    print("  These usually mean the database or Redis was unreachable. Start it")
    print("  with `make dev-infra`, or move the test behind a build tag so its")
    print("  absence is declared rather than silently tolerated.")
    sys.exit(2)

sys.exit(1 if failed else 0)
PY
STRICT_STATUS=$?

# A skip (2) is reported even when go itself was happy; otherwise go's own
# verdict wins so compile errors and panics are not masked.
if [ "$STRICT_STATUS" -eq 2 ]; then
	exit 1
fi
exit "$GO_STATUS"
