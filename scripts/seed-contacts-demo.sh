#!/usr/bin/env bash
#
# Seeds a demo organization for Danh bạ on the LOCAL DEV stack, through the real APIs.
#
# DEV ONLY. It signs people in with the documented test-only fixed one-time code, so it
# refuses to run unless the target is this machine and the auth service says that mode is
# on (GET /api/auth/providers -> otp_fixed_code). It must never be pointed at a shared or
# production environment, and nothing here makes it able to: every guard below fails
# closed.
#
# What it builds (all through the REST API, so every person has a user node under
# <workspace>_Members and a directory node set exists for every department):
#   - the owner, who creates the organization "NovaPay (demo danh bạ)"
#   - departments: Khối Công nghệ -> {Đối soát, Nền tảng}, Khối Vận hành -> {Đối soát},
#     Pháp chế -> {Tuân thủ}  (two "Đối soát" in different branches on purpose)
#   - a custom role "Cộng tác viên"
#   - people with names and titles, invited by email and accepting the invitation
#   - one hidden department (Pháp chế) and one limited role (Cộng tác viên), set through the
#     directory rule endpoints
#
# Idempotent: it looks before it creates and skips what exists, so a second run creates
# nothing. It never touches the old a1000000-* rows and writes nothing to ngac_*.
#
# The one statement it sends to the database is `UPDATE users SET email_verified_at`, for
# its own @example.test addresses: creating an organization and accepting an invitation both
# need a verified address, and the fixed one-time code (by design) proves no address, so no
# API could verify a made-up one. It touches no other column and no other table.
#
# Usage:
#   scripts/seed-contacts-demo.sh            # needs `make dev` (vite on :5173, all services)
#   SEED_BASE_URL=http://localhost:5173 scripts/seed-contacts-demo.sh
#
# Environment (all optional):
#   SEED_BASE_URL         where the Vite proxy listens (default http://localhost:5173; must be local)
#   AUTH_FIXED_OTP_CODE   the fixed code, if the dev stack was started with another than the default
#   SEED_PG_CONTAINER     the dev Postgres container (default: the one publishing :5433)
set -euo pipefail

BASE="${SEED_BASE_URL:-http://localhost:5173}"
CODE="${AUTH_FIXED_OTP_CODE:-999999}"
ORG_NAME="NovaPay (demo danh bạ)"
ROLE_NAME="Cộng tác viên"
LIMITED_ROLE="$ROLE_NAME"
HIDDEN_DEPT="Pháp chế"

# email | display name | title | department path ("" = none) | role ("" = none)
OWNER="duc.tran@example.test|Trần Minh Đức|Giám đốc điều hành||"
PEOPLE=(
	"vinh.le@example.test|Lê Quang Vinh|Kỹ sư nền tảng|Khối Công nghệ/Nền tảng|"
	"bao.hoang@example.test|Hoàng Gia Bảo|Chuyên viên đối soát|Khối Công nghệ/Đối soát|"
	"lan.nguyen@example.test|Nguyễn Thu Lan|Cộng tác viên vận hành|Khối Vận hành/Đối soát|$ROLE_NAME"
	"tung.vo@example.test|Võ Thanh Tùng|Chuyên viên đối soát|Khối Vận hành/Đối soát|"
	"yen.pham@example.test|Phạm Hải Yến|Chuyên viên tuân thủ|Pháp chế/Tuân thủ|"
	"linh.do@example.test|Đỗ Thùy Linh|Trợ lý||"
)
DEPARTMENTS=(
	"Khối Công nghệ" "Khối Công nghệ/Đối soát" "Khối Công nghệ/Nền tảng"
	"Khối Vận hành" "Khối Vận hành/Đối soát"
	"Pháp chế" "Pháp chế/Tuân thủ"
)

die() { echo "✗ $*" >&2; exit 1; }
say() { echo "▸ $*"; }

command -v curl >/dev/null || die "curl is required"
command -v jq >/dev/null || die "jq is required"

# --- guards: development, local, fixed-code mode ------------------------------------------
# APP_ENV from the environment, else from the dev env file (read for that one key only).
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
env_name="${APP_ENV:-}"
if [ -z "$env_name" ] && [ -f "$ROOT/.env.dev" ]; then
	env_name="$(grep -m1 '^APP_ENV=' "$ROOT/.env.dev" | cut -d= -f2- | tr -d "\"' " || true)"
fi
case "$env_name" in
	dev | development) ;;
	*) die "APP_ENV is '${env_name:-unset}', not a development environment (set APP_ENV=dev or use the dev env file). This script is for the local dev stack only." ;;
esac
case "$BASE" in
	http://localhost:* | http://127.0.0.1:* | http://localhost | http://127.0.0.1) ;;
	*) die "SEED_BASE_URL must be this machine (http://localhost:PORT); got '$BASE'. This script is for the local dev stack only." ;;
