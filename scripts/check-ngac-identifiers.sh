#!/usr/bin/env bash
# Fails when Go code outside backend/ngac builds an NGAC identifier by hand.
#
# Every operation string and node name must come from backend/ngac, so that renaming a helper
# is a compile error instead of a silent authorization change (CLAUDE.md §5, "NGAC identifiers").
# A raw "read" compiles forever and quietly stops matching anything the day the vocabulary moves;
# a hand-built `fmt.Sprintf("Dept_%s", name)` keyed by a display name lets two tenants share a node.
#
# Scans non-test, non-generated Go sources and reports:
#   1. a quoted operation literal   ("read", "write", "upload", "approve", "share", "manage",
#                                    "invite", "create_channel") in code, not in a comment
#   2. a node-name pattern          (fmt.Sprintf, concatenation, or a well-known name literal)
#
# A line that is a deliberate exception ends with the marker  // ngac-lint:allow <reason>.
#
# Usage:  scripts/check-ngac-identifiers.sh            scan the repository
#         scripts/check-ngac-identifiers.sh --self-test  prove the scan catches what it claims to
set -uo pipefail
cd "$(dirname "$0")/.."

OPS='read|write|upload|approve|share|manage|invite|create_channel'
PREFIXES='PC|Dept|Ch|Folder|Share|Asset|User|DriveRoot|TenantMember|TenantOwner|Role|U'
SUFFIXES='Owners|Members|Mgmt|Documents|DraftDocs|ApprovedDocs|Channels|Assets|Content|Drive'

# scan <root>... : prints "file:line: text" for every violation under the given roots.
scan() {
  local roots=("$@") files f
  files=$(find "${roots[@]}" -type f -name '*.go' \
    -not -name '*_test.go' -not -name '*.pb.go' -not -name '*.pb.gw.go' \
    -not -path '*backend/ngac/*' -not -path '*/node_modules/*' -not -path '*/.agentkit/*' \
    -not -path '*/.claude/*' \
    -not -path '*backend/services/approval/*' | sort)
  # approval is skipped for now: it still carries "read"/"approve" literals (domain/execution.go,
  # domain/queries.go) and is being reworked by another change. Remove that exclusion, and the
  # literals with it, once that work lands.
  [ -z "$files" ] && return 0
  for f in $files; do
    awk -v file="$f" -v ops="$OPS" -v pre="$PREFIXES" -v suf="$SUFFIXES" '
      {
        line = $0
        if (line ~ /ngac-lint:allow/) next
        # Drop the // comment tail (naive: a "//" inside a string literal is rare enough here).
        code = line
        sub(/[ \t]*\/\/.*$/, "", code)
        if (code ~ /^[ \t]*(\/\*|\*)/) next
        hit = ""
        # Q matches either kind of string quote: a raw (backtick) literal is the same string.
        Q = "[\"`]"
        if (match(code, Q "(" ops ")" Q)) hit = "operation literal " substr(code, RSTART, RLENGTH)
        # fmt.Sprintf / Sprint / Sprintln whose first string starts a platform name, or carries one
        # of the middle/suffix namespaces. The prefix must start the string or follow a non-word
        # character, so MENU_%s is not mistaken for U_%s.
        else if (match(code, "Sprint(f|ln)?\\([ \t]*" Q "([^\"`]*[^A-Za-z0-9])?(" pre ")_")) hit = "node name built with Sprint"
        else if (match(code, "Sprint(f|ln)?\\([ \t]*" Q "[^\"`]*_(" suf "|Category|Type)(_|" Q "|[ \t,])")) hit = "node name built with Sprint"
        else if (match(code, Q "(" pre ")_" Q "[ \t]*\\+")) hit = "node name built by concatenation"
        else if (match(code, "\\+[ \t]*" Q "_(" suf "|Category|Type)(_[^\"`]*)?" Q)) hit = "node name built by concatenation"
        else if (match(code, Q "(PC_Global|PublicUsers|PC_AssetManagement)" Q)) hit = "well-known node name literal"
        if (hit != "") printf "%s:%d: %s: %s\n", file, NR, hit, line
      }' "$f"
  done
}

