#!/usr/bin/env bash
# Fails when CLAUDE.md drifts from the code it describes.
#
# CLAUDE.md is loaded into every agent session, and an agent trusts it over the repo. A stale
# claim there is worse than no claim: it produces confident wrong work. This script checks the
# facts that are cheap to verify and expensive to get wrong.
set -uo pipefail
cd "$(dirname "$0")/.."

FAIL=0
ok()   { printf '  \033[32m✓\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$1"; FAIL=1; }

echo "▸ Checking CLAUDE.md against the codebase..."

# --- Go version -------------------------------------------------------------
GO_MOD_VER=$(grep -m1 '^go ' backend/go.mod | awk '{print $2}' | cut -d. -f1,2)
if grep -q "Go $GO_MOD_VER" CLAUDE.md; then
  ok "Go version ($GO_MOD_VER) matches backend/go.mod"
else
  bad "CLAUDE.md does not state 'Go $GO_MOD_VER' (backend/go.mod says $GO_MOD_VER)"
fi

# --- Module count -----------------------------------------------------------
MODS=$(find . -name go.mod -not -path '*/node_modules/*' | wc -l | tr -d ' ')
if [ "$MODS" -eq 9 ] && grep -q 'Nine Go modules' CLAUDE.md; then
  ok "module count ($MODS) matches the 'Nine Go modules' claim"
else
  bad "found $MODS go.mod files but CLAUDE.md claims nine — update both"
fi

# --- Service list -----------------------------------------------------------
SVC_DIRS=$(ls -d backend/services/*/ 2>/dev/null | wc -l | tr -d ' ')
MK_SVCS=$(grep -m1 '^SERVICES :=' Makefile | cut -d= -f2 | wc -w | tr -d ' ')
if [ "$SVC_DIRS" -eq "$MK_SVCS" ]; then
  ok "service directories ($SVC_DIRS) match Makefile SERVICES"
else
  bad "$SVC_DIRS service directories but Makefile SERVICES lists $MK_SVCS"
fi

# --- Frontend majors --------------------------------------------------------
check_major() { # $1=package  $2=label
  local want got
  want=$(grep -oE "\"$1\": \"[~^]?[0-9]+" frontend/package.json | head -1 | grep -oE '[0-9]+$')
  got=$(grep -oE "$2 $want\b" CLAUDE.md | head -1)
  if [ -n "$want" ] && [ -n "$got" ]; then
    ok "$2 $want matches package.json"
  else
    bad "CLAUDE.md does not state '$2 $want' (package.json has major $want)"
  fi
}
check_major react React
check_major tailwindcss Tailwind

# --- Make targets referenced in CLAUDE.md exist -----------------------------
MISSING=""
for t in dev dev-stop run dev-infra build-check test db-migrate proto; do
  grep -qE "^$t:" Makefile || MISSING="$MISSING $t"
done
if [ -z "$MISSING" ]; then
  ok "every make target named in CLAUDE.md exists"
else
  bad "CLAUDE.md names missing make targets:$MISSING"
fi

# --- Files CLAUDE.md points at ---------------------------------------------
MISSING=""
for f in backend/ngac/ngac_ops.go \
         backend/services/policy/internal/ngac/pip_store.go \
         backend/services/policy/internal/ngac/pdp_decision_engine.go \
         frontend/vite.config.js \
         data/init.sql \
         .stitch/DESIGN.md; do
  [ -e "$f" ] || MISSING="$MISSING $f"
done
if [ -z "$MISSING" ]; then
  ok "every file path cited in CLAUDE.md exists"
else
  bad "CLAUDE.md cites missing paths:$MISSING"
fi

# --- Spec index matches the directory --------------------------------------
IDX_MISSING=""
for d in docs/specs/*/; do
  n=$(basename "$d")
  grep -q "\`$n\`" docs/specs/README.md || IDX_MISSING="$IDX_MISSING $n"
done
if [ -z "$IDX_MISSING" ]; then
  ok "every spec appears in the docs/specs index"
else
  bad "specs missing from docs/specs/README.md index:$IDX_MISSING"
fi

# --- NGAC operation count ---------------------------------------------------
OPS=$(sed -n '/^const (/,/^)/p' backend/ngac/ngac_ops.go | grep -cE '^[[:space:]]+Op[A-Za-z]+[[:space:]]*=[[:space:]]*"')
if [ "$OPS" -eq 8 ] && grep -q 'Eight fixed operations' CLAUDE.md; then
  ok "NGAC operation count ($OPS) matches the 'Eight fixed operations' claim"
else
  bad "ngac_ops.go defines $OPS operations but CLAUDE.md claims eight"
fi

echo ""
if [ "$FAIL" -eq 0 ]; then
  echo "✓ CLAUDE.md is consistent with the codebase"
else
  echo "✗ CLAUDE.md has drifted — fix the document or the code before continuing"
fi
exit $FAIL
