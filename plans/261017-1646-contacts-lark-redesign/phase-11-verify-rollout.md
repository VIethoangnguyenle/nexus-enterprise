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
Full-stack verification against the plan acceptance, specs brought in line with what shipped, then release A (code) and
release B (064) through the approval-gated deploy.

## Requirements
- Gates (env in plan.md "Test env"; `TEST_DATABASE_URL` = `ngac_ci`, never the dev `ngac` DB): `make build-check`,
  `make check-layering`, `make check-ngac`, `make check-docs`, `make test` (note the known local-drift notification
  failures, do not mask others), `make test-realpolicy`, `cd frontend && npm test && npm run lint && npm run build`.
- Dev DB: `make db-migrate` twice (second run of 064 changes 0 rows), `make dev-restart` of policy + policy-read,
  `make policy-invalidate`; read-only SQL confirms every workspace's `Directory_<id>`, every department's triple, one
  `ngac_prohibitions` row per workspace, 0 `tenant_users.department_id` drift.
- Release-A tolerance on dev: before applying 064 to a scratch copy, the same build serves contacts (everyone visible) and
  rule endpoints 503; after 064 the rules work.
- Browser matrix (`/browse`) at 1280, 1024, 800, 390 on the org seeded by `scripts/seed-contacts-demo.sh` with four accounts:
  owner, plain member, member of a limited role, member of the hidden department. Check every acceptance bullet in plan.md,
  incl. pickers that read departments (approval template department approver) and calendar guest pickers for a limited user,
  and a hidden person's name in chat/approval/docs/calendar. `make dev-stop` afterwards.
- Specs final pass: `contacts-visibility`, `contacts-documents-settings-screens`, `admin-screens`, `tenant-ngac-init`,
  `workspace-admin-authorization` (directory rule endpoints + manage requirement), `calendar` (guest rows, visible-only
  pickers). Update `docs/specs/README.md` index (it lists capabilities).
- `ak:test` then `ak:code-review` (security focus on phases 02–05 and 064).

## Architecture
No new code beyond fixes found here.

## Related Code Files
- Modify: specs listed above; `docs/specs/README.md`; plan status fields.

## Implementation Steps
1. Run gates; fix regressions at source (no test weakening).
2. Browser matrix; file defects back to the owning phase's files.
3. Specs pass; mark phases completed; update plan.md status.
4. Release A: `ak:ship` one PR from `feat/contacts-lark` without `064_*`; approval-gated deploy; prod smoke (owner, member).
5. Prod dry-run of 064 in the migrate container (`docker compose --profile tools run --rm migrate psql …`, runbook in
   `docs/deployment.md`); paste counts in the PR; set the thresholds in the file.
6. Release B: PR carrying only `064_directory_visibility.sql`; deploy (recreates policy + policy-read, runs
   `policy invalidate-cache --all-workspaces`); prod smoke of the three roles in a scratch org.

## Success Criteria
- All plan acceptance bullets checked with evidence (screenshots + test names); prod smoke passes for three roles after B.

## Risk Assessment
- Prod 064 differs from dev (Med×Med): migrate-container dry-run before approval; guard thresholds abort an unexpected size.
- Policy services serving pre-064 graph after B (Low×High): deploy recreates both and invalidates (runbook); smoke checks a
  rule end-to-end.
- Rollback: plan.md rollback section.
