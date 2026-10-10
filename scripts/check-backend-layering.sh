#!/usr/bin/env bash
# Fails when backend code drifts from the shared start-up, error and layering conventions.
#
# These are the rules that kept breaking by copy and paste: eight services each carried their own
# envOr/connectDB/interceptors, so one of them lacked recovery and one lacked logging; four REST
# edges mapped gRPC errors by hand and each missed a code; 500 answers carried the text of the
# database error; the REST edge called the gRPC server in the same process; and store errors were
# dropped on write paths. Each rule below names the shared thing to use instead.
#
# Scans non-test, non-generated Go sources under backend/ and reports:
#   1. a local copy of a shared helper (envOr, connectDB, connectRedis, gracefulShutdown,
#      loggingInterceptor, recoveryInterceptor, mapGRPCError)         -> pkg/bootstrap, grpcauth, httputil
#   2. grpc.NewServer without grpcauth.ServerOptions (no recovery/logging/caller check)
#   3. grpc.NewClient outside pkg/grpcauth                            -> grpcauth.Dial
#   4. an internal/rest file importing the service's internal/grpc    -> call the domain, not the gRPC server
#   5. SQL or a database pool in internal/grpc or internal/rest       -> the store, behind the domain
#   6. a 500 whose body is built by hand, or a gRPC Internal status   -> httputil.Internal / grpcutil.Status
#   7. a store write whose error is dropped (`_ =` or a bare call)    -> handle it, or run it in a transaction
#   8. the old POLICY_ADDR name outside pkg/bootstrap and comments    -> POLICY_SERVICE_ADDR
#
# A line that is a deliberate exception ends with the marker  // layering-lint:allow <reason>.
#
# Usage:  scripts/check-backend-layering.sh              scan the repository
#         scripts/check-backend-layering.sh --self-test  prove the scan catches what it claims to
set -uo pipefail
cd "$(dirname "$0")/.."

# goFiles <root> : non-test, non-generated Go files under root.
goFiles() {
  find "$1" -type f -name '*.go' -not -name '*_test.go' -not -name '*.pb.go' -not -name '*.pb.gw.go' \
    -not -path '*/node_modules/*' -not -path '*/.agentkit/*' | sort
}

# hits <label> <file>... : stdin is "file:line:text" grep output; prints it with the label.
label() { sed "s|^|$1: |"; }


