# Conventions

## Go

- Every service is `backend/services/<name>/` with `cmd/` (entrypoint only) and `internal/` (everything else). `internal/` is subdivided by role — `grpc/`, `rest/`, `store/`, `domain/` — and the split is enforced by convention, not tooling: transport code does not reach into the DB, `store/` does not build protobuf messages.
- Cross-service shared code lives in `backend/pkg/` (currently only `httputil`: JWT parsing, claims, tenant extraction, error shapes, schema helpers). Anything reusable belongs there, not copied per service.
- `backend/ngac` is imported by every service that touches permissions. Build NGAC identifiers with its helper functions (`PCName`, `OwnersUAName`, `ChannelContentOAName`, `TenantMemberUAName`, …); do not `fmt.Sprintf` the patterns inline.
- pgx directly, no ORM. Services that emit events go through `internal/events/`.
- Test helpers in `backend/testutil/`. Tests run with `-count=1` — do not rely on caching.

## Frontend

- One API module per backend domain in `src/api/` mirroring service names (`auth.ts`, `drive.ts`, `messaging.ts`, …), all built on `src/api/client.ts`. New endpoints extend the matching module rather than calling fetch from components.
- State is deliberately split: **TanStack Query owns server state**, **Zustand owns client/session state** (`src/stores/*.store.ts`). Do not cache server responses in Zustand — `websocket.store.ts` is the one large exception because it manages a live connection plus its buffered state.
- Routes are file-based under `src/routes/`, grouped by layout with leading-underscore segments (`_auth`, `_workspace`, `_drive`). `routeTree.gen.ts` is generated.
- Tailwind 4 utility classes; global styles in `src/index.css`.
- Tests colocate with source as `*.test.ts` (e.g. `src/api/messaging.test.ts`), not in a separate tree; `src/test/` holds setup only.

## Protobuf

`.proto` files are grouped per domain under `backend/proto/<domain>/`, and larger domains split by read/write surface (`policy.proto`, `policy_read.proto`, `policy_write.proto`) so the read path can be served by a separate replica.
