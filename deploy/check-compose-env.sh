#!/usr/bin/env bash
# Renders the production compose file (dummy env) and asserts what a deploy depends on:
#   - every service sets the environment variables its cmd/main.go reads (derived from the source,
#     so a variable added to a service later fails here until the compose file sets it);
#   - every *_ADDR points at a service that exists;
#   - no published port, no host networking; only the nginx edge and MinIO are on `traefik`;
#   - the OTP test code is off and auth trusts proxy headers from the nginx edge only.
# Run from the repository root. Needs docker compose and jq.
set -euo pipefail
cd "$(dirname "$0")/.."

# Variables a service may leave unset: they have a safe default or a feature switch.
OPTIONAL=" METRICS_PORT STRICT_OPERATIONS AUTH_PUBLIC_RATE_LIMIT AUTH_DEV_OTP APP_BASE_URL
 GOOGLE_CLIENT_ID GOOGLE_CLIENT_SECRET GOOGLE_REDIRECT_URL SMTP_HOST SMTP_PORT SMTP_USERNAME
 SMTP_PASSWORD SMTP_FROM SMTP_TLS MINIO_USE_SSL AUTH_FIXED_OTP_CODE workspace:POLICY_READ_SERVICE_ADDR "
OPTIONAL=" $(tr -s '[:space:]' ' ' <<<"$OPTIONAL") "

declare -A MAIN=(
  [policy]=backend/services/policy/cmd/main.go
  [policy-read]=backend/services/policy/cmd/policy-read/main.go
  [auth]=backend/services/auth/cmd/main.go
  [workspace]=backend/services/workspace/cmd/main.go
  [document]=backend/services/document/cmd/main.go
  [messaging]=backend/services/messaging/cmd/main.go
  [asset]=backend/services/asset/cmd/main.go
  [drive]=backend/services/drive/cmd/main.go
  [approval]=backend/services/approval/cmd/main.go
)

cfg=$(NEXUS_HOME=/opt/nexus docker compose -f deploy/docker-compose.prod.yml \
  --env-file deploy/env.example --profile tools config --format json)

fail=0
bad() { printf 'FAIL: %s\n' "$*" >&2; fail=1; }

# --- structure ----------------------------------------------------------------------------
[[ $(jq '[.services[] | select(has("ports"))] | length' <<<"$cfg") -eq 0 ]] || bad "a service publishes ports"
[[ $(jq '[.services[] | select(has("network_mode"))] | length' <<<"$cfg") -eq 0 ]] || bad "a service sets network_mode"
on_traefik=$(jq -r '[.services | to_entries[] | select(.value.networks | has("traefik")) | .key] | sort | join(" ")' <<<"$cfg")
[[ $on_traefik == "nexus-frontend nexus-minio" ]] || bad "services on the traefik network: '$on_traefik' (want: nexus-frontend nexus-minio)"
[[ $(jq -r '.services.auth.environment.AUTH_FIXED_OTP_CODE | if . == null then "unset" else . end' <<<"$cfg") == "" ]] ||
  bad "auth: AUTH_FIXED_OTP_CODE must be set and empty"
edge_ip=$(jq -r '.services["nexus-frontend"].networks.nexus.ipv4_address' <<<"$cfg")
[[ $(jq -r '.services.auth.environment.AUTH_TRUSTED_PROXIES' <<<"$cfg") == "$edge_ip/32" ]] ||
  bad "auth: AUTH_TRUSTED_PROXIES must be exactly the edge address $edge_ip/32"

# --- environment each service needs ---------------------------------------------------------
names=$(jq -r '.services | keys[]' <<<"$cfg")
for svc in "${!MAIN[@]}"; do
  main=${MAIN[$svc]}
  [[ -f $main ]] || { bad "$svc: $main not found"; continue; }
  jq -e --arg s "$svc" '.services[$s]' <<<"$cfg" >/dev/null || { bad "$svc: not in the compose file"; continue; }
  needed=$(grep -ohE '(bootstrap\.Env|bootstrap\.EnvAlias|os\.Getenv|os\.LookupEnv)\("[A-Z][A-Z0-9_]*"' "$main" | grep -oE '"[A-Z0-9_]+"' | tr -d '"' || true)
  grep -q 'bootstrap\.PolicyAddr()' "$main" && needed+=$'\nPOLICY_SERVICE_ADDR'
  grep -q 'RequireJWTSecret' "$main" && needed+=$'\nJWT_SECRET'
  grep -q 'realtime\.' "$main" && needed+=$'\nKAFKA_BROKERS'
  for var in $(sort -u <<<"$needed"); do
    [[ $OPTIONAL == *" $var "* || $OPTIONAL == *" $svc:$var "* ]] && continue
    val=$(jq -r --arg s "$svc" --arg v "$var" '.services[$s].environment[$v] // ""' <<<"$cfg")
    [[ -n $val ]] || bad "$svc: $var is read by $main but empty or missing in the compose file"
  done
  # Every address it dials must name a service in this file.
  for kv in $(jq -r --arg s "$svc" '.services[$s].environment | to_entries[] | select(.key | test("_ADDR$|^MINIO_ENDPOINT$")) | "\(.key)=\(.value)"' <<<"$cfg"); do
    host=${kv#*=}; host=${host%%:*}
    grep -qx "$host" <<<"$names" || bad "$svc: ${kv%%=*} points at '$host', which is not a compose service"
  done
done

[[ $fail -eq 0 ]] && echo "compose environment check: ok"
exit $fail