# calls <rule> <file>... : reads Go sources with comments blanked out (line numbers kept) and parses
# the calls the greps below cannot, so a call split over lines, a comment, or an indirection through a
# variable neither hides a violation nor raises a false one.
#   servers  : every grpc.NewServer(...) is built from grpcauth.ServerOptions(...), directly or through
#              an `opts...` variable that was assigned from it
#   handlers : rest.NewHandler(...) never receives a gRPC server (New*Server / Serve* result)
calls() {
  python3 - "$@" <<'PY'
import re, sys

def blank_comments(src):
    out, i, n, mode = [], 0, len(src), None
    while i < n:
        c = src[i]
        if mode is None:
            if src.startswith('//', i):
                j = src.find('\n', i); j = n if j < 0 else j
                out.append(' ' * (j - i)); i = j; continue
            if src.startswith('/*', i):
                j = src.find('*/', i + 2); j = n if j < 0 else j + 2
                out.append(re.sub(r'[^\n]', ' ', src[i:j])); i = j; continue
            if c in '"`\'': mode = c
        else:
            # Inside a string or rune literal: keep the quotes and the line breaks,
            # blank the rest, so text that merely mentions a call is not a call.
            if c == '\\' and mode != '`':
                out.append('  '); i += 2; continue
            if c == mode: mode = None
            else:
                out.append('\n' if c == '\n' else ' '); i += 1; continue
        out.append(c); i += 1
    return ''.join(out)

def call_args(src, start):
    """Text between the parenthesis at src[start] and its match."""
    depth, i, mode = 0, start, None
    while i < len(src):
        c = src[i]
        if mode:
            if c == '\\' and mode != '`': i += 2; continue
            if c == mode: mode = None
        elif c in '"`\'': mode = c
        elif c == '(': depth += 1
        elif c == ')':
            depth -= 1
            if depth == 0: return src[start + 1:i]
        i += 1
    return src[start + 1:]

rule, files = sys.argv[1], sys.argv[2:]
for f in files:
    raw = open(f, encoding='utf-8', errors='replace').read()
    src = blank_comments(raw)
    lines = raw.split('\n')
    def report(pos, msg):
        ln = src.count('\n', 0, pos) + 1
        if 'layering-lint:allow' in lines[ln - 1]: return
        print('%s: %s:%d: %s' % (msg, f, ln, lines[ln - 1].strip()))
    if rule == 'servers':
        for m in re.finditer(r'(?<![\w.])grpc\.NewServer\(', src):
            args = call_args(src, m.end() - 1).strip()
            if 'grpcauth.ServerOptions(' in args: continue
            v = re.fullmatch(r'(\w+)\.\.\.', args)
            if v and re.search(r'\b%s\s*:?=\s*grpcauth\.ServerOptions\(' % re.escape(v.group(1)), src): continue
            report(m.start(), 'grpc.NewServer without grpcauth.ServerOptions (no recovery, logging or caller check)')
    elif rule == 'handlers':
        servers = set(re.findall(r'\b(\w+)\s*:?=\s*[\w.]*\.(?:New\w*Server|Serve\w+)\(', src))
        for m in re.finditer(r'(?<![\w])rest\.NewHandler\(', src):
            args = call_args(src, m.end() - 1)
            direct = re.search(r'\.(?:New\w*Server|Serve\w+)\(', args)
            via = [n for n in servers if re.search(r'\b%s\b' % re.escape(n), args)]
            if direct or via:
                report(m.start(), 'REST handler wired to a gRPC server (give it the domain service)')
PY
}

