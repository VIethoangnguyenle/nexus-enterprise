# Nexus first-release E2E report

Date: 2026-10-10. Tested build: images of commit `d9817e4` (pulled from GHCR, revision labels checked).
Repo HEAD during run: `61616dc` (docs only, committed during run). Working tree has uncommitted notifications work (Plan B, 36 modified tracked files + untracked) that is NOT in the tested images.

## Verdict

**NO-GO for tagging.** Four defects break release criteria on the tested build; one is cross-tenant data disclosure. A fifth (approval module unusable in any new tenant) is a blocker. Re-run this suite on the candidate images after fixes.

## Results overview

| Area | Result |
|---|---|
| Recorded checks (saved JSON, latest run per script) | 185: 169 PASS, 16 FAIL |
| The 16 FAIL: real defects | 13 = F1 (1), F4 (1), F5 (8 checks), F7 (1), F10 (2) |
| The 16 FAIL: by spec / by design / superseded | 3 = hand-over race (by spec, F11), channel search on home (by design), pin owner-side selector (superseded by API run) |
| Defects not in the FAIL list | F2 (approval schema) and F3 (notifications, prod only): worked around or seen in logs, not in the saved checks |
| Production no-login smoke | PASS (see below), with header gaps |
| Stack teardown | Done. No nexus-e2e containers/volumes/networks left. Dev stack untouched. |

Coverage was driven through the real UI (Playwright via gstack package, same as earlier reports) for flows 1, 2, 4, 5, 8, 9, 10, 11, 12, 13 and smoke for 3, 6, 7. API used where UI path was impractical or to test deny/race cases. Some checks are NOT covered (see Coverage gaps).

## Findings (ordered by severity)

### F1 [CRITICAL] Cross-tenant disclosure: unread list returns every channel in the system

