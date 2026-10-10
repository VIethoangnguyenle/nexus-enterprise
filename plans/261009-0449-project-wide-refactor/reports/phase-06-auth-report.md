# Phase 04b-auth + 06 auth/onboarding redesign: report

Date 2026-10-10. Branch `refactor/project-wide`. Nothing committed or staged (the index still holds only the user's own `.claude/skills` deletions; see Concerns 9 for one slip I reverted).

## What the person now gets

Login (Google + code) → verify (six boxes) → profile (new people only) → workspace selection with pending invitations → create workspace. Mockup: `design/mockups/auth.html`. No dev OTP hint anywhere, no SSO stub, no "Join an Organization", no typed URL or slug, no "Free Tier", no welcome page or timed redirect (`welcome.tsx` deleted).

- **Workspace selection** lists invitations above the person's workspaces (who invited, to what, role, when it lapses) with Tham gia / Từ chối. Accept joins and opens that workspace; the session is first re-scoped with `POST /api/auth/switch-tenant` (services such as approval read the tenant from the token). Declining says so by toast. 404/409/400/403 from accept each get their own sentence. Loading, empty, error, and a partial failure (offers failed, workspaces fine) all exist.
- **Unverified email**: no offers, one calm line with the address, and only real ways to verify: "Xác minh bằng Google" (address as `login_hint`) and, only when `otp_proves_email` is true, a code sent to the address with the same code entry. When neither exists it says so. Phone-only accounts are told offers go to an email. The fixed test code is never offered as verification.
- **Profile**: display name (+ optional title), live initials preview on the person's own hue. Only filled fields are sent. No photo upload: the server has no image store a profile could point at.
- **Create workspace**: one field, name. Body is exactly `{"name"}`.
- **Code field** (`primitives/OtpInput`, rewritten): one real input (`one-time-code`, numeric) drawn as six boxes, so paste and SMS autofill work. States idle, checking, wrong, expired, locked, success, disabled. Paste keeps the first six digits of whatever was copied; arrows/Home/End move the highlighted box; sixth digit submits once. Resend countdown 60 s in tabular figures; a refused resend locks the button for the time the server named.
- **Errors inline** (all auth mutations `silentError`): 429 says how long ("thử lại sau 12 phút"), 503 says sign-in is paused, offline says check the network, wrong code says tries left. No server text and no code ever reach the screen.
- **Logout**: "Đổi tài khoản" goes through `logoutSession` (server revoke, then store logout + `queryClient.clear`), tested by asserting the `/api/auth/logout` POST.

## Backend (`backend/services/auth`)

Migration `data/migrations/032_users_profile_completed.sql` (031 is the other agent's). Backed up first (`pg_dump -Fc` of `ngac` and `ngac_ci`, scratchpad `backup-*-auth-screens.dump`), applied with `make db-migrate` and by psql to `ngac_ci`. It adds `users.profile_completed_at` and marks existing accounts done only in the run that adds the column (idempotent under re-apply).

New behaviour:
- `/api/me` and OTP verify report `email_verified` and `needs_profile` (so Google and code accounts take the same road to the profile step).
- `PATCH /api/me/profile` is a real partial update (absent = unchanged, `""` clears), validated server-side (80/120 runes, no control or invisible characters, avatar only `http(s)` with host), all-or-nothing, caller's own record only, empty body is 400 and does not end the step. Before, any PATCH blanked every field not sent.
- `GET/POST /api/me/workspaces`: list with own role and headcount; create from `{name}` with full provisioning (the old `POST /api/workspaces` never wrote `tenant_users` or `#general`), 5 per hour per person (429 + `Retry-After`).
- Error bodies are `{message, code}` (+ `attempts_left`, `retry_after_seconds`); `Retry-After` header on 429. Unexpected failures answer `internal error`; the text goes to the log (before: `err.Error()` with hosts and downstream errors went to the caller).
- `/api/auth/providers` adds `otp_proves_email`.

Authorization and enumeration fixes found on the way (each with a deny test, red first):
- `SwitchTenant` accepted a `disabled` or `invited` membership and issued a token for the tenant; `GetMe` presented it as current. Now active only.
- `GET /api/workspaces/:id/contacts` listed any workspace to any signed-in person: now 403 unless an active member.
- `GET /api/users` and `GET /api/users/lookup` (whole user table, username oracle; no consumer) removed.
- Password sign-in timing: an unknown address, a Google/code-only account (no password) and a wrong password now all cost one bcrypt comparison and give one error; signup hashes before looking the address up. Measured live: 51 ms vs 51 ms.
- OTP: wrong-code answer carries `attempts_left` (session property, same for known and unknown addresses; tested). OTP request answers identically for known/unknown. Rate counters that lost their expiry get one again.
- `docker-compose.yml`: Traefik never routed `/api/me*` to auth (frontend router caught it). Added `PathRegexp(^/api/me(/|$$))` (regexp, because plain `PathPrefix(/api/me)` would also catch `/api/messages`). Not exercised against Traefik.

Specs: `docs/specs/tenant-auth-flow` (account state, partial profile, my workspaces, error bodies, directory, switch-tenant, OTP bodies, enumeration), new `docs/specs/sign-in-screens`, `workspace-admin-authorization` status line, `README` index.

## Frontend

Routes are thin; screens live in `components/auth/` (`LoginScreen`, `CodeEntry`, `ProfileScreen`, `WorkspaceSelectScreen`, `InvitationRow`, `VerifyEmailNote`, `CreateWorkspaceScreen`, `AuthShell`, `Notice`, `Steps`, `GoogleLogo`). Words and decisions in `lib/auth-flow.ts`; guards in `lib/auth-guards.ts` (run in `beforeLoad`, so they cannot race a sign-in: the old layout redirected any signed-in person to `/documents`, which made workspace selection and onboarding unreachable). `hooks/useAuth.ts`, `hooks/useInvitations.ts`, `api/auth.ts` rewritten; dead password `useLogin/useRegister` removed.

Found only by running the real stack in a browser: `apiFetch` treats every 401 as "session ended" (refresh, log out, drop the body), and the auth service answers a wrong code with 401. Every wrong code would have logged out the page and lost `attempts_left`. Added `publicFetch` to `api/client.ts` (no token, no refresh, no logout, body kept) and moved providers / request code / verify code onto it. Tested at client and API level.

Also: the store now persists only `{id, username, ngac_node_id}` after sign-in (it used to keep email, phone and union id in localStorage). `TextField` gained `large` (44px) and `hint`. `eslint.config.js`: the Google-logo hex exemption moved from `login.tsx` to `components/auth/GoogleLogo.tsx`. `vite.config.js`: comment only, `/api/me` prefix already covers `/api/me/workspaces`. `routeTree.gen.ts` regenerated by `npm run build` (not hand-edited).

## Decisions (mockup silent or user rules differ)

1. **Wrong code** uses error colour plus a fade (`m.route`: 160 ms opacity, 120 ms under reduced motion), no shake, no transform. It clears the boxes, keeps focus, says tries left. A source test forbids the word "shake" in the group.
2. Create workspace is **name only** (user rule): mockup's invite-by-email chips and "Để sau" are dropped; the hint says invite later from Quản trị.
3. Profile keeps the mockup's optional **title** (sent as `title`); the preview avatar is 64 (the scale step the other agent added; mockup says 56).
4. **Step bar** only on profile (2/3) and on create when the person has no workspace yet; on verify it would say "1 of 3" to returning people.
5. People with **one workspace and an unverified email stop on the picker** instead of going straight in, otherwise the explanation is never seen. Everyone else with one workspace and nothing to answer goes straight in, decided once on first load (declining a last offer does not throw you into a workspace).
6. Landing after entry is `/channels` (what `/` already redirects to), not `/documents`. A `?ws=` in the login link, when the person belongs to it, is opened directly.
7. Accepting does not animate "row moves to the list then opens": the page is left immediately.
8. Invitation row: both Tham gia and Từ chối, under the sentence (mockup had one button beside it; with two there was no width). Selection box 420 px (`wide`), forms 380.
9. Dropped: footer Điều khoản / Chính sách links (no such pages), "dùng số điện thoại" link on verify (Đổi email covers it). Back arrow only below 1024.
10. Row subtitle is "N thành viên · Chủ sở hữu/Thành viên" (tenant role; the data has no organisation name or custom role name).
11. Brand pane keeps the mockup's three sample changes (names are decoration, `aria-hidden`).
12. Buttons 44 px (`h-11`) and fields 44 px per mockup note and DESIGN §9.

## Evidence

- Backend: `make build-check` all 8 services; `make check-ngac` clean; `go vet ./...` clean on auth; `make test s=auth` exit 0 on **ngac** and **ngac_ci**, 229 passes each, zero skips (`TEST_DATABASE_URL`, `TEST_REDIS_ADDR`, `REDIS_ADDR` set). New: store (against Postgres), domain (profile validation, workspaces, OTP state, SwitchTenant deny, timing), REST end-to-end through Echo + JWT middleware (`rest/account_test.go`). Mutation-checked: timing fix removed → 3 tests fail; SwitchTenant status check absent → 3 red before the fix.
- Frontend: `npm run lint` 0; `typecheck:diff` no file above baseline; `npm test` 74 files / 969 tests; `npm run build` 0. Test files added: `auth-flow`, `auth-guards`, `OtpInput`, `CodeEntry`, `LoginScreen`, `ProfileScreen`, `WorkspaceSelectScreen`, `CreateWorkspaceScreen`, `AuthShell`, source guards, route wiring, `publicFetch`, request-body shapes. The fixture (`test/auth-fixtures.ts`) rejects any body field the Go handlers do not read. Mutation-checked: client-side invitation hiding, auto-entry rule, switch-tenant skip, paste digit filter, code clearing all turn tests red.
- Production build scanned: no "test mode", no `OTP code is`; the only `999999` strings are unrelated numeric constants in router and motion code. A source test also fails on `999999`, "test mode" or `import.meta.env.DEV/PROD` in the group.
- **Live smoke** against freshly built policy, auth and workspace services (47 checks, all passed): wrong code 401 with 4 tries left; new account unverified and owing a profile; five refused profile bodies change nothing; save, trim, partial update keeps name; own workspaces list; create from a name ignores slug/domain/owner, no domain claimed, creator is `owner/active` in `tenant_users`; five bad names refused; switch-tenant works for a member, refused for a stranger and for a suspended member; 6th creation 429 with `Retry-After`; invite → unverified account sees none → (verified by SQL stand-in) sees the offer by name with no address in it → accept → role member, 2 members, can switch; outsider 403 on contacts; `/api/users*` gone; wrong password vs unknown address same answer and time. Rows deleted afterwards; my three processes stopped (the Vite on :5173 belongs to the other agent and was reused, not stopped).
- **Screenshots**: `plans/261009-0449-project-wide-refactor/reports/auth-screens/` (71 files): login, verify, profile, selection-with-invitations, create at desktop 1440 / tablet 820 / mobile 390 in light and dark; plus desktop states in both themes: Google error, invalid field, 429, 503, sign-in paused, empty code, wrong code, expired, locked, resend refused (429), list, unverified with Google, unverified with a delivered code (typing), unverified with nothing available, empty, error, loading skeleton, accept refused, profile error, create 429; and wrong code under reduced motion. Playwright from the gstack install against the running Vite; the API is answered in-page by the same fixture the tests use (not a live backend).

## Concerns

1. **No real OTP sender exists in dev** and Google needs credentials, so the "verify by code" path is proven by unit tests, the fixture screens and a SQL stand-in in the live smoke, not by a real delivered code.
2. **Password signup/signin/login/register have no rate limit**, and signup for a taken address is still 409 (it cannot create the account). No screen uses these endpoints; recommend retiring them or adding a limiter.
3. **The sidebar workspace switcher only changes `?ws=`** and never calls switch-tenant, so after a sidebar switch the token still names the old tenant (approval reads it). My picker re-scopes correctly; the sidebar is off limits this round.
4. `useCreateWorkspace` / `workspaceApi.create` (`POST /api/workspaces`) are now unused and create workspaces without `tenant_users` or `#general`. Left alone (the other agent edits those files); delete or make the workspace service provision fully.
5. Personal workspaces are named `<handle>'s Workspace` by the server (English, from the address); `workspaceDisplayName` only masks `user_N's workspace`, so the picker shows the handle form.
6. The Docker Traefik change was not run against Traefik.
7. Plan and phase files are not ticked.
8. `routeTree.gen.ts` regenerated by the build also contains the other agent's routes.
9. **Index slip:** I ran `git rm --cached` on `welcome.tsx` once by mistake, noticed at once and restored the entry with `git reset -- <that file>`; the staged set is again only the user's 158 deletions, and `welcome.tsx` is deleted in the working tree only.

Unresolved questions: whether the sidebar should call switch-tenant (3); whether password endpoints stay (2).

Status: DONE_WITH_CONCERNS
Summary: The auth screen group is rebuilt to the approved mockup and wired to real server behaviour: new-person profile step, invitation accept/decline on workspace selection, an honest unverified-email state, name-only workspace creation, a code field with paste, keyboard and all its states, and inline 429/503/offline handling. The auth service gained account-state flags, a validated partial profile update, own-workspace list/create, safe error bodies, and fixes for suspended-member token issuance, an open contacts directory, a user-table oracle and a password timing oracle. All gates pass on `ngac` and `ngac_ci` with zero skips, a 47-check live smoke passed, and screenshots are saved.
Concerns: no real OTP sender in dev (verify-by-code proven by tests and a stand-in); password endpoints still unlimited with a 409 on signup; the sidebar workspace switcher never re-scopes the token; the unused `POST /api/workspaces` client path is left behind; Traefik label not run.


---

# Review fixes (2026-10-10)

User decisions applied: (a) password signup/signin/login/register removed entirely; (b) creating a workspace needs a verified email (403 `email_unverified`; the UI shows the verify path, not the form).

## What changed

- **Password routes gone** (REST, domain methods, gRPC `Register/Login/Signup/Signin/ListUsers`, tests, `/api/users*`, the Vite `/api/users` proxy). Without a session these answer 401 `session_required` (group middleware), with one 404. `test_app.sh` signs in by OTP (`NGAC_TEST_OTP`, default 999999). `TestNoPasswordOrListingRPCsExist` guards the gRPC side.
- **Workspace creation is strict and verified.** Unverified or no-email caller: 403 `email_unverified`, nothing created, budget not spent. Any failure after the workspace exists is undone through a new workspace RPC `DeleteWorkspace` and answers 500. Live evidence: with messaging down, `#general` failed, the log said "removed a workspace whose provisioning failed" and no workspace row or tenant node remained.
- **`DeleteWorkspace` (additive, approved by the coordinator; compensation-only, no REST route).** Files: `backend/proto/workspace/workspace.proto`, `workspace/internal/store/teardown.go`, `domain/workspace_delete.go`, `grpc/delete_workspace.go` (+ tests); server.go/service.go untouched (optional-interface assertions). Owner-only and sole-member-only from the grpcauth caller (deny tests: non-owner, owner with other members, no caller, another workspace); one transaction removes rows, drive, channels, tenant approval schema, tenant_users; MinIO bucket best-effort; graph nodes deleted newest first through policy-write (EPP invalidation), stopping at the first failure; idempotent incl. half-created and retry. Tests prove no rows or nodes remain.
- **Error envelope everywhere.** `{message, code}` for handler, middleware (`pkg/httputil` jwt/claims/tenant: `session_required`, `session_invalid`, `tenant_required`), refresh/logout, unknown routes; unknown failures are `internal` with text only in the log. A sweep over the route table asserts it.
- **Public-route limiter** per address (60/min default, `AUTH_PUBLIC_RATE_LIMIT`) on otp-request/verify, refresh, logout, google-start; 429 `rate_limited` + `Retry-After`; `AUTH_TRUSTED_PROXIES` controls where `X-Forwarded-For` is believed (spoofed header test); `docker-compose.yml` sets Docker bridge ranges.
- **Email proof without sign-in**: `POST /api/me/email/verify` (code; 503 `verification_unavailable` with the fixed code or log sender, 409 `already_verified`) and `POST /api/me/email/verify/google` (mismatch refuses, signs nobody in, returns to `/workspace-select?verified=1|verify_error=`). First proof ends other sessions, spares the caller's.
- **OTP refresh-tenant bug fixed**: the OTP handler now passes `TenantID` to `RefreshIdentity`, so the refresh token no longer loses its tenant after a reload.
- **Profile**: `department` (`department_not_editable`) and `avatar_url` (`avatar_not_supported`) refused by name. `GET /me?workspace=` returns title/location/avatar/`current_tenant.department` (admin-assigned, per workspace). Settings → Hồ sơ reads this instead of searching the directory.
- **Contacts**: members-only, keyset paging (`cursor` / `next_cursor`, `limit` default 50 max 200), true `total`, `department`/`location`/`search` filters, admin-assigned department, `display_name` empty (not the handle) before the profile is saved. Wire covered by `useContacts.test.tsx` and the store/REST tests.
- **Suspended members** can no longer switch tenant or read `/me`.
- **Picker** built from `GET /me/workspaces` (now with company `domain`); `ProfileScreen` keeps `ws`; Google verify return handled by workspace-select (toast / calm error).
- **Docker/Traefik**: auth router now also claims `/api/me` and `/api/workspaces/:id/contacts` (priority 110 over workspace).
- **Migration** `032_users_profile_completed.sql` applied to `ngac` and `ngac_ci` (backups in the scratchpad); no new migration this round.
- **Specs updated**: `docs/specs/tenant-auth-flow/spec.md`, `docs/specs/sign-in-screens/spec.md`.

## Verification

- `make build-check`, `make check-ngac`: pass.
- `make test s=<svc>` for auth, workspace, policy, document, messaging, asset, drive, approval on **ngac and ngac_ci**: all exit 0, no FAIL, no skip; `go test ./pkg/...` passes. No `t.Skip` in the new tests.
- Frontend: `npm run lint` clean; `typecheck:diff` no file above baseline (fixed three new errors in tests/fixtures); `npm test` 76 files / 1030 tests pass; `npm run build` OK.
- **Live smoke** (policy, auth, workspace, messaging built from source; all stopped afterwards; throwaway `@smoke-auth.test` rows and tenant graphs deleted by SQL): password routes gone; unknown route envelope; unverified create 403 `email_unverified`, verified 201; compensation on a real downstream failure (above); department / avatar refusals; `/me?workspace=`; contacts 6 people over 3 pages, distinct, true total, bad cursor 400, outsider 403; fixed-code mode returns 503 `verification_unavailable`, verified account 409; limiter 429 with `Retry-After`. 
- **Screenshots** retaken without the React Query devtools button, 77 files in `reports/auth-screens/`, adding unverified create-workspace (Google, and delivered-code typing) and the Google-mismatch banner on workspace selection. My own Vite on :5173 was started for this and stopped.

## Skipped, with reasons

- **Invitation headcount**: lives in the workspace service's shared invitation view/store; not trivial and outside my files.
- **Terms/Privacy footer**: no such pages exist.
- Org name shown on the picker is the claimed workspace domain, only when present.

## Concerns

1. No real OTP sender in dev, so verify-by-code is proven by unit tests and fixtures, not a delivered code; the live smoke covers the refusal in fixed-code mode.
2. The limiter fails open on Redis errors (never blocks a person when the counter is down).
3. The Traefik rule change was not run against Traefik; `routing_test.go` checks it against docker-compose and the Vite config only.
4. `useCreateWorkspace` / `POST /api/workspaces` removal belongs to the other agent.
5. Unknown `/api` paths without a session answer 401 `session_required`, not 404, because the group middleware runs first.
6. `contacts.username` is still sent because chat maps messages by it; it is never shown as a name.
7. `docker-compose.yml` also carries the other agent's unrelated `POLICY_SERVICE_ADDR` hunk. Git index untouched.

Status: DONE_WITH_CONCERNS
Summary: Every review item is fixed with deny tests: password endpoints removed, workspace creation requires a verified email and is undone on failure through the new compensation-only `DeleteWorkspace` RPC, errors carry codes everywhere, public routes are rate limited per address, email can be proved without signing in, contacts are paged with a true total and admin department, and Settings reads its own record. Go gates on ngac and ngac_ci, frontend gates, live smoke and retaken screenshots all pass; specs are updated.
Concerns: no real OTP sender in dev; limiter fails open on Redis errors; Traefik rule not run against Traefik; invitation headcount and Terms/Privacy footer skipped; `POST /api/workspaces` client cleanup belongs to the other agent.
