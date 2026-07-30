# Tech Stack

## Backend

- Go **1.25.0** (`backend/go.mod`, module `ngac-platform`). Single module covering all 8 services — no per-service `go.mod`.
- Toolchain on this machine is Go **1.26.1** at `/usr/local/go/bin`; `gopls`, `grpcurl`, `overmind`, `protoc-gen-go`, `protoc-gen-go-grpc` at `~/go/bin`. Both dirs are outside the system default PATH — `Procfile.dev` works around this by calling `/usr/local/go/bin/go` absolutely.
- HTTP: `labstack/echo/v4`. RPC: `grpc` + `protobuf`. DB: `jackc/pgx/v5` (no ORM). Auth: `golang-jwt/jwt/v5`.

## Frontend

- React **19**, Vite **8**, TypeScript, Vitest **4** + Testing Library, ESLint **10**.
- TanStack Router (file-based, `routeTree.gen.ts` is generated — never hand-edit) + TanStack Query + TanStack Virtual.
- Zustand for client state, Tailwind **4** (via `@tailwindcss/vite`, not PostCSS), Tiptap for the editor, `@protobuf-ts` for wire types.

## Infra

Postgres, Redis, Redpanda (Kafka API), MinIO (S3 API), Traefik. Redis is partitioned per service by URL (`REDIS_URL_POLICY`, `REDIS_URL_AUTH`, `REDIS_URL_MESSAGING`) rather than shared with key prefixes.
