---
title: "Execution plan — chat phase 05 wave-1 verification (two people, real stack)"
owner: Claude (planning, review) · executor: agy/Gemini
status: ready
---

# Execution plan — verify wave 1 end to end

The dev stack is running (`make dev`: Vite http://localhost:5173, all services). Do NOT start/stop it. This task
writes ONE Playwright script + screenshots + a report; it must not change app code.

## Accounts
Two people in the same workspace. Use the existing smoke account `smoke1791570521@example.org` (A) and create
B by signing in with a new address `verify-b-<timestamp>@example.org` via OTP (fixed code 999999). Then make B a
member of A's workspace AND of one group: find how the app invites/adds members (Quản trị / channel members
dialog) and do it through the UI as A — never by SQL. OTP requests are rate limited (5 per 15 min per address):
sign in ONCE per person and reuse each browser context; never reload pages (the token is in memory).

## Script
Create `frontend/e2e/wave1-two-people.mjs` (check first whether the repo already has an e2e folder/runner — if it
does, put it there in its style; do not add a new framework). Plain Playwright from `frontend/node_modules`, two
browser contexts, viewport 390x844 for A and 1440x900 for B, light scheme.
1. A opens the group, posts a root message M; B replies in M's thread → within 1 s A sees M's summary row with
   "1 mới" (timestamp the frame; record the measured delay).
2. A unfollows another thread T (or uses one A does not follow); B replies in T → A's T summary shows the reply
   count change but NO "mới".
3. A opens M's thread → its "mới" goes to 0 and the navigator unread for the thread disappears.
4. B posts 30 messages while A is on another conversation; A opens the group → the "Tin mới" divider is above the
   first unread and the jump button ("Tin mới ↓" or as designed) scrolls to it.
Screenshots after each step for both people → `plans/261010-1559-chat-inline-threads/reports/phase-05-screens/`.
Record console errors and failed network requests per context.

## Gates (report output, don't fix app code)
- `cd frontend && npx vitest run`, `npm run lint`, `npm run typecheck:diff`, `npm run build`.
- `bash scripts/check-docs-drift.sh` from the repo root.
- Backend: for each service dir under backend/services run `go test ./...` with
  `PATH=/usr/local/go/bin:$PATH TEST_DATABASE_URL=postgres://ngac:ngac_secret@localhost:5433/ngac_chat_mine?sslmode=disable REDIS_ADDR=localhost:6379`
  and report pass/fail/skip counts per service.

## Rules
- PROGRESS LOG after EVERY step: `plans/reports/agy-progress-chat-05.log` (`<time> | step | result`); read it first
  when resuming.
- Never edit app code; if a criterion fails, record exactly what you saw (screenshot + console) as a finding.
- No commits, no git index. Leave the test data (B's account, messages) in place and list it in the report.
- Report: `plans/reports/agy-<date>-chat-05-verification.md` — each of the 4 criteria PASS/FAIL with evidence,
  measured delays, gate outputs, findings.