self_test() {
  local dir rc=0
  dir=$(mktemp -d)
  trap 'rm -rf "$dir"' RETURN
  mkdir -p "$dir/svc" "$dir/backend/ngac" "$dir/backend/services/approval"

  cat >"$dir/svc/bad.go" <<'EOF'
package svc

func bad(id string) {
	check(user, node, "read")
	name := fmt.Sprintf("Dept_%s", id)
	name2 := fmt.Sprintf("%s_Mgmt", id)
	name3 := "Folder_" + id
	name4 := id + "_Owners"
	find("PC_Global")
	raw := check(user, node, `manage`)
	t1 := fmt.Sprintf("%s_Type_%s", ws, id)
	t2 := fmt.Sprintf("%s_Category_%s", ws, cat)
	t3 := fmt.Sprint("Dept_", id)
	t4 := fmt.Sprintln("Ch_", id, "_Drive")
	t5 := fmt.Sprintf(`Folder_%s`, id)
}
EOF
  cat >"$dir/svc/good.go" <<'EOF'
package svc

// "read" in a comment, and fmt.Sprintf("Dept_%s", x) in a comment, are not code.
func good(id string) {
	check(user, node, ngac.OpRead)
	name := ngac.DeptUAName(ngac.DeptID(id))
	key := fmt.Sprintf("drive/%s/%s", id, file)
	allowed := "read" // ngac-lint:allow wire value of an unrelated field
	menu := fmt.Sprintf("MENU_%s", id)
	path := fmt.Sprintf("drive/%s/%s_Report", id, file)
	// `read` and fmt.Sprintf("%s_Type_%s") in a comment are not code either.
}
EOF
  # Excluded by design: the vocabulary itself, tests, generated code, the temporarily skipped service.
  printf 'package ngac\nconst OpRead = "read"\nvar _ = fmt.Sprintf("Dept_%%s", 1)\n' >"$dir/backend/ngac/ops.go"
  printf 'package svc\nvar _ = "read"\n' >"$dir/svc/x_test.go"
  printf 'package svc\nvar _ = "read"\n' >"$dir/svc/x.pb.go"
  printf 'package domain\nvar _ = "approve"\n' >"$dir/backend/services/approval/x.go"

  local out
  out=$(scan "$dir")
  for want in 'bad.go:4:' 'bad.go:5:' 'bad.go:6:' 'bad.go:7:' 'bad.go:8:' 'bad.go:9:' 'bad.go:10:' 'bad.go:11:' 'bad.go:12:' 'bad.go:13:' 'bad.go:14:' 'bad.go:15:'; do
    echo "$out" | grep -q "$want" || { echo "self-test: expected a violation at $want"; rc=1; }
  done
  echo "$out" | grep -q 'good.go' && { echo "self-test: good.go must pass"; echo "$out" | grep good.go; rc=1; }
  for skipped in backend/ngac x_test.go x.pb.go services/approval; do
    echo "$out" | grep -q "$skipped" && { echo "self-test: $skipped must be skipped"; rc=1; }
  done
  [ $rc -eq 0 ] && echo "self-test ok"
  return $rc
}

if [ "${1:-}" = "--self-test" ]; then
  self_test
  exit $?
fi

echo "▸ Checking Go sources for hand-built NGAC identifiers..."
violations=$(scan backend)
if [ -n "$violations" ]; then
  printf '%s\n' "$violations"
  echo ""
  echo "✗ NGAC operations and node names must come from backend/ngac (ngac.Op*, ngac.*Name helpers)."
  echo "  A deliberate exception ends its line with:  // ngac-lint:allow <reason>"
  exit 1
fi
echo "  ✓ no inline operation strings or node-name patterns outside backend/ngac"
