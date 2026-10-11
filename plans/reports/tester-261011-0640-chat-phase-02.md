# QA: chat phase 02 (thread data model and API)

Branch claude/inspiring-cray-32y4n1, uncommitted. No code modified.

## 1. make test s=messaging (strict, 0 skips)
- Exit 0. Packages: events, rest, store, domain, grpc all ok.
- 173 `--- PASS`, 0 FAIL, 0 SKIP.

## 2. backend shared (ngac, pkg, testutil)
- Exit 0. 9 ok, 2 no test files.

## 3. make test (all services)
- Exit 2 on first run. Failures are environmental, not code:
  - policy: 12 skips, `TEST_REDIS_ADDR not set`.
  - auth: 54 skips/fails, `REDIS_ADDR must be set`.
- Cause: `make test` does not export Redis env. CI sets `localhost:6379` (`.github/workflows/ci.yml:40-41`).
- Redis tests `FlushDB` on the server they connect to. Did NOT point them at the shared 6379 (other project's Redis).
- Re-ran policy and auth against a throwaway `redis:7-alpine` container on 127.0.0.1:6399 (stopped after). Both strict-green, 0 skips:
  - policy: events, grpc, ngac ok.
  - auth: cmd, domain, rest, grpc, googleauth, store ok.
- Other services green under make test: workspace, document, messaging, asset, drive, approval (all ok, 0 skips).
- Changed-area coverage: messaging is the only changed service. Its suite is green (section 1).

## 4. Checks
| Target | Result |
|---|---|
| make check-layering | PASS (self-test ok, all checks ✓) |
| make check-ngac | PASS |
| make check-docs | PASS (CLAUDE.md consistent) |
| make build-check | PASS (all 8 services build) |

## 5. Frontend: npm test -- --run
- RED: 1 failed / 1144 passed (1145 tests, 86 files).
- Failing: `src/components/assets/AssetsScreen.test.tsx` "shows the four figures ... who did it". Expects text `Phạm Hải Yến · 08:52` in "Chờ bạn duyệt" region; not found.
- Deterministic (same on isolated rerun). Not timezone-caused (TZ=Asia/Ho_Chi_Minh also fails).
- Not touched by this phase: no frontend files in `git status`. Likely pre-existing, but not proven (did not stash to check).

## Summary
Messaging (phase 02 scope) strict-green, 0 skips. Shared, build, layering, ngac, docs green. Full Go suite green once Redis is provided; without Redis, policy/auth fail by env. Frontend has 1 unrelated asset test failure.

Status: DONE_WITH_CONCERNS
Summary: Messaging suite 173/173 pass with 0 skips; all Go services green when Redis is provided (throwaway container); build, layering, ngac, docs checks pass. Frontend vitest has 1 deterministic failure in the assets screen, outside this phase.
Concerns/Blockers:
- `make test` (all services) fails without Redis env. Policy and auth need `TEST_REDIS_ADDR`/`REDIS_ADDR`; the Makefile does not set them. Suggest exporting them in the make target or in dev docs.
- Shared Redis on 6379 is used by something else; the Redis tests FlushDB, so keep them off it.

Unresolved questions:
- Is the AssetsScreen failure pre-existing on `main` or the branch? Needs a stash/checkout run to confirm.
- Should `make test` wire Redis (e.g. docker-compose redis on a dedicated test port)?
