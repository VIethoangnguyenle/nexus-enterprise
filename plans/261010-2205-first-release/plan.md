---
title: "First release readiness"
description: "Before tagging the first release: full-system E2E, a notifications screen, the Docker-dev WebSocket fix, and signed service-to-service identity."
status: in-progress
priority: P1
branch: refactor/project-wide
created: 2026-10-10
---

# First release readiness

User decision (2026-10-10): all four items below ship before the first release.

| # | Item | Notes |
|---|---|---|
| A | Full-system E2E | On a local copy of the production stack running the exact deployed images (`d9817e4`) with the OTP test code enabled, so production gets no test data; plus a no-login smoke on production and a short manual checklist for the user (real Google/OTP sign-in). Includes role/department approval against the real policy service. |
| B | Notifications screen | Design first (DESIGN.md + mockup, user approval), then code. Backend + realtime events exist. |
| C | Docker-dev WebSocket | The dev Traefik `ws-strip` likely breaks `/api/ws`; fix and verify in the Docker dev stack. |
| D | Signed service identity | Replace the unsigned `x-caller-*` gRPC metadata with a short-lived token signed at the REST edge and verified by every gRPC server (plan.md decision 13 of the refactor). |

## Acceptance criteria
- [ ] A: every main flow passes on the production-image stack; defects found are fixed or listed.
- [ ] B: approved mockup; screen built; unread state and realtime updates work.
- [x] C: WebSocket connects and receives events in the Docker dev stack.
- [x] D: a gRPC call with forged or missing identity is refused on every server; tests + spec.
- [ ] Release tagged after a green deploy of all four.

## Status at session end (2026-10-10 ~23:00)
- Deployed `c56af89` to production (green, 14/14 healthy): invitation emails (E, new), signed
  service identity (D), Docker-dev WebSocket fix (C), notifications mockup + DESIGN.md entries.
  `INTERNAL_IDENTITY_SECRET` was added to `/opt/nexus/.env` via the bootstrap before deploying.
- **B (notifications screen): CODE COMPLETE, NOT REVIEWED, NOT COMMITTED** (working tree and the
  `wip/notifications` branch). Report: `reports/notifications-report.md`; all gates green on `ngac`
  and `ngac_ci` at session end. Includes migration 037, structured notifications, the approval
  recipient fix, invitation notifications + "Lời mời đang chờ (N)" in both switchers, and the
  `invitation_created` emit in workspace `invitations.go` (handled/dropped in messaging). Next:
  independent review → fixes → commit → deploy (with the E2E hotfixes below).
  Known follow-ups: the desktop panel is modal (useModalFocus) while the mockup shows a popover; a
  revoked/expired invitation leaves its notice unread.
- **A (full-system E2E): DONE — verdict NO-GO** (`reports/e2e-report.md`, 169/185 pass, 13 real
  defects). Fix FIRST next session, in this order:
  1. **CRITICAL, live in production:** the unread list returns every channel in the system to an
     outsider (cross-tenant leak) — `messaging/.../store/reactions_pins_receipts.go:249` uses
     `ngac_node_id` where the column is `ngac_node`. Hotfix + deny test + deploy.
  2. **Blocker:** workspaces created through the product get no approval schema → approvals 404 in
     every new tenant. Check existing prod tenants (read-only) and backfill.
  3. High: approval notifications not stored (fix is in the uncommitted notifications work —
     verify); deleting a department strands approval requests; polls/tasks/reactions/pins not live;
     approve-and-assign impossible in a one-owner tenant (product decision).
  4. Medium/low: empty rejection reason accepted; edge lacks HSTS/CSP/Referrer-Policy/
     Permissions-Policy. Open questions are at the end of the report.
  The `nexus-e2e` stack was torn down by the agent.
- Release tag: after B lands and deploys green, and A's findings are triaged.
