---
phase: 11
title: "Verify, rollout, spec close-out"
status: pending
priority: P1
effort: "4h"
dependencies: [7, 8, 9, 10]
---

# Phase 11: Verify, rollout, spec close-out

## Overview
Full-stack verification against the plan acceptance, specs brought in line with what shipped, then the approval-gated deploy.

## Requirements
- Gates: `make build-check`, `make check-layering`, `make check-ngac`, `make test` (with `TEST_DATABASE_URL` on :5433 and
  `REDIS_ADDR`; note the 2 known local-drift notification failures, do not mask others), `cd frontend && npm test &&
  npm run lint && npm run build`.
- `make db-migrate` on dev DB, observe new columns; start stack (`make dev`), confirm workspace backfill log and per-dept
  directory nodes via read-only SQL.
- Browser matrix (`/browse`) at 1280, 1024, 800, 390 with three accounts in a seeded org: owner, plain member, member of
  a limited role; one hidden department. Check every acceptance bullet in plan.md, incl. pickers that read departments
  (approval template department approver) for a limited user. `make dev-stop` afterwards.
- Specs final pass: `contacts-visibility`, `contacts-documents-settings-screens`, `admin-screens`, `tenant-ngac-init`,
  `workspace-admin-authorization` (directory rule endpoints + manage requirement). Update `docs/specs/README.md` index if it
  lists capabilities.
- `ak:test` then `ak:code-review` (security focus on phases 02–05).
- Rollout and rollback exactly as in plan.md; after deploy, smoke the three accounts on prod in a scratch org.

## Architecture
No new code beyond fixes found here.

## Related Code Files
- Modify: specs listed above; `docs/specs/README.md` (if indexed); plan status fields.

## Implementation Steps
1. Run gates; fix regressions at source (no test weakening).
2. Browser matrix; file defects back to the owning phase's files.
3. Specs pass; mark phases completed; update plan.md status.
4. Ship via `ak:ship` (one PR from `feat/contacts-lark`), approval-gated deploy, prod smoke.

## Success Criteria
- All plan acceptance bullets checked with evidence (screenshots + test names); prod smoke passes for three roles.

## Risk Assessment
- Prod backfill differs from dev (Med×Med): dry-run backfill against a prod DB snapshot locally before deploy, compare
  node counts.
- Rollback: plan.md rollback section.
