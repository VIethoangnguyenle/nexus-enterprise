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
- **B (notifications screen): in progress, UNCOMMITTED on disk.** An agent was mid-way when the
  session ended: messaging notifications restructured (migration `037_notifications_structured.sql`,
  already applied to the local `ngac` and `ngac_ci` DBs), proto `messaging.proto`/`ws.proto`,
  `pkg/realtime/event.go` (`KindInvitationCreated`), workspace `invitation_announce_test.go`, and
  the frontend panel/sheet. Messaging did not compile at last check. Next session: review what is
  on disk against `reports/notifications-design-report.md` and finish, including the
  invitation-surfacing scope (invitations as notifications + "Lời mời đang chờ (N)" in the
  workspace switcher), then re-add the `s.announce(ctx, realtime.KindInvitationCreated, …)` line in
  `workspace/internal/domain/invitations.go` (kept out of `c56af89`) together with the hub/consumer
  handling that drops it from workspace fan-out.
- **A (full-system E2E): in progress.** Output under `reports/e2e/`; no final report yet. A local
  compose project `nexus-e2e` may still be running — check `docker ps` and tear it down
  (`docker compose -p nexus-e2e down -v`).
- Release tag: after B lands and deploys green, and A's findings are triaged.
