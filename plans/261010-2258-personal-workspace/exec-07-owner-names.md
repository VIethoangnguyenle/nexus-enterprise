---
title: "Execution plan — workspace owners as a list, not a joined string"
owner: Claude (decisions) · executor: agy/Gemini
status: ready
---

# owner_names array

Today `GET /api/workspaces/:id/details` returns `owner_name` as one string joined with ", "
(`backend/services/workspace/internal/domain/workspace_details.go`, `rest/workspace_details_handler.go`), and the
frontend `components/settings/WorkspaceTab.tsx` splits it on ", " — a display name containing ", " splits wrongly.

## Decisions (fixed)
- Add `owner_names: string[]` (display names, ordered as today) to the response; keep `owner_name` (joined, unchanged)
  for one release so an older client still works. Domain gets `OwnerNames []string`.
- Frontend reads `owner_names` when present (fallback to the old split only when absent); one PersonChip per name.
- Update `docs/specs/` for this route (find the spec that documents workspace details — grep `owner_name`).

## Steps (tests first)
1. Domain + REST tests: two owners, one owner whose name contains ", " → `owner_names` has the exact names; a
   non-member still gets 403 (deny test unchanged but asserted).
2. Implement backend. Gates in `backend/services/workspace` with
   `export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" TEST_DATABASE_URL=postgres://ngac:ngac_secret@localhost:5433/ngac_chat_mine?sslmode=disable REDIS_ADDR=localhost:6379`:
   `go test ./...`, `go vet ./...`; repo root `make check-layering`.
3. Frontend test (WorkspaceTab: a name with ", " renders as one chip) → implement in `api/workspaces.ts` type +
   `WorkspaceTab.tsx`. Gates: touched tests, full `cd frontend && npx vitest run` (known: AssetsScreen), `npx eslint`
   touched files, `npm run typecheck:diff`.

## Rules
Touch ONLY the files named above plus their tests and the spec. Do NOT restart services. PROGRESS LOG after EVERY
step: `plans/reports/agy-progress-owner-names.log`. No commits, no git index. Report:
`plans/reports/agy-261011-owner-names.md`.