- Steps: sign in as outsider X (tenant B only). `GET /api/channels/unread`.
- Expected: only channels of tenant B (X's own memberships).
- Actual: HTTP 200, 393 entries = every channel row in the DB, including tenant A `#general`. Entries carry `channel_id` (count fields were not present in the sampled entries). Result: `plans/.../reports/e2e/xtenant-api.json` (test "outsider unread list has no tenant-A channel" FAIL).
- Root cause: `backend/services/messaging/internal/store/reactions_pins_receipts.go:249`: `WHERE cm.ngac_node_id = (SELECT ngac_node_id FROM users WHERE id = $1)`. `users` has no `ngac_node_id` column (it is `ngac_node`), so Postgres resolves the name to the outer `cm.ngac_node_id`, and the predicate is always true. Verified in DB: buggy form returns 395 rows; with `ngac_node` it returns 2 (X's own).
- Also affects every user's unread badges, not only outsiders.
- Present in HEAD and in working tree (unchanged).
- Fix: use `ngac_node`; add a DB-backed test asserting another tenant's channels are excluded.

### F2 [BLOCKER] Approval unusable in any workspace created through the product

- Steps: owner creates workspace via `POST /api/me/workspaces` (the path the UI uses). Then create template or request.
- Expected: approval works in the new tenant.
- Actual: 404 `tenant schema not provisioned` on every approval route. `tenant_schemas` table stays empty after workspace creation.
- Root cause: nothing provisions the approval schema on workspace creation. `backend/services/auth/internal/domain/tenants.go:68-114` (`provisionTenant`) has no approval step; `backend/services/approval/internal/rest/handler.go:100-104` answers 404; the only provisioning route `/api/admin/tenants/:id/provision` (`approval/internal/rest/handler.go:61`) is deliberately not routed publicly (`deploy/nginx/default.conf:144`). Spec `docs/specs/tenant-auth-flow/spec.md:250` lists the workspace steps without the approval schema.
- Workaround used in this run: called the provision route from inside the `nexus-e2e_nexus` network with the tenant owner's token. Approval tests ran only after that.
- Impact on prod: every new workspace has no approvals until someone provisions it by hand. Whether existing prod tenants have schemas is unknown (see questions).

### F3 [HIGH] Approval notifications are never stored (deployed build)

- Steps: any approval step/completion with a requester.
- Expected: requester gets a notification row.
- Actual: 65 `notification not recorded` ERROR lines in messaging logs (`plans/.../reports/e2e/logs/messaging.log`), FK violation on `notifications_user_id_fkey`. `notifications` table has 0 `approval_*` rows after the run; only asset types exist.
- Root cause: `backend/services/messaging/internal/events/consumer.go:286` sets `Recipient: evt.CreatedBy`, which is an NGAC node id, while `store/notifications.go:66-72` writes it to `user_id` (FK to `users.id`).
- Working tree (uncommitted) changes recipient to `evt.RequesterID` at `consumer.go:242`. Fixed in WIP, NOT verified here.

### F4 [HIGH] Deleting a department strands approval requests

- Steps: template with step `department = X`; owner deletes department X (204); create a request on that template.
- Expected: refusal, or template repointed / delete blocked.
- Actual: request 201 with assignment `grant_source=department:<deleted UA>`. No one can act (member 403; owner `can_act=false`). Request stays pending forever.
- Root cause: `backend/services/approval/internal/domain/steps.go:118-130` (`buildAssignments`) accepts any UA id without checking the department is live; `backend/services/workspace/internal/domain/department.go:300-352` (`DeleteDepartment`) neither blocks nor checks templates.
- Fix: refuse request creation when a step resolves to zero approvers; block delete or repoint templates.

### F5 [HIGH] Realtime gaps: poll, task, reaction, pin and vote do not reach other clients live

- Steps: member has `#general` open (and "Công việc" tab for tasks). Owner creates/changes via API or UI.
- Expected (plan flow 5): other user sees the change without refresh.
- Actual: plain messages arrive in ~250 ms. Poll creation, task creation, task status change, reaction, pin, and poll vote are NOT rendered on the other page within 5 s; they appear only after reload (poll verified by reload). Evidence: `reports/e2e/flow5-poll-task-live.json`, `flow5-subfeatures-live.json`, screenshots `ui/80-poll-live-check.png`, `ui/81-poll-after-reload.png`.
- Root cause: only the REST message send broadcasts (`backend/services/messaging/internal/rest/handler.go:208`). `rest/polls_tasks.go:16-57, 89-125` and `rest/reactions_pins_receipts.go` never call `hub.BroadcastToChannel`. `backend/pkg/realtime/event.go` has no poll/task/reaction/pin kinds. Not changed in working tree.

### F6 [HIGH, product decision] Asset request → approve → assign cannot run in a normal tenant

- Steps: member files an asset request; owner (only approver) approves-and-assigns.
- Actual: member `POST /api/workspaces/:id/asset-requests` 403 `no write access` (`backend/services/asset/internal/domain/requests.go:60` needs `write` on the type OA). Owner cannot approve own request (403). No other approver is possible: the permission editor exposes only documents, channels and management (`GET /permission-areas`), and the Owners role is not assignable by invitation (404 `role not in this workspace`).
- Works: lifecycle (requested→available via `approve` transition, maintenance, retire), owner hand-over and return, history with actor names, 409 `state_changed` race on concurrent transitions.
- Note: new assets start in `requested` and need an approve transition before they can be assigned. UX confusion.
- Decision needed: expose asset-type grants in the permission editor, or define who requests and who approves in a single-owner tenant.

### F7 [MEDIUM] Rejection with empty reason accepted

- Steps: `POST /api/approval/reject` with `comment: ""`.
- Actual: 200 `rejected`. Expected 400 (asset spec requires a reason; approval screen spec says the reason is the comment).
- Root cause: `backend/services/approval/internal/domain/execution.go:272-276` checks ids only; `rest/decisions.go:87` passes through.

### F8 [MEDIUM] Production edge: missing security headers, version disclosed

- `https://nexus.zaneng.xyz/`: present `X-Content-Type-Options: nosniff`, `X-Frame-Options: SAMEORIGIN`. Missing: `Strict-Transport-Security`, `Content-Security-Policy`, `Referrer-Policy`, `Permissions-Policy`. `server: nginx/1.31.6` discloses version.
- Repo config adds only two headers: `deploy/nginx/default.conf:36-37` (and 159-160 for assets). HSTS may exist in Traefik static config on the server (not in repo; unverified).

### F9 [MEDIUM] `#general` shows "1 thành viên" while every workspace member can read and post

- Steps: open `#general` as a member in a 5-person workspace.
- Actual: count shows 1 (owner). Member API GET/POST on the channel returns 200/201; `channel_members` has 1 row.
- Root: member count comes from the `channel_members` cache (`backend/services/messaging/internal/store/store.go:69,174`). Spec calls it denormalized; the real membership is the policy assignment. Count is misleading.

### F10 [LOW] Foreign or nonexistent workspace listings answer 200 `{}`, not 403

- `GET /api/workspaces/:foreign/drive` and `GET /api/workspaces/:foreign/asset-types` return 200 `{}`, identical to a nonexistent id. No data and no existence oracle, but inconsistent with the 403 used on other routes. Asset types handler: `backend/services/asset/internal/rest/asset_types.go:36`.

### F11 [LOW] Hand-over of an already assigned asset transfers without a holder check

- Two concurrent hand-overs of one asset both return 200 (last write wins). Spec `asset-authorization:129` allows hand-over from `assigned`, so by spec. Confirm intent: transfer silently, or require current holder.

### F12 [LOW] Duplicate-key ERROR logs on concurrent approval assignments

- Postgres logs `duplicate key ... uq_aa_request_step_user` five times during quorum/concurrency tests. Outcome correct (quorum passed). Error noise. Source: approval assignment insert path (not traced further).

### F13 [LOW] Redpanda memory below recommended

- Redpanda logs `Memory: 335544320 below recommended: 1073741824`. Prod compose uses `--memory 320M` with 640M limit (`deploy/docker-compose.prod.yml`, redpanda). Capacity risk, not a functional failure.

### F14 [LOW] `deploy.sh` image prune also matches the MinIO mirror

- `deploy/deploy.sh` prunes `ghcr.io/${OWNER}/nexus-[a-z-]+:` images other than current/previous tags. That pattern matches `nexus-minio:RELEASE...`. In-use images fail silently (`|| true`), unused mirror would be removed. Not executed here (needs `/opt/nexus` layout and would prune a host image). Replicated compose steps instead.

### F15 [LOW] Duplicate file names allowed in one folder

- Two `hop-dong-ui.pdf` in the same folder. Possibly intended; confirm.

### F16 [INFO] `docker kill` leaves messaging stopped under `unless-stopped`

- Docker treats `docker kill` as a manual stop; the container stayed exited (137) and was not restarted. A crash-driven restart could not be reproduced here (host-side kill not permitted; PID 1 ignores in-namespace SIGKILL). Restart-on-crash is unverified.

### F17 [INFO] OTP request rate limit

- After about 4 OTP requests per identifier in ~15 min, sign-in returns "Bạn đã xin mã quá nhiều lần. Thử lại sau 13 phút." Intended; affects test friction only.

## Flow matrix

Status: PASS / FAIL / PARTIAL / NOT TESTED (reason). Evidence JSON in `reports/e2e/`.

| # | Flow | Status | Notes |
|---|---|---|---|
| 1 | OTP sign-in, profile, create workspace (name only), selection | PASS (UI) | Email verified by SQL: onboarding blocks until verified, and no verify method exists when SMTP and Google are off. Fixed code does not prove email (`otp_proves_email=false`), by design. |
| 2 | Invite by email, accept on selection, leave, last-owner refusal | PASS (UI + API) | Invitee verified by SQL. Last-owner message shown in UI and API `reason:last_owner`. |
| 3 | Admin: roles, permission editor, assign role/dept, delegation guard, dept move/delete | PARTIAL | API: guard refuses grant of `invite` the manager lacks (403) and role assignment conferring it (403); owner can. UI: roles list and panel PASS; editing via UI NOT exercised. Dept move PASS, move-into-self refused PASS, delete re-homes members (by design) PASS. Delete strands approvals: FAIL (F4). |
| 4 | Drive: upload via presigned URL, folders, move-into-self, share, trash/undo, quota 413, foreign URL | PARTIAL | Upload through storage host PASS (UI + API), download bytes match PASS, share write → grantee lists file (UI) and downloads (API) PASS, folder create/rename/move PASS, move-into-self refused PASS, trash/undo PASS (API), 413 `quota_exceeded` PASS (quota set via SQL), foreign folder by id 403 PASS, foreign listing 200 `{}` (F10). UI rename/move/trash NOT exercised. |
| 5 | Chat: channel, message, thread, reaction, pin, poll, task, realtime | PARTIAL | Message live PASS (~250 ms both ways). Reaction, pin, poll, vote, task: FAIL (F5). Thread NOT tested. |
| 6 | Approval: templates, routing by conditions, person / role-in-dept / department approvers via real policy, quorum 2, reject, audit, outsider 403 | PASS (API), with FAILs | Manager create PASS; member create/edit 403 PASS; routing by `amount` PASS; each approver type through real policy PASS, incl. denies (step-order, wrong person); live revocation of role gives 403 PASS, restore PASS; quorum 2: concurrent PASS, single approval keeps `pending`, duplicate refused 409; audit names actors PASS; rejection reason in audit PASS; outsider 403 on detail and audit PASS. FAIL: tenant schema missing (F2), empty reject accepted (F7), notifications (F3), stuck after dept delete (F4). Approval UI actions NOT exercised. |
| 7 | Assets: type with fields, asset, request, approve-and-assign, return, lifecycle, 409 races | PARTIAL | Type with custom fields PASS; required field blank 400 PASS; lifecycle approve/flag/retire PASS; hand-over and return PASS; history actor PASS; 409 `state_changed` race on transitions PASS; outsider read 403 PASS. BLOCKED: member request 403, owner self-approve 403, approve-and-assign (F6). |
| 8 | Documents: create, autosave, two-tab conflict, sanitisation, write-without-read | PASS (UI) except write-without-read | Autosave PASS. Stale save 409 with server content PASS (API), conflict dialog and "Tải lại bản mới nhất" PASS (UI, two tabs). Sanitisation PASS: script, onerror, `javascript:` link and iframe stored raw and did not execute or render as live markup. Write-without-read NOT TESTABLE: no product path to a write-only grant. |
| 9 | Settings: profile edit, department read-only, theme persistence, workspace details | PASS (UI) | Department and login email read-only ("Do quản trị viên đặt") PASS; name edit persists after reload PASS; dark theme persists PASS; member sees workspace name read-only PASS. Owner workspace edit NOT tested. |
| 10 | Contacts: directory paging, profile panel, DM | PASS (UI) except paging | Directory, profile panel, "Nhắn tin" opens DM PASS. Paging NOT TESTABLE with 5 users. |
| 11 | Mobile 360px: bottom bar, sheet, workspace switch, top-bar search | PASS (UI) | Bar shows Tin nhắn, Tài liệu, Phê duyệt, Thêm. Sheet PASS. Switch A→B re-scopes: top bar and channel list change PASS. Top-bar search appears only on screens with search (Tài liệu) by design; focus lands in search box PASS. |
| 12 | Cross-tenant: outsider sees nothing | PASS (UI), FAIL (API unread) | UI: no tenant-A message live, no A names in directory, no A file names by URL PASS. API: search 403, messages 403, members 403, contacts 403, post 403, notifications no leak PASS, approval templates no leak PASS. FAIL: unread list (F1). Asset-types 200 `{}` (F10). |
| 13 | Resilience: messaging kill and restart, policy restart | PASS | Kill messaging: post refused 504 during outage; open page no errors; `docker start` healthy in 6 s; new message reaches member live on a new socket. Policy + policy-read restart: approval suite identical before and after (29 PASS; same known defect). Auto-restart after crash NOT verified (F16). |

## Production no-login smoke (`https://nexus.zaneng.xyz`)

Evidence: `reports/e2e/prod-smoke/`. No login, no data created.

- `/` → 200 (SPA)
- `/api/auth/providers` → 200 `{"google":true,"otp":true,"otp_fixed_code":false,"otp_proves_email":true}`
- `/api/me` → 401 `session_required`
- `http://nexus.zaneng.xyz/` → 308 to `https://nexus.zaneng.xyz/`
- Storage `https://storage.nexus.zaneng.xyz/minio/health/live` and `/ready` → 200
- Headers: nosniff and X-Frame-Options present; HSTS, CSP, Referrer-Policy, Permissions-Policy absent (F8)
- Malformed requests (invalid JSON to OTP request, `null` body, unknown `/api/*`, `/api/admin/x`, bad UUID path): 400/401/404 with short JSON. No stack traces or internal text. Unknown `/api` returns `{"error":"not found"}`.

## Build and deploy status

- Images: 10 nexus images pulled from GHCR at `d9817e49571364d9e26c3c52635f18b238d778e1`. Revision labels equal the tag. No local builds needed. Source tree at `d9817e4` identical to `git diff` (no change under backend, frontend, data, deploy).
- MinIO: local `ghcr.io/viethoangnguyenle/nexus-minio:RELEASE.2025-09-07T16-13-09Z` (same as prod).
- Migrations: 35 files applied to a fresh DB through the ledger path (`migrate.sh`), with the ledger written.
- Compose: `docker compose config` valid with the override.
- Container health: all 14 services healthy after start (postgres, redis, redpanda, nexus-minio, policy, policy-read, auth, workspace, document, messaging, asset, drive, approval, nexus-frontend).
- Error scan of logs: auth, policy, policy-read, drive, workspace, asset, document, approval, nginx: 0 ERROR lines. Messaging: 65 `notification not recorded` (F3). Postgres: 65 notification FK (F3) and 5 duplicate-key (F12). The other Postgres ERROR lines came from my own manual `psql` probes with wrong column names, not from the app. Redpanda: 1 memory warning (F13). No panics or goroutine dumps anywhere.
- Build/unit suites (`go test`, `vitest`, `lint`) were NOT run: scope was E2E on deployed images.

## Deviations from the brief

1. Compose project `nexus-e2e`, release layout copied to a scratch dir (as CI rsyncs). `deploy.sh` was not run verbatim (F14). Its steps (pull skipped, migrate, up --wait, policy force-recreate) were run by hand.
2. Override file (outside the repo, `reports/e2e/stack/override.yml`) sets three production values for local use: `AUTH_FIXED_OTP_CODE=999999`, `MINIO_PUBLIC_ENDPOINT=storage.localhost:18080` with `MINIO_PUBLIC_SECURE=false`, and MinIO CORS origin for localhost. No change to repo files.
3. Local edge: a Traefik v3.4 container on a created `traefik` network on 127.0.0.1:18080, file provider, no TLS. nginx config unchanged. Hosts `localhost` and `storage.localhost`.
4. Approval schema provisioned by an internal call (F2 workaround).
5. Email verification by SQL (`UPDATE users SET email_verified_at`), the same as earlier smokes.
6. Browser automation: Playwright from the gstack package, as in earlier reports. The global CLAUDE.md asks for the `/browse` skill for web browsing; this run used Playwright directly because the brief names it as the method and it gives per-step screenshots and DOM assertions.
7. Test users have `example.test` addresses. World data lived only in the local stack, now removed.

## Coverage gaps (not tested)

- Google sign-in, OTP email delivery from "Nexus", SMTP, logout (plan items for the manual checklist).
- Approval actions through the UI (approve/reject/batch), template editor UI, approval detail UI audit (API only).
- Drive UI rename, move, trash/undo, grantee download (API only).
- Asset UI flows beyond the lifecycle (API only).
- Thread replies, contacts paging, owner workspace edit, admin permission editing via UI.
- Write-without-read document case: no product path.
- Crash-driven auto-restart (F16).
- Build, unit and lint suites.

## Release recommendation

**NO-GO.** Required before tagging:

1. Fix F1 (unread query column) and add a cross-tenant regression test. Blocker.
2. Provision approval schema on workspace creation (or on first use) and cover it in a test. Blocker (F2).
3. Ship and test the notifications WIP for F3; verify approval notifications end to end.
4. Make department delete and request creation refuse dangling approver steps (F4).
5. Broadcast poll, task, reaction, pin and vote changes over WebSocket (F5), per plan flow 5.
6. Decide the asset approval model for single-owner tenants (F6) and implement it.
7. Refuse empty rejection reason (F7). Add HSTS, CSP, Referrer-Policy, Permissions-Policy at the edge (F8).
8. Re-run this suite against the candidate images. Expected to take about one hour with the harness in `reports/e2e/harness/`.

Lower-priority items (F9–F17) can follow the first release with tickets.

## Manual checklist for production (Vietnamese)

1. Mở https://nexus.zaneng.xyz trong cửa sổ ẩn danh. Trang đăng nhập hiện, có nút "Tiếp tục với Google" và ô "Email hoặc số điện thoại".
2. Đăng nhập bằng Google với tài khoản thử nghiệm. Xác nhận vào được màn "Chọn workspace" và tên hiển thị đúng.
3. Đăng nhập bằng email: nhập email thử, nhấn "Nhận mã đăng nhập". Mã 6 số phải đến hộp thư, người gửi là "Nexus", và có hiệu lực 5 phút. Kiểm tra cả mục Thư rác.
4. Nếu chưa có workspace, tạo một workspace chỉ với tên. Xác nhận vào được kênh #general và tên workspace hiển thị đúng.
5. Trong "Tài liệu", tải lên một tệp PDF nhỏ (dưới 5 MB). Xác nhận tệp xuất hiện trong danh sách, rồi tải xuống được. Khi di chuột vào liên kết tải, host phải là storage.nexus.zaneng.xyz và giao thức https.
6. Mở cùng workspace bằng một trình duyệt thứ hai (tài khoản thứ hai hoặc cửa sổ ẩn danh khác). Gửi tin nhắn từ cửa sổ thứ nhất. Cửa sổ thứ hai phải thấy tin ngay, không cần tải lại trang.
7. Trong cửa sổ thứ hai, mở "Phê duyệt" và "Tài sản". Nếu thấy lỗi "tenant schema" hoặc 404, ghi lại: đây là lỗi F2 đã biết.
8. Đăng xuất: vào Cài đặt, chọn "Đăng xuất". Xác nhận quay về trang đăng nhập và tải lại trang không còn phiên.
9. Trên điện thoại (hoặc chế độ 360 px): thanh dưới có 4 mục (Tin nhắn, Tài liệu, Phê duyệt, Thêm). Mở "Thêm", đổi workspace, và xác nhận dữ liệu đổi theo.
10. Chạy `curl -sI https://nexus.zaneng.xyz/ | grep -i strict` và ghi lại kết quả. Hiện tại không có HSTS (F8).

## Unresolved questions

1. Do existing production tenants have approval schemas? Needs a read-only check on prod DB (`SELECT tenant_id FROM tenant_schemas`). Not done (no prod access, by brief).
2. Does the production Traefik add HSTS? Its static config is not in the repo.
3. Product decision on asset approvals in single-owner tenants (F6).
4. Should a department with approval references be undeletable, or repointed (F4)?
5. Should foreign-workspace listings answer 403 or stay silent 200 (F10)?
6. Is `#general` meant to include every workspace member (F9)? The policy already allows it.
7. Are poll, task, reaction and pin changes meant to be live? Plan flow 5 says yes; code does not push them (F5).
8. Is the working-tree notifications fix (F3) complete? It was not in the tested images.
9. Hand-over of an assigned asset: transfer silently, or require the current holder (F11)?
10. How should the write-without-read document case be built for tests, since no product path grants write without read?

## Evidence index

- Report: `plans/261010-2205-first-release/reports/e2e-report.md` (this file)
- Results JSON: `plans/261010-2205-first-release/reports/e2e/*.json`
- Screenshots: `plans/261010-2205-first-release/reports/e2e/ui/*.png` (FAIL-* files show the failing state)
- Container logs: `plans/261010-2205-first-release/reports/e2e/logs/*.log`
- Production smoke: `plans/261010-2205-first-release/reports/e2e/prod-smoke/`
- Harness: `plans/261010-2205-first-release/reports/e2e/harness/`
- Stack overrides (no secrets): `plans/261010-2205-first-release/reports/e2e/stack/`