esac
TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

# call METHOD PATH [JSON-BODY] [TOKEN]: prints the body; fails on anything but 2xx.
call() {
	local method="$1" path="$2" body="${3:-}" token="${4:-}" code
	local args=(-sS -o "$TMP" -w '%{http_code}' -X "$method" "$BASE$path" -H 'Content-Type: application/json')
	[ -n "$token" ] && args+=(-H "Authorization: Bearer $token")
	[ -n "$body" ] && args+=(--data "$body")
	code="$(curl "${args[@]}")" || die "cannot reach $BASE (is 'make dev' running?)"
	case "$code" in
		2??) cat "$TMP" ;;
		*) die "$method $path -> HTTP $code: $(head -c 300 "$TMP")" ;;
	esac
}

providers="$(curl -sS "$BASE/api/auth/providers")" || die "cannot reach $BASE (is 'make dev' running?)"
[ "$(jq -r '.otp_fixed_code // false' <<<"$providers")" = "true" ] ||
	die "the auth service at $BASE is not in fixed-one-time-code mode; this script only runs against the local dev stack."

PG="${SEED_PG_CONTAINER:-$(docker ps --filter publish=5433 --format '{{.Names}}' | head -n1)}"
[ -n "$PG" ] || die "no Postgres container publishes :5433; start the dev infra with 'make dev-infra'"

# --- helpers -----------------------------------------------------------------------------
declare -A TOKEN USERID NODE

field() { echo "$1" | cut -d'|' -f"$2"; }

# sign_in EMAIL: signs the account in (creating it on first use) and keeps its token.
sign_in() {
	local email="$1" session out
	session="$(call POST /api/auth/otp/request "$(jq -n --arg e "$email" '{identifier:$e,type:"email"}')" | jq -r .session_id)"
	out="$(call POST /api/auth/otp/verify "$(jq -n --arg s "$session" --arg c "$CODE" '{session_id:$s,code:$c}')")"
	TOKEN[$email]="$(jq -r .access_token <<<"$out")"
	USERID[$email]="$(jq -r .user.id <<<"$out")"
	NODE[$email]="$(jq -r .user.ngac_node_id <<<"$out")"
	[ -n "${TOKEN[$email]}" ] && [ "${TOKEN[$email]}" != null ] || die "sign-in of $email gave no token"
}

# verify_address EMAIL: marks one of this script's own addresses verified (see the header).
verify_address() {
	local email="$1"
	case "$email" in *@example.test) ;; *) die "refusing to verify $email: not a seeded address" ;; esac
	# The address goes in as a psql variable (quoted by psql as a literal), never spliced into the SQL text.
	printf '%s\n' "UPDATE users SET email_verified_at = NOW() WHERE lower(email) = lower(:'email') AND email_verified_at IS NULL" |
		docker exec -i "$PG" psql -U ngac -d ngac -v ON_ERROR_STOP=1 -v "email=$email" -At >/dev/null
}

set_profile() {
	local email="$1" name="$2" title="$3"
	call PATCH /api/me/profile "$(jq -n --arg n "$name" --arg t "$title" '{display_name:$n,title:$t}')" "${TOKEN[$email]}" >/dev/null
}

# --- owner and organization ----------------------------------------------------------------
OWNER_EMAIL="$(field "$OWNER" 1)"
say "owner $OWNER_EMAIL"
sign_in "$OWNER_EMAIL"
set_profile "$OWNER_EMAIL" "$(field "$OWNER" 2)" "$(field "$OWNER" 3)"
verify_address "$OWNER_EMAIL" # creating an organization needs a verified address, as inviting does
OT="${TOKEN[$OWNER_EMAIL]}"

WS="$(call GET /api/me/workspaces "" "$OT" | jq -r --arg n "$ORG_NAME" '[.workspaces[] | select(.name==$n and .type=="organization")][0].id // empty')"
if [ -z "$WS" ]; then
	say "creating organization '$ORG_NAME'"
	WS="$(call POST /api/me/workspaces "$(jq -n --arg n "$ORG_NAME" '{name:$n,type:"organization"}')" "$OT" | jq -r .id)"
else
	say "organization exists"
fi
[ -n "$WS" ] && [ "$WS" != null ] || die "no workspace id"

