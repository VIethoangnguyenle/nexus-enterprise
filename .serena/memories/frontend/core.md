# Frontend — Core

React 19 SPA in `frontend/`, Vite + TanStack Router/Query, Zustand, Tailwind 4. Layout, state-split, and test-placement rules are in `mem:conventions`.

## Two proxy topologies

`frontend/vite.config.js` switches on `VITE_DEV_MODE`:

- `VITE_DEV_MODE=true` (native dev, what `make run`/`make dev` set) — proxies each `/api/*` prefix to a **distinct localhost port** per service.
- unset (Docker) — proxies everything to Traefik on `:80`.

A new REST route therefore has to be registered in the dev proxy table or it 404s locally while working fine in Docker. Two traps in that table:

- **Order matters.** `/api/messages` is declared before `/api/me` because Vite matches by prefix and `/api/me` would otherwise swallow it.
- **`/api/workspaces` is not one service.** It targets workspace (`:8181`) by default but a `configure` hook re-dispatches nested paths by regex: `/drive` → `:8185`, `/documents` → `:8182`, `/channels` → `:8183`, `/contacts` → auth `:8180`, `/asset*` → `:8184`. Adding a new `/api/workspaces/:id/<thing>` owned by another service means adding a branch there, not just a table entry.

## WebSocket

Realtime runs over `/api/ws` → `ws://localhost:8081`, a **different port from messaging's REST (`:8183`)** even though the same service owns both. `src/stores/websocket.store.ts` is the largest store in the codebase and owns connection lifecycle plus buffered message state — it is the deliberate exception to "Zustand holds no server state".

## Generated code

- `src/routeTree.gen.ts` — TanStack Router plugin output.
- `src/generated/` — protobuf TS from `npm run proto:gen`, which is a **separate command from `make proto`**; a shared `.proto` edit requires both.
