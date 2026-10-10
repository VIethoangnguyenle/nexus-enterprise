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
- [ ] C: WebSocket connects and receives events in the Docker dev stack.
- [ ] D: a gRPC call with forged or missing identity is refused on every server; tests + spec.
- [ ] Release tagged after a green deploy of all four.
