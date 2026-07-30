# CLAUDE.md — NGAC Platform

## 1. What this repo is

A multi-tenant workspace platform (documents, drive, chat, assets, approvals) whose entire
authorization layer is NGAC — NIST Next Generation Access Control — rather than roles.
Eight independent Go services behind no gateway, plus a React 19 SPA.
Every access decision is a graph reachability question answered by the policy service.

## 2. Commands

Run from the repo root. `make help` lists everything.

```bash
make dev            # infra + all 8 Go services + frontend; PIDs in .dev-pids, logs in .dev-logs/
make dev-stop       # stop everything started by `make dev`
make run            # same stack in the foreground via overmind (Procfile.dev)
make dev-infra      # postgres/redis/redpanda/minio only, then applies schema

make build-check    # compile every service's ./cmd/ — the fast "does it still build" gate
make test           # Go tests, all services; exits non-zero on failure
make test s=policy  # one service, verbose
make db-migrate     # re-apply data/init.sql + data/migrations/ to the running DB
make proto          # regenerate Go from backend/proto/

cd frontend && npm test     # vitest
cd frontend && npm run lint # eslint
cd frontend && npm run build
cd frontend && npm run proto:gen   # regenerate TS wire types — separate from `make proto`
```

**Go is not on the default PATH.** `export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"` — without
it every Go command fails with a misleading "Go is not installed".

## 3. Tech stack and conventions

Go 1.25, Echo, pgx (no ORM), gRPC. React 19, Vite, TanStack Router/Query, Zustand, Tailwind 4.
Postgres, Redis, Redpanda, MinIO, Traefik.

Only what is not derivable in ten seconds:

- **Nine Go modules, not one.** `backend/go.mod` holds shared code (`ngac`, `pkg/httputil`,
  `proto`, `testutil`); each service has its own `go.mod` with `replace ngac-platform => ../..`.
  There is no `go.work`. **`go test ./...` from `backend/` silently skips every service** and
  reports success having tested nothing — always run Go commands from inside a service directory.
- **Schema lives in three places**: `data/init.sql` (base), `data/migrations/` (numbered chain —
  `union_id`, `open_id`, `tenant_users` come from `005_multi_tenant_auth.sql`), and
  `backend/services/policy/migrations/` (policy-local). Approval tables live in a per-tenant
  schema, never in `public`.
- **No gateway.** Each service owns its REST surface on its own port. In Docker, Traefik routes;
  in native dev, the Vite proxy does. `policy` has no REST surface at all — gRPC only.
- **`frontend/vite.config.js` is not a flat proxy table.** `/api/workspaces` re-dispatches nested
  paths by regex to five different services (`/drive` → 8185, `/documents` → 8182, `/channels` →
  8183, `/contacts` → 8180, `/asset*` → 8184). `/api/messages` must stay declared before
  `/api/me` or prefix matching swallows it. WebSocket is `:8081`, not messaging's REST `:8183`.
- **Frontend state split**: TanStack Query owns server state, Zustand owns client state.
  `websocket.store.ts` is the deliberate exception. `routeTree.gen.ts` and `src/generated/` are
  generated — never hand-edit.
- **Design source of truth is Stitch** (`.stitch/DESIGN.md`, `.stitch/WORKFLOW.md`). UI is
  designed there first; code renders that design. Do not design in code.

### NGAC model — the parts that are counter-intuitive

- **The graph does not contain objects.** `LoadGraph()` loads only `U`, `UA`, `OA`, `PC`
  (`pip_store.go`). Files, messages, and approval requests are **not** nodes — they live in
  Postgres with a foreign key to a parent OA. Access is therefore checked **on the OA**, never on
  the object itself. This is the single most important architectural decision in the system.
- **Intersection principle.** A permission holds only when the user side (`U → UA`) and the
  resource side (`O → OA`) both reach the **same PC**. Different PC means DENY even when an
  association exists.
- **Eight fixed operations**, defined in `backend/ngac/ngac_ops.go`: `read, write, upload,
  approve, share, manage, invite, create_channel`. They stay generic verbs — context comes from
  the OA the association targets, and is never encoded into an operation name.
