#!/usr/bin/env bash
# One-time (and safely re-runnable) server preparation for the Nexus deploy. Run as root on the
# VPS, after the repo's deploy/ files were copied there or via `ssh root@host 'bash -s' < ...`:
#
#   sudo bash server-bootstrap.sh --pubkey-file /root/nexus-deploy.pub
#   sudo bash server-bootstrap.sh --pubkey 'ssh-ed25519 AAAA... nexus-deploy'
#   sudo bash server-bootstrap.sh                 # no key change, only directories and .env
#
# What it does, and only this:
#   - creates /opt/nexus, /opt/nexus/backups and /opt/nexus/releases
#   - creates /opt/nexus/.env (chmod 600) with generated secrets; an existing file is never
#     overwritten, only keys that are missing get appended, so re-running cannot rotate a secret
#     that a live database or MinIO volume already depends on
#   - checks the external docker network `traefik` exists (does not create it)
#   - appends the deploy public key to /root/.ssh/authorized_keys, prefixed with `restrict`,
#     only when that key is not already there
# It never prints a secret, and never touches /opt/traefik, /opt/odysseus, /opt/openclaw or ufw.
set -euo pipefail

APP_DIR=${APP_DIR:-/opt/nexus}
AUTH_KEYS=${AUTH_KEYS:-/root/.ssh/authorized_keys}
PUBLIC_URL=${PUBLIC_URL:-https://nexus.zaneng.xyz}
pubkey=""

say() { printf '  %s\n' "$*"; }
step() { printf '\n== %s\n' "$*"; }
die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

while [[ $# -gt 0 ]]; do
  case $1 in
    --pubkey) pubkey=${2:?--pubkey needs a value}; shift 2 ;;
    --pubkey-file) pubkey=$(<"${2:?--pubkey-file needs a path}"); shift 2 ;;
    -h | --help) sed -n '2,19p' "$0"; exit 0 ;;
    *) die "unknown argument: $1" ;;
  esac
done

[[ $EUID -eq 0 ]] || die "run as root"
command -v openssl >/dev/null || die "openssl is required"

step "Prerequisites"
docker compose version >/dev/null 2>&1 || die "docker compose plugin not found"
say "docker compose: ok"
for tool in rsync flock curl; do
  command -v "$tool" >/dev/null || die "$tool is required by the deploy (apt install ${tool/flock/util-linux})"
  say "$tool: ok"
done
if docker network inspect traefik >/dev/null 2>&1; then
  say "external network 'traefik': present"
else
  die "docker network 'traefik' not found. It belongs to /opt/traefik; start that stack first. Not creating it here."
fi

step "Directories"
for d in "$APP_DIR" "$APP_DIR/backups" "$APP_DIR/releases"; do
  if [[ -d $d ]]; then say "exists: $d"; else mkdir -p "$d"; say "created: $d"; fi
done
chmod 750 "$APP_DIR"
chmod 700 "$APP_DIR/backups"
chmod 750 "$APP_DIR/releases"

step "Environment file $APP_DIR/.env"
env_file=$APP_DIR/.env
if [[ -f $env_file ]]; then
  say "exists: keeping current values, adding only keys that are missing"
else
  ( umask 077; : >"$env_file" )
  say "created (empty)"
fi
chmod 600 "$env_file"

added=()
# ensure_key NAME VALUE [comment-line]: append NAME=VALUE when NAME is absent. Value is never echoed.
ensure_key() {
  local name=$1 value=$2 comment=${3:-}
  if grep -qE "^${name}=" "$env_file"; then return 0; fi
  # A hand-edited file may lack the final newline; appending to it would glue two keys together.
  if [[ -s $env_file && -n $(tail -c1 "$env_file") ]]; then printf '\n' >>"$env_file"; fi
  { [[ -z $comment ]] || printf '%s\n' "$comment"; printf '%s=%s\n' "$name" "$value"; } >>"$env_file"
  added+=("$name")
}
rand_hex() { openssl rand -hex "$1"; }

ensure_key POSTGRES_DB ngac
ensure_key POSTGRES_USER ngac
ensure_key POSTGRES_PASSWORD "$(rand_hex 24)" "# Generated. Changing it after the first start does not change the database user's password."
ensure_key REDIS_PASSWORD "$(rand_hex 24)"
ensure_key JWT_SECRET "$(rand_hex 48)" "# Signs every session token. Rotating it signs everyone out."
ensure_key MINIO_ROOT_USER "nexus$(rand_hex 6)"
ensure_key MINIO_ROOT_PASSWORD "$(rand_hex 24)"
ensure_key APP_BASE_URL "$PUBLIC_URL"
ensure_key GOOGLE_REDIRECT_URL "$PUBLIC_URL/api/auth/google/callback" \
  "# Must be listed verbatim under the OAuth client's Authorized redirect URIs."
ensure_key GOOGLE_CLIENT_ID "" \
  "# >>> FILL IN: Google OAuth client id. Empty = Google sign-in off. Production has no other sign-in:
# the OTP test code is disabled and the services ship no email/SMS sender."
ensure_key GOOGLE_CLIENT_SECRET "" "# >>> FILL IN: Google OAuth client secret (a literal \$ must be written \$\$)."

if [[ ${#added[@]} -gt 0 ]]; then say "added keys: ${added[*]}"; else say "nothing to add; file already complete"; fi
todo=()
for k in GOOGLE_CLIENT_ID GOOGLE_CLIENT_SECRET; do
  grep -qE "^${k}=.+" "$env_file" || todo+=("$k")
done

step "Deploy key"
if [[ -z $pubkey ]]; then
  say "no --pubkey given: authorized_keys untouched"
else
  pubkey=$(printf '%s' "$pubkey" | tr -d '\r' | head -n1)
  [[ $pubkey =~ ^(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp[0-9]+)\ [A-Za-z0-9+/=]+(\ .*)?$ ]] ||
    die "that does not look like a single-line OpenSSH public key"
  blob=$(printf '%s' "$pubkey" | awk '{print $2}')
  install -d -m 700 "$(dirname "$AUTH_KEYS")"
  [[ -f $AUTH_KEYS ]] || ( umask 077; : >"$AUTH_KEYS" )
  if grep -qF "$blob" "$AUTH_KEYS"; then
    say "key already present in $AUTH_KEYS: unchanged"
  else
    # `restrict` turns off pty, agent/X11/port forwarding. The workflow only runs rsync and plain
    # commands over ssh, so nothing it needs is lost. The key is still root-equivalent: it can run
    # any command, which is why it must be a dedicated key used by this workflow alone.
    printf 'restrict %s\n' "$pubkey" >>"$AUTH_KEYS"
    chmod 600 "$AUTH_KEYS"
    say "appended deploy key with 'restrict' to $AUTH_KEYS"
  fi
fi

step "Summary"
say "app dir:   $APP_DIR (backups in $APP_DIR/backups)"
say "env file:  $env_file (mode $(stat -c %a "$env_file"), owner $(stat -c %U "$env_file"))"
if [[ ${#todo[@]} -gt 0 ]]; then
  say "STILL TO FILL IN in $env_file: ${todo[*]}"
fi
say "untouched: /opt/traefik /opt/odysseus /opt/openclaw ufw"
