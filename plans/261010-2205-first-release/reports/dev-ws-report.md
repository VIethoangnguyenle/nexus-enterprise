# Dev WebSocket path fix

## Bug (confirmed)
- Hub (`backend/services/messaging/cmd/main.go:123`) serves only `/api/ws` on :8081.
- `docker-compose.yml` ws router had a `ws-strip` stripprefix `/api` middleware, so the hub received `/ws` -> 404.
- Vite native dev (`devProxy['/api/ws']` -> ws://localhost:8081, `ws: true`): already correct, path unchanged.
- Vite docker mode (`dockerProxy`): `'/api'` lacked `ws: true` (no upgrade through the Vite proxy) and `'/ws'` entry was dead (frontend connects to `/api/ws`, `websocket.store.ts:168`).
- Prod nginx (`deploy/nginx`) passes `/api/ws` unchanged: correct, untouched.

## Reproduction method
Full dev stack NOT started: port 80 is held by an unrelated `docker-nginx-1` (Dify). Instead a throwaway rig on a private
docker network: a stub hub (python, 404 on anything except `/api/ws`, echoes first frame) + Traefik on 127.0.0.1:18080 using
the exact ws router labels from compose, constrained to a test label so no other containers are touched.
- With `ws-strip`: `HTTP/1.1 404 Not Found`.
- Without: `101 Switching Protocols`, auth frame `{"auth":"tok"}` sent and echoed back.
Stub, not the real hub (real hub needs policy/auth/redis/jwt); route-level proof only.
All test containers + network removed. Pulled images traefik:v3.2 and v3.6 remain locally.

## Changes
- `docker-compose.yml`: removed the `ws-strip` middleware definition and the router `middlewares` label (messaging ws labels only), added a one-line comment.
- `frontend/vite.config.js`: `dockerProxy['/api']` gets `ws: true`; dead `/ws` entry removed.
- CLAUDE.md §3: WS facts (`:8081`, not REST `:8183`) are correct; not edited.
- Not committed, index untouched. `docker compose --profile app config` parses with no `ws-strip` left.

## Side finding
`traefik:v3.2` (pinned in compose) fails against this host's Docker daemon: "client version 1.24 is too old, min 1.44";
the docker provider never loads, so the dev Traefik would 404 everything here. `traefik:v3.6` works. Not changed (out of scope).

Status: DONE_WITH_CONCERNS
Summary: Removed the /api strip from the dev ws Traefik router so the hub gets /api/ws; Vite docker mode now upgrades ws. Verified 404 -> 101 + auth frame echo through Traefik on a stub rig.
Concerns: Not verified against the real hub or the real compose stack (port 80 busy). Compose pins traefik:v3.2, which cannot talk to this machine's Docker API (needs >=v3.6 or an older daemon API) so the dev stack's Traefik likely will not route anything here until bumped.