# scan <root> : prints "rule: file:line: text" for each violation under root (a backend/ directory).
scan() {
  local root=$1 f
  local services=() all=() transport=() rest=() cmds=()
  while IFS= read -r f; do all+=("$f"); done < <(goFiles "$root")
  [ ${#all[@]} -eq 0 ] && return 0
  for f in "${all[@]}"; do
    case "$f" in
      */services/*/internal/grpc/* | */services/*/internal/rest/*) transport+=("$f") ;;
    esac
    case "$f" in
      */services/*/internal/rest/*) rest+=("$f") ;;
      */services/*/cmd/*) cmds+=("$f") ;;
    esac
    case "$f" in */services/*) services+=("$f") ;; esac
  done

  # 1. Local copies of shared helpers.
  if [ ${#services[@]} -gt 0 ]; then
    grep -nHE '^func (envOr|connectDB|connectRedis|gracefulShutdown|loggingInterceptor|recoveryInterceptor|mapGRPCError)\(' \
      "${services[@]}" | grep -v 'layering-lint:allow' | label 'local copy of a shared helper (use pkg/bootstrap, grpcauth, httputil)'
  fi

  # 2. Every gRPC server is built from the shared options (parsed, not grepped).
  calls servers "${all[@]}"

  # 3. Client connections come from grpcauth.Dial.
  grep -nHE 'grpc\.NewClient\(' "${all[@]}" | grep -vE '/pkg/grpcauth/|/testutil/' | grep -v 'layering-lint:allow' |
    label 'grpc.NewClient (use grpcauth.Dial, which forwards the caller)'

  # 4. REST calls the domain, never the gRPC server.
  if [ ${#rest[@]} -gt 0 ]; then
    grep -nHE '"[^"]*/services/[^"/]+/internal/grpc"' "${rest[@]}" | grep -v 'layering-lint:allow' |
      label 'REST imports the gRPC transport (call the domain service instead)'
  fi

  # 4b. ...and no main hands a REST handler a gRPC server.
  if [ ${#cmds[@]} -gt 0 ]; then
    calls handlers "${cmds[@]}"
  fi

  # 5. No SQL in a transport.
  if [ ${#transport[@]} -gt 0 ]; then
    grep -nHE '\.(Exec|Query|QueryRow|Begin|BeginTx|SendBatch)\(|pgxpool|pgx\.Tx' "${transport[@]}" | grep -v 'layering-lint:allow' |
      label 'SQL or a database pool in a transport (move it to the store, behind the domain)'
  fi

  # 6. A 500 never carries the cause; a gRPC Internal never carries the cause.
  if [ ${#services[@]} -gt 0 ]; then
    grep -nHE 'StatusInternalServerError' "${services[@]}" | grep -vE 'InternalMessage|\.Code[ )]|layering-lint:allow' |
      label 'hand-built 500 (use httputil.Internal, which logs the cause and answers a generic body)'
  fi
  # ...built by number too, or with the cause in the message.
  grep -nHE '(NewHTTPError|\.JSON|\.String|\.Blob|\.NoContent|apiError)\([^)]*\b500\b' "${all[@]}" |
    grep -vE '/pkg/httputil/|InternalMessage|layering-lint:allow' |
    label 'hand-built 500 by number (use httputil.Internal)'
  grep -nHE 'status\.(New|Errorf?)\(codes\.(Internal|Unknown)' "${all[@]}" | grep -vE '/pkg/grpcauth/|layering-lint:allow' |
    label 'gRPC Internal built by hand (use grpcauth.Internal or grpcutil.Status)'

  # 7. Store writes whose error is dropped.
  if [ ${#services[@]} -gt 0 ]; then
    local verbs='Insert|Update|Delete|Set|Increment|Decrement|Track|Add|Remove|Mark|Upsert|Exec|Create|Revoke|Append|Record|Put|Save|Release|Activate'
    local owners='store|db|st|pool|tx|members|directory|invitations|deptStore'
    grep -nHE "^[[:space:]]*_,? ?_? ?:?= .*\b($owners)\.($verbs)[A-Za-z]*\(" "${services[@]}" | grep -v 'layering-lint:allow' |
      label 'store write error assigned to _ (handle it, or run the writes in one transaction)'
    grep -nHE "^[[:space:]]*[A-Za-z_.()]*\b($owners)\.($verbs)[A-Za-z]*\(" "${services[@]}" |
      grep -vE ':[0-9]+:[[:space:]]*(return|go|defer|if|for|switch|case|\}|//)|layering-lint:allow' |
      grep -vE '\) *\{ *$|, *err *:?=|err *:?= ' |
      label 'store write whose error is not checked (handle it, or run the writes in one transaction)'
  fi

  # 8. One name for the policy address.
  grep -nHE 'POLICY_ADDR' "${all[@]}" | grep -vE '/pkg/bootstrap/|layering-lint:allow' |
    label 'POLICY_ADDR is the old name (use POLICY_SERVICE_ADDR via bootstrap.PolicyAddr)'
}

# configs <root> : the deployment files under root that name the policy address.
configs() {
  local root=${1:-.}
  ( cd "$root" && grep -nHE 'POLICY_ADDR' docker-compose.yml Procfile.dev Makefile .env.example scripts/*.sh 2>/dev/null ) |
    grep -vE '^[^:]+:[0-9]+:[[:space:]]*#|check-backend-layering.sh' |
    label 'POLICY_ADDR is the old name (use POLICY_SERVICE_ADDR)'
}

self_test() {
  local dir rc=0 out
  dir=$(mktemp -d)
  trap 'rm -rf "$dir"' RETURN
  local svc="$dir/backend/services/demo"
  mkdir -p "$svc/cmd" "$svc/internal/grpc" "$svc/internal/rest" "$svc/internal/domain" "$dir/backend/pkg/grpcauth" "$dir/backend/pkg/bootstrap"

  cat >"$svc/cmd/main.go" <<'EOF'
package main

func envOr(k, d string) string { return d }
func connectDB() {}
func gracefulShutdown() {}
func recoveryInterceptor() {}
func mapGRPCError() {}

func main() {
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(x))
	conn, _ := grpc.NewClient(addr)
}
EOF
  cat >"$svc/internal/rest/handler.go" <<'EOF'
package rest

import (
	agrpc "ngac-platform/services/demo/internal/grpc"
)

func h() error {
	return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
}
EOF
  cat >"$svc/internal/grpc/server.go" <<'EOF'
package grpc

func s() error {
	rows, err := s.db.Query(ctx, "SELECT 1")
	return status.Errorf(codes.Internal, "boom: %v", err)
}
EOF
  cat >"$svc/internal/domain/service.go" <<'EOF'
package domain

func d() {
	_ = s.store.InsertThing(ctx, t)
	s.store.UpdateThing(ctx, t)
	s.store.IncrementReplyCount(ctx, id)
	s.store.Exec(ctx,
		"UPDATE x")
	addr := os.Getenv("POLICY_ADDR")
}
EOF
  cat >"$svc/internal/domain/good.go" <<'EOF'
package domain

func good() error {
	if err := s.store.InsertThing(ctx, t); err != nil {
		return err
	}
	err := s.store.UpdateThing(ctx, t)
	if err != nil {
		return err
	}
	return s.store.DeleteThing(ctx, id)
}

func quiet() {
	_ = s.store.Close() // layering-lint:allow nothing to do about it
	srv := grpc.NewServer(grpcauth.ServerOptions(p)...)
	he := echo.NewHTTPError(http.StatusInternalServerError, httputil.InternalMessage)
	v, _ := s.store.GetThing(ctx, id)
	// s.store.UpdateThing(ctx, t) in a comment is not code
}
EOF
  # Bypasses of rules 2, 4 and 6 that a same-line text match cannot see.
  mkdir -p "$dir/backend/services/viaopts/cmd" "$dir/backend/services/okopts/cmd" "$dir/backend/services/wired/cmd"
  cat >"$dir/backend/services/viaopts/cmd/main.go" <<'EOF'
package main

func main() {
	opts := []grpc.ServerOption{grpc.UnaryInterceptor(x)}
	srv := grpc.NewServer(opts...)
	other := grpc.NewServer(
		grpc.ChainUnaryInterceptor(y),
	)
}
EOF
  cat >"$dir/backend/services/okopts/cmd/main.go" <<'EOF'
package main

func main() {
	// srv := grpc.NewServer(grpc.ChainUnaryInterceptor(x)) is only a comment
	opts := grpcauth.ServerOptions(grpcauth.ServerPolicy{})
	srv := grpc.NewServer(opts...)
	multi := grpc.NewServer(
		grpcauth.ServerOptions(
			policy,
		)...,
	)
	/* grpc.NewServer(nothing) */
	msg := "grpc.NewServer(in a string)"
}
EOF
  cat >"$dir/backend/services/wired/cmd/main.go" <<'EOF'
package main

func main() {
	srv := agrpc.NewAssetServer(store)
	restHandler := rest.NewHandler(srv)
	inline := rest.NewHandler(
		other,
		agrpc.NewThingServer(store),
	)
	fine := rest.NewHandler(domainSvc, notifications)
	// rest.NewHandler(srv) in a comment
}
EOF
  cat >"$dir/backend/services/demo/internal/rest/by_number.go" <<'EOF'
package rest

func f(c echo.Context) error {
	c.JSON(500, map[string]string{"error": err.Error()})
	return echo.NewHTTPError(500, err.Error())
}

func g() error {
	return echo.NewHTTPError(http.StatusBadRequest, "x")
}
EOF
  cat >"$dir/backend/services/demo/internal/grpc/unknown.go" <<'EOF'
package grpc

func h() error {
	a := status.New(codes.Internal, err.Error())
	return status.Error(codes.Unknown, fmt.Sprintf("%v", err))
}
EOF
  printf 'package main\nfunc envOr() {}\nvar _ = "POLICY_ADDR"\n' >"$dir/backend/pkg/bootstrap/boot.go"
  printf 'package grpcauth\nfunc d() { grpc.NewClient(a); status.Error(codes.Internal, InternalMessage) }\n' >"$dir/backend/pkg/grpcauth/x.go"
  printf 'package svc\nfunc envOr() {}\n' >"$svc/internal/domain/x_test.go"

  out=$(scan "$dir/backend")
  want() { echo "$out" | grep -q "$1" || { echo "self-test: expected a violation matching: $1"; rc=1; }; }
  not()  { echo "$out" | grep -q "$1" && { echo "self-test: must not be flagged: $1"; echo "$out" | grep "$1"; rc=1; }; }

  want 'local copy.*main.go:3:'
  want 'local copy.*main.go:4:'
  want 'local copy.*main.go:5:'
  want 'local copy.*main.go:6:'
  want 'local copy.*main.go:7:'
  want 'grpc.NewServer without.*main.go:10:'
  want 'grpc.NewClient.*main.go:11:'
  want 'REST imports the gRPC transport.*rest/handler.go:4:'
  want 'hand-built 500.*rest/handler.go:8:'
  want 'SQL or a database pool.*grpc/server.go:4:'
  want 'gRPC Internal built by hand.*grpc/server.go:5:'
  want 'assigned to _.*service.go:4:'
  want 'not checked.*service.go:5:'
  want 'not checked.*service.go:6:'
  want 'not checked.*service.go:7:'
  want 'POLICY_ADDR is the old name.*service.go:9:'
  want 'grpc.NewServer without.*viaopts/cmd/main.go:5:'
  want 'grpc.NewServer without.*viaopts/cmd/main.go:6:'
  not 'okopts/cmd/main.go'
  want 'REST handler wired to a gRPC server.*wired/cmd/main.go:5:'
  want 'REST handler wired to a gRPC server.*wired/cmd/main.go:6:'
  not 'wired/cmd/main.go:(9|11):'
  want 'by number.*by_number.go:4:'
  want 'by number.*by_number.go:5:'
  not 'by_number.go:9:'
  want 'built by hand.*unknown.go:4:'
  want 'built by hand.*unknown.go:5:'
  not 'good.go'
  not 'bootstrap/boot.go'
  not 'grpcauth/x.go'
  not 'x_test.go'

  # The deployment files: the old name is flagged, a comment about it is not.
  mkdir -p "$dir/cfg/scripts"
  printf 'services:\n  approval:\n    environment:\n      POLICY_ADDR: policy:50051\n' >"$dir/cfg/docker-compose.yml"
  printf '# the old POLICY_ADDR is still read\nPOLICY_SERVICE_ADDR=localhost:50051\n' >"$dir/cfg/.env.example"
  printf 'dev:\n\tPOLICY_ADDR=$$X sh run\n' >"$dir/cfg/Makefile"
  printf 'POLICY_ADDR=x go run\n' >"$dir/cfg/Procfile.dev"
  out=$(configs "$dir/cfg")
  echo "$out" | grep -q 'docker-compose.yml:4' || { echo "self-test: docker-compose.yml must be flagged"; rc=1; }
  echo "$out" | grep -q 'Makefile:2' || { echo "self-test: Makefile must be flagged"; rc=1; }
  echo "$out" | grep -q 'Procfile.dev:1' || { echo "self-test: Procfile.dev must be flagged"; rc=1; }
  echo "$out" | grep -q '.env.example' && { echo "self-test: a comment naming the old variable must pass"; rc=1; }
  [ $rc -eq 0 ] && echo "self-test ok"
  return $rc
}

if [ "${1:-}" = "--self-test" ]; then
  self_test
  exit $?
fi

echo "▸ Checking backend code for the shared start-up, error and layering conventions..."
violations=$( { scan backend; configs .; } )
if [ -n "$violations" ]; then
  printf '%s\n' "$violations"
  echo ""
  echo "✗ Use the shared packages (backend/pkg/bootstrap, grpcauth, grpcutil, httputil, policyclient)"
  echo "  and keep transport → domain → store. A deliberate exception ends its line with:"
  echo "  // layering-lint:allow <reason>"
  exit 1
fi
echo "  ✓ shared helpers, gRPC options, REST→domain, no SQL in transports, no dropped store errors"
