# Tester report: avatar upload (branch feat/avatar-upload, uncommitted), 2026-10-15

Report only. No source or test files changed.

## 1. Test results

| Suite | Result |
|---|---|
| auth `go test -count=1 -v ./...` | 363 PASS / 0 FAIL / 0 SKIP. All 6 pkgs ok |
| auth `-race` domain / store / rest | all ok (domain 22.6s, store 3.3s, rest 11.7s) |
| `make test s=auth` (strict, fails on skip) | exit 0, 0 skip lines. Run with env set |
| `make build-check` | ok, all services build |
| `make check-layering` | ok |
| `make check-ngac` | ok |
| frontend `npx vitest run` | 223/223 files, 2799/2799 tests pass (87s) |
| frontend `npm run lint` | ok, 0 errors |
| frontend `npm run build` | ok. Warnings only: "Route file ... does not export a Route" for `src/routes/*.test.*`, plus chunk-size. Not avatar-related |
| frontend `npm run typecheck` | exit 2, 7 errors. All in `src/generated/` (`google/protobuf/timestamp.ts`, `proto/messaging/ws.ts`). Zero outside generated |

Avatar tests confirmed running, none skipped (41 avatar-named PASS lines):
- store: TestSetAvatar_StoresEverySize…, _ReplacingRetiresTheOldToken, _Refusals*, _OneDecodeAtATime, _TenUploadsAnHour; TestClearAvatar_RemovesThePicture…, _RemovesItAndRefuses…; TestAvatar_*; TestGetAvatar_*. Tests named in the brief are present under other names (all PASS).
- domain: TestDecodedBytes_* (2), TestJPEGOrientation_*, TestNormalizeAvatar_* (6), TestSetAvatar_* (4), TestAvatar_*, TestClearAvatar_*.
- rest: TestUploadAvatar_* (4 incl. subtests), TestRemoveAvatar, TestServeAvatar_* (3), TestProfile_RefusesAnAvatarURL.
- frontend: Avatar.test.tsx 6 tests; SettingsScreen 'ảnh đại diện' block 7 tests; settings-model avatar tests 4 (`validateAvatarFile`, `avatarErrorMessage`).

Performance: auth suite ~6 s wall. Slowest avatar tests: TestNormalizeAvatar_PhonePhotoStaysInBudget 0.46s, TestSetAvatar_TenUploadsAnHour 0.51s, TestUploadAvatar_RateLimited 0.61s. Acceptable.

## 2. Coverage of avatar code (Go, per-function, from -coverprofile)

| File | Func coverage |
|---|---|
| domain/avatar.go | SetAvatar 90.5%, ClearAvatar 71.4%, Avatar 90%, takeAvatarUpload 90.9%, acquireAvatarGate 85.7%, normalizeAvatar 94.7%, orient 76.5%, decodedBytes 87%, jpegSamples 77.8%, exifOrientation 65% |
| store/avatar.go | SetAvatar 77.8%, ClearAvatar 77.8%, GetAvatar 90%, inTx 77.8% |
| rest/avatar.go | UploadAvatar 82.6%, RemoveAvatar 66.7%, ServeAvatar 94.4% |

No Go coverage run for frontend. Frontend coverage not measured (no `test:coverage` script run).

## 3. Untested branches

### Deny / refusal paths (project rule: each new decision path needs a deny test)

| Priority | Gap | Location | Risk |
|---|---|---|---|
| High | No test that a failed INSERT in `SetAvatar` rolls back the `users.avatar_url` UPDATE. Atomicity is untested | store/avatar.go:37-45 (tx), test TestSetAvatar_* | Profile points at a URL with no row behind it |
| High | Redis INCR error in rate limit → refused? Not tested | domain/avatar.go:149-151 | Upload path if Redis down; expected fail-closed 500, unverified |
| High | ServeAvatar store/DB failure → 500 with no cache header not tested | rest/avatar.go:93-95 | A transient DB error could be cached as 404 or leak text |
| Med | Mirror orientations 2, 4, 5, 7 never tested. Only 1, 3, 6, 8 in table | domain/avatar_test.go:119-122; avatar.go:252-263 | Phone photos with mirrored EXIF come out flipped silently |
| Med | Avatar() store error → not-found vs internal not tested | domain/avatar.go:132-134 | |
| Med | ClearAvatar with empty userID → ErrInvalidInput not tested | domain/avatar.go:108-110 | |
| Med | DELETE /api/me/avatar with account deleted (domain ErrNotFound → 404) not tested at REST level. Domain-level only | rest/avatar.go:70-72 | Mapping regression undetected |
| Med | Decode-gate ctx cancel branch untested (client disconnects while waiting) | domain/avatar.go:172-173 | Goroutine/gate leak not checked |
| Med | Budget fallbacks: unknown JPEG layout (assume full planes), progressive JPEG budget, 16-bit PNG and paletted budget arms | domain/avatar.go:301-303, 315-322, 334-353 | Budget under-estimate → OOM risk on auth (prod 192M) |
| Low | EXIF parser: MM (big-endian) order, out-of-range orientation value, truncated IFD branches | domain/avatar.go:398-427 | Falls back to 1 (safe), but untested |
| Low | UploadAvatar: fh.Open error, io.ReadAll error, post-read size check (53-55) | rest/avatar.go:44-55 | Defence in depth; post-read check unreachable by current test |
| Low | RemoveAvatar RequireClaims error branch (67-69) | rest/avatar.go | Middleware 401 test covers it in practice |
| Low | Store DB-error branches (Exec, Delete, Begin, Commit, GetAvatar query) | store/avatar.go:31-33, 55-57, 61-63, 80-82, 89-91, 96-98 | Error wrapping only |