- **Decision order** (`pdp_decision_engine.go`): resolve graph (shard → global) → BFS →
  CTE SQL fallback for OAs not in memory → prohibitions as deny-overrides applied **only** to an
  ALLOW. Default is DENY.
- **Runtime checks never touch the DB.** They run against the in-memory graph; the DB is read at
  startup and when the graph changes. Any graph mutation must go through the EPP invalidation
  path or every service keeps serving stale decisions.
- **`ngac_nodes`, `ngac_assignments`, `ngac_associations` are the source of truth.** Business
  tables hold only a FK to `ngac_nodes.id`. `channel_members` is a denormalized cache — the real
  membership is an assignment.

## 4. Required workflow

**superpowers is the sole workflow axis.** Do not add a second one.

- Non-trivial work: `superpowers:brainstorming` → `superpowers:writing-plans` (plans go in
  `docs/superpowers/plans/YYYY-MM-DD-<feature>.md`) → `superpowers:executing-plans` or
  `superpowers:subagent-driven-development`.
- All code: `superpowers:test-driven-development` — failing test first.
- All bugs: `superpowers:systematic-debugging` before proposing a fix.
- Before claiming done: `superpowers:verification-before-completion`.
- Any plan that changes system behaviour must name the capability in `docs/specs/` it adds or
  modifies, and update that spec when the task lands.

**Boundary between the two skill sets:** superpowers = quy trình (khi nào làm gì, theo trình tự
nào). `.claude/skills/` = kiến thức domain (làm thế nào cho đúng trong lĩnh vực đó). Skill domain
KHÔNG được định nghĩa quy trình; cần quy trình thì gọi superpowers.

## 5. Enforcement Rules

These define what counts as an unfinished change. They are not advice.

**Policy model changes.** Changing node types, assignments, associations, prohibitions, or the
decision order MUST update the matching `docs/specs/<capability>/spec.md` and add a test vector
to `backend/services/policy/internal/ngac/` in the same commit. *A change to the policy model
without both the spec update and the test vector is incomplete.*

**New decision paths.** Any new branch in the PDP MUST be covered by tests for the **deny** case,
not only the allow case — the system's default is DENY and a branch that only proves ALLOW proves
nothing about the boundary. *A new decision path tested only on its allow branch is incomplete.*

**NGAC identifiers.** Every operation string and node name MUST come from `backend/ngac`.
Never `fmt.Sprintf` a node-name pattern inline; the helper functions exist so a rename becomes a
compile error instead of a silent authorization change. *A change that introduces an NGAC string
outside `backend/ngac` is incomplete.*

**Graph mutations.** Any code that writes to `ngac_nodes`, `ngac_assignments`, or
`ngac_associations` MUST route the invalidation through the EPP path, because runtime decisions
read the in-memory graph and will otherwise stay stale indefinitely. *A graph write without EPP
invalidation is incomplete.*

**Cross-cutting proto changes.** Editing a `.proto` that both sides consume MUST run `make proto`
**and** `cd frontend && npm run proto:gen`. Regenerating one side leaves the wire contract skewed
with no compile error on the other. *A shared proto change with only one side regenerated is
incomplete.*

**New REST routes.** Adding a route MUST add it to the dev proxy table in
`frontend/vite.config.js` — and, if it sits under `/api/workspaces/:id/`, to the regex
re-dispatch block, not merely the flat table. Otherwise it 404s in native dev while working in
Docker. *A new REST route absent from the dev proxy is incomplete.*

**Identifiers in the UI.** No UUID, database ID, foreign key, or internal code may reach the
screen; entities render as a name or title, users as display name (plus avatar and role where
available), and audit entries always name the actor alongside the action and timestamp. ID entry
by hand is never an input method — use a picker, dropdown, or search. *A UI change that surfaces
a raw identifier, or that asks the user to type one, is incomplete.*

**Schema changes.** Editing `data/init.sql` or adding to `data/migrations/` MUST be applied with
`make db-migrate` and verified against a running database before the change is called done.
*A schema change that has not been applied and observed is incomplete.*