# --- departments (parents first) -------------------------------------------------------------
dept_paths() {
	call GET "/api/workspaces/$WS/departments" "" "$OT" | jq -r '
		.departments as $d
		| def path($x): if ($x.parent_id // "") == "" then $x.name else (path($d[] | select(.id == $x.parent_id)) + "/" + $x.name) end;
		$d[] | "\(path(.))\t\(.id)"'
}
declare -A DEPT
load_depts() { DEPT=(); while IFS=$'\t' read -r p id; do DEPT[$p]="$id"; done < <(dept_paths); }
load_depts
for path in "${DEPARTMENTS[@]}"; do
	[ -n "${DEPT[$path]:-}" ] && continue
	name="${path##*/}"
	parent=""
	[[ "$path" == */* ]] && parent="${DEPT[${path%/*}]}"
	say "department $path"
	call POST "/api/workspaces/$WS/departments" "$(jq -n --arg n "$name" --arg p "$parent" '{name:$n,parent_id:$p}')" "$OT" >/dev/null
	load_depts
done

# --- role -------------------------------------------------------------------------------------
role_id() { call GET "/api/workspaces/$WS/roles" "" "$OT" | jq -r --arg n "$1" '[.roles[] | select(.name==$n)][0].id // empty'; }
ROLE="$(role_id "$ROLE_NAME")"
if [ -z "$ROLE" ]; then
	say "role $ROLE_NAME"
	call POST "/api/workspaces/$WS/roles" "$(jq -n --arg n "$ROLE_NAME" '{name:$n}')" "$OT" >/dev/null
	ROLE="$(role_id "$ROLE_NAME")"
fi
[ -n "$ROLE" ] || die "no role id"

# --- people: sign in, invite, accept ------------------------------------------------------------
member_node() { # EMAIL -> the person's node id if they are a member, else empty
	call GET "/api/workspaces/$WS/admin/members" "" "$OT" |
		jq -r --arg e "$1" '[.members[] | select((.email | ascii_downcase) == ($e | ascii_downcase))][0].ngac_node_id // empty'
}

for row in "${PEOPLE[@]}"; do
	email="$(field "$row" 1)"
	name="$(field "$row" 2)"
	title="$(field "$row" 3)"
	say "person $name <$email>"
	sign_in "$email"
	set_profile "$email" "$name" "$title"
	if [ -z "$(member_node "$email")" ]; then
		verify_address "$email"
		call POST "/api/workspaces/$WS/members" "$(jq -n --arg e "$email" '{email:$e}')" "$OT" >/dev/null
		inv="$(call GET /api/invitations "" "${TOKEN[$email]}" | jq -r --arg n "$ORG_NAME" '[.invitations[] | select(.workspace_name==$n)][0].id // empty')"
		[ -n "$inv" ] || die "no invitation to '$ORG_NAME' was offered to $email"
		call POST "/api/invitations/$inv/accept" "" "${TOKEN[$email]}" >/dev/null
	fi
done

# --- department and role assignment (idempotent PUTs) -----------------------------------------------
for row in "${PEOPLE[@]}"; do
	email="$(field "$row" 1)"
	dept="$(field "$row" 4)"
	role="$(field "$row" 5)"
	node="$(member_node "$email")"
	[ -n "$node" ] || die "$email is not a member after accepting"
	if [ -n "$dept" ]; then
		[ -n "${DEPT[$dept]:-}" ] || die "unknown department '$dept'"
		call PUT "/api/workspaces/$WS/members/$node/department" "$(jq -n --arg d "${DEPT[$dept]}" '{department_id:$d}')" "$OT" >/dev/null
	fi
	if [ -n "$role" ]; then
		call PUT "/api/workspaces/$WS/members/$node/roles/$ROLE" "" "$OT" >/dev/null
	fi
done

# --- the two directory rules ---------------------------------------------------------------------------
rules="$(call GET "/api/workspaces/$WS/admin/directory-scope" "" "$OT")"
[ "$(jq -r .ready <<<"$rules")" = "true" ] ||
	die "the workspace's directory is not set up (ready=false); apply migration 064 and restart policy first"
call PUT "/api/workspaces/$WS/admin/directory-scope/hidden/${DEPT[$HIDDEN_DEPT]}" "" "$OT" >/dev/null
call PUT "/api/workspaces/$WS/admin/directory-scope/limited/roles/$ROLE" "" "$OT" >/dev/null
say "rules: '$HIDDEN_DEPT' hidden, role '$LIMITED_ROLE' limited"

# --- summary (names only) ---------------------------------------------------------------------------------
echo
echo "✓ Demo organization '$ORG_NAME' is ready. Sign in at $BASE with the fixed one-time code as:"
printf '    %-28s %s\n' "$OWNER_EMAIL" "owner (sees everyone)"
for row in "${PEOPLE[@]}"; do
	where="$(field "$row" 4)"
	[ -n "$(field "$row" 5)" ] && where="${where:+$where · }vai trò $(field "$row" 5)"
	printf '    %-28s %s\n' "$(field "$row" 1)" "$(field "$row" 2)${where:+ · $where}"
done
