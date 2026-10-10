# Frontend

React 19 single-page app for the NGAC platform: Vite, TanStack Router and Query, Zustand, Tailwind 4.
TanStack Query owns server state; Zustand owns client state (the WebSocket store is the one deliberate exception).

Design source of truth is [`../DESIGN.md`](../DESIGN.md) ("Tín hiệu") and the approved mockups in
[`../design/mockups/`](../design/mockups/). Design there first; this code renders that design.
Repo-wide rules live in [`../CLAUDE.md`](../CLAUDE.md).

## Scripts

Run from this directory.

| Command | What it does |
| --- | --- |
| `npm run dev` | Vite dev server. From the repo root, `make dev` starts the infra, all Go services and this. |
| `npm test` | Vitest (jsdom), one run. `npm run test:watch` to watch. |
| `npm run lint` | ESLint, including the design-rule restrictions (no raw palette classes, `transition-all`, etc.). |
| `npm run typecheck` | Full `tsc --noEmit`. |
| `npm run typecheck:diff` | Compares error counts per file with `typecheck-baseline.txt`; fails if any file gets worse. Never grow the baseline; shrink it when you fix errors. |
| `npm run build` | Production build. |
| `npm run proto:gen` | Regenerates `src/generated/` from `backend/proto/messaging/ws.proto`. Needs `protoc`. |

## Dev proxy

There is no gateway. In native dev, [`vite.config.js`](vite.config.js) forwards `/api/*` to the Go services on their own
ports, and `/api/ws` to the WebSocket server on `:8081`. It is not a flat table: `/api/workspaces` re-dispatches nested
paths by regex (`/drive`, `/documents`, `/channels`, `/contacts`, `/asset*`) to different services, and `/api/messages`
must stay declared before `/api/me`. A new REST route has to be added there, or it will 404 in native dev while working
in Docker (where Traefik routes).

## Generated code

Never hand-edit:

- `src/routeTree.gen.ts`: written by the TanStack Router plugin on `dev`/`build`.
- `src/generated/`: protobuf wire types, from `npm run proto:gen`. After a shared `.proto` change also run `make proto`
  at the repo root.

## Layout

- `src/routes/`: file-based routes; screens live in `src/components/<area>/`.
- `src/api/`: typed REST clients. `src/hooks/`: query and mutation hooks; `src/hooks/keys/` is the single source of query keys.
- `src/stores/`: Zustand stores (`auth`, `ui`, `drive`, `websocket`).
- `src/lib/`: pure logic (models, search-param builders, formatting, realtime planning). `src/test/`: shared fixtures.