Note: `decodedBytes` 301-303 is the "unknown layout" fallback. It is fail-safe (over-estimate), but still untested.

### Frontend

| Priority | Gap | Location |
|---|---|---|
| Med | Bỏ ảnh failure path (`catch → return`, no toast, error shown by shared handler). Only success tested | ProfileTab.tsx AvatarEdit `clear()`; SettingsScreen.test.tsx:386 |
| Med | Double-click / second pick while upload pending → disabled? Not asserted | ProfileTab.tsx `busy`, `choose` |
| Low | useUploadAvatar / useRemoveAvatar have no direct tests (only via SettingsScreen). Query invalidation on success not asserted directly | src/hooks/useProfile.ts:32-44 |
| Low | avatarErrorMessage: `avatar_missing` / `invalid_input` fall to generic text. No test (generic 500 path is tested) | settings-model.ts:74-85 |
| OK | Avatar.tsx: variant 64/192 choice, non-stored URL pass-through, onError → initials, retry on new src — all tested (Avatar.test.tsx 6 tests) |

## 4. Concerns

- Mirror-orientation gap (2, 4, 5, 7) is the most likely silent bug. Add table cases to TestNormalizeAvatar_DropsEXIFButKeepsTheWayUp.
- Atomicity test for SetAvatar is missing. Suggested: force a unique-violation on `user_avatars.token` and assert `users.avatar_url` unchanged.
- Deny tests needed (Redis error, ServeAvatar DB error → 500 without immutable cache header).
- Plan status: `plan.md` says phase 3 "pending" but the frontend code is in the diff. Sync before ship (project doc rule).
- `make test s=auth` was run with `TEST_DATABASE_URL` and `REDIS_ADDR` set. Not verified without env.
- Typecheck has 7 pre-existing errors in `src/generated/` (unchanged by this feature, per brief). Not blocking under the project's gate, but `npm run typecheck` exits 2.

## 5. Recommendations (prioritised)

1. Add store test: forced INSERT failure rolls back the UPDATE (atomicity).
2. Add domain test: Redis INCR error → `Upload` refuses with error, writes nothing.
3. Add rest test: ServeAvatar with a store error → 500, no `Cache-Control: immutable`.
4. Extend orientation table to 2, 4, 5, 7.
5. Add rest test: DELETE for a deleted account → 404.
6. Add frontend test: Bỏ ảnh failure shows error and keeps the picture.
7. Add domain test: truncated JPEG header → budget uses fallback and refuses an over-budget picture.
8. Update plan.md phase 3 status.

## 6. Build status

Green: build-check, check-layering, check-ngac, make test s=auth, vitest, lint, build. Typecheck: only pre-existing generated errors.

Status: DONE_WITH_CONCERNS

Summary: All gates green. Auth 363/363 pass, 0 skip. Avatar tests run and pass, race clean. Frontend 2799/2799, lint and build ok. Typecheck errors are all in `src/generated/`. Coverage gaps: deny and error branches (Redis error, ServeAvatar DB error, store rollback atomicity), mirror EXIF orientations 2/4/5/7, and frontend Bỏ ảnh failure path. Fix the high items before ship.

## Unresolved questions

- Does `fail()` map the Redis rate-limit error to 500? Not verified; mapError was not read.
- Is RemoveAvatar's RequireClaims branch reachable without middleware? Assumed not.
- Should `plan.md` phase 3 status be updated by tester or by the implementer?
