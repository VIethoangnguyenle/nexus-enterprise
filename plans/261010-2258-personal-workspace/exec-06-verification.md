---
title: "Execution plan — personal workspace phase 06 verification (real stack)"
owner: Claude (review) · executor: agy/Gemini
status: ready
---

# Verify personal workspaces end to end — NO app code changes

Read `plans/261010-2258-personal-workspace/plan.md` (acceptance criteria) and `phase-06-verification.md`.
The dev stack is running (`make dev`: Vite http://localhost:5173, services up). Do NOT start/stop/restart it.
OTP sign-in: fixed code 999999; OTP requests are rate limited (5 per 15 min per address) — sign in ONCE per person and
reuse the browser context; never reload pages (the token is in memory).

## Script
Create `frontend/e2e/personal-workspace.mjs` in the style of `frontend/e2e/wave1-two-people.mjs` (plain Playwright
from frontend/node_modules; read that file first). Viewports 1440x900 and 390x844, light scheme.
1. New account `pws-<timestamp>@example.org` signs in → lands in a personal workspace named after the person;
   sidebar (desktop) and tab bar / Thêm sheet (phone) show NO Tài sản / Phê duyệt / Quản trị entries. Screenshot.
2. Deep links as that person: go to `/assets` and `/approval` → redirected, the area never renders. Screenshot.
3. Direct API calls with that person's session (use `page.evaluate(fetch …)` with the same auth the app uses — find
   how `frontend/src/api/client.ts` sends it): an asset list call and an approval create call for the personal
   workspace → both refused (record status codes and bodies; no 2xx).
4. Cài đặt → Workspace: the convert card; convert to organization; Phê duyệt and Tài sản appear in the sidebar and
   phone nav WITHOUT reload. Screenshot. Repeat the step-3 calls → no longer refused for the reason "personal".
5. "Tạo workspace" from the switcher: the type choice defaults to Cá nhân; create one; Back returns to where you came
   from. Screenshot.
6. Admin → Roles → permission areas of a personal workspace (use the one from step 5): no Tài sản area.
Record console errors and failed network requests per step.

## Gates (report output, don't fix app code)
From repo root with `export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"`: `make test` (if it needs TEST_DATABASE_URL use
`postgres://ngac:ngac_secret@localhost:5433/ngac_chat_mine?sslmode=disable` and `REDIS_ADDR=localhost:6379`),
`make check-layering`, `make check-ngac`. Frontend gates are run by Claude — skip them.

## Rules
- PROGRESS LOG after EVERY step: `plans/reports/agy-progress-pws-06.log` (`<time> | step | result`); read first when
  resuming.
- Never edit app code (frontend/src/**, backend/**). If a criterion fails, record exactly what you saw (screenshot +
  console + network) as a finding.
- Screenshots → `plans/261010-2258-personal-workspace/reports/phase-06-screens/`.
- No commits, no git index. Leave the test accounts in place and list them.
- Report: `plans/reports/agy-261011-pws-06-verification.md` — each acceptance criterion PASS/FAIL with evidence.
