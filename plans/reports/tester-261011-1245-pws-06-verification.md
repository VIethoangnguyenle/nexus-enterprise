# PWS-06 verification: personal workspaces on real stack

Date: 2026-10-11. Branch claude/inspiring-cray-32y4n1. Script: `frontend/e2e/personal-workspace.mjs`.
Stack: dev (Vite :5173) untouched. No app code edited. No commits.

**Status: PARTIAL.** Plan.md acceptance A1–A4 PASS. Exec steps 5 (Tạo workspace) BLOCKED by environment (email verification unavailable). Step 6 PASS on the personal workspace pre-conversion.

## Gates

| Gate | Result | Evidence |
|---|---|---|
| `make test` | PASS | 48 packages `ok`, 13 `no test files`, 0 FAIL. Log in scratchpad `make-test.log`. Panic lines are recover-middleware tests logging by design. |
| `make check-layering` | PASS | "shared helpers, gRPC options, REST→domain, no SQL in transports, no dropped store errors" |
| `make check-ngac` | PASS | "no inline operation strings or node-name patterns outside backend/ngac" |

Frontend gates skipped per exec.

## Plan.md acceptance

| # | Criterion | Result | Evidence |
|---|---|---|---|
| A1 | Personal: no Tài sản / Phê duyệt in sidebar, mobile, other screens | PASS | 1b desktop: visible `a[href=/assets]`=false, `a[href=/approval]`=false. 1c phone tab bar = Tin nhắn, Tài liệu, Danh bạ (Danh bạ replaces Phê duyệt per plan decision 3); Thêm sheet = /admin, /settings. 3b: no Tài sản area in Roles matrix (see 6). |
| A2 | `/assets`, `/approval` redirect to main page | PASS | 2-assets: pushState /assets → landed /channels, 0 area API calls. 2-approval: → /channels, 0 calls. Phone /assets → /channels, 0 calls. |
| A3 | Asset and approval API refused (403) for personal | PASS | 3a: `GET /api/workspaces/{ws}/assets` → 403 `{"message":"not available in a personal workspace","reason":"personal_workspace"}`. 3b: `POST /api/approval/requests` → 403 `{"message":"access denied: not available in a personal workspace"}`. |
| A4 | After convert to organization, Phê duyệt + Tài sản appear and work without reload | PASS | 4a: type after = organization; sidebar assets=true, approval=true; same page, no goto. 4b phone: tab bar has /approval, Thêm has /assets. 4c: GET assets → 200; POST approval → 404 `get template: not found` (passed authz; fake template id). Neither refused for "personal". |

## Exec steps

| Crit | Step | Result | Evidence |
|---|---|---|---|
| 1a | 1. New account → personal workspace named after person | PASS | `GET /api/workspaces`: `[PWS Kiểm Thử, personal]`. Switcher: "PWS Kiểm Thử / Workspace cá nhân". Screens: `01-desktop-personal-sidebar.png` |
| 1b | 1. Desktop sidebar no Tài sản / Phê duyệt | PASS | See A1. |
| 1c | 1. Phone tab bar / Thêm no Tài sản / Phê duyệt | PASS | See A1. `01-phone-personal-tabbar.png`, `01-phone-personal-more-sheet.png` |
| 2 | 2. Deep links /assets, /approval | PASS | See A2. `02-deeplink-assets.png`, `02-deeplink-approval.png`, `02-phone-deeplink-assets.png` |
| 3a/3b | 3. Direct API calls refused | PASS | See A3. |
| 4a–4c | 4. Convert in Cài đặt → Workspace | PASS | See A4. Screens: `04-settings-workspace-before.png`, `04-convert-dialog.png`, `04-desktop-organization-sidebar.png`, `04-phone-organization-tabbar.png`, `04-phone-organization-more-sheet.png` |
| 5a | 5. Tạo workspace: default Cá nhân, creates personal | BLOCKED | Create screen on /onboarding shows "Cần xác minh <email> trước khi tạo workspace" and no type picker (`input[name=cw-type]` count 0). Applies to the new account and to the pre-existing smoke1791570521 account (also unverified). No verified account was found. Root cause: `POST /api/me/email/verify` → 503 `verification_unavailable` ("email verification by code is not available"). Screen: `05-create-workspace-default.png` |
| 5b | 5. Quay lại returns to origin | BLOCKED | Same gate; not exercised. |
| 6 | 6. Admin → Roles, permission areas of personal workspace: no Tài sản | PASS | Run on the personal workspace before conversion (step 3b). Roles tab → "Chủ sở hữu" (1 member) permissions: Tài liệu (Xem, Sửa, Chia sẻ), Tin nhắn (Xem, Sửa, Quản lý, Mời, Tạo nhóm chat), Quản trị (Quản lý, Mời). No Tài sản. Screen: `06-admin-role-matrix-personal.png`. |
| 6b | 6. Same check on the workspace created in step 5 | BLOCKED | No workspace created (5a). |

## Console and network (per step)

- Console errors: only resource-load messages from deliberate calls (403 on step 3 and step 4c's 404, 503 on 4d). No `pageerror`, no uncaught exceptions.
- Failed requests: `403 GET .../assets` and `403 POST /api/approval/requests` (step 3, expected); `404 POST /api/approval/requests` (step 4c, expected post-authz); `503 POST /api/me/email/verify` (4d, environment).
- A transient "Mất kết nối. Đang thử kết nối lại…" banner was visible in one screenshot (`05-create-workspace-default.png`). Probably the hot reload from concurrent frontend edits. Not investigated.
- Step 3 first attempt threw "Execution context was destroyed" (navigation during a reload). Retry passed. Treated as transient.

## Findings

1. **[Blocker, environment] New accounts cannot create workspaces here.** Create screen gates on verified email. Verification by code is unavailable in dev (503). Blocks exec step 5 and 6b. Not an app-code defect I could confirm; it is a configuration state. Decision needed: enable a dev verification channel, or allow a dev-only verify fixture. I did not write to the DB to flip `email_verified_at`, because that bypasses a security gate.
2. **[Spec inconsistency] Quản trị is visible in personal workspaces.** Exec step 1 says "no Tài sản / Phê duyệt / Quản trị". Exec step 6 needs Admin → Roles in a personal workspace, which requires Quản trị. Observed: Quản trị shows in the desktop sidebar and phone Thêm sheet. Script treats Quản trị as recorded only. Needs a decision.
3. **Deep links are simulated in-app.** Typing the URL reloads the page, which would drop the in-memory token. The script used `history.pushState` + `popstate`. This exercises the router's in-app path. A cold-load of `/assets` was not tested.
4. **Sign-in count.** Multiple runs created new accounts (one per run), each signed in once per run. Earlier runs failed before reaching the steps. Each run used a fresh address, so OTP rate limits were not hit.

## Test accounts created (leave in place)

| Email | Used for | Note |
|---|---|---|
| pws-1791697464407@example.org | run 1 | Signed in, personal ws "PWS Kiểm Thử". Failed earlier script. |
| pws-1791697767726@example.org | run 4 | Reached create screen; gated by verification. |
| pws-1791698322843@example.org | run 7 | Steps 1–4 ran; personal ws converted to organization in step 4. |
| pws-1791698399660@example.org | run 8 (final) | Steps 0–4d, 6 (pre-convert) ran. Converted to organization in step 4. |
| run 2, 5, 6 accounts | failed sign-ins | Emails not logged. |
| probe-1791…@example.org | curl OTP probe | OTP requested only. |
| smoke1791570521@example.org | pre-existing | Signed in on page 2 for the step 5 gate check. Still unverified. |

Note: run 8 account's personal ws was converted to organization in step 4.

## Screenshots

`plans/261010-2258-personal-workspace/reports/phase-06-screens/`: 01-*, 02-*, 04-*, 05-*, 06-* plus `results.json` (full criteria, console, network, step timing).

## Progress log

`plans/reports/progress-pws-06.log` (exec referenced `agy-progress-…`; path changed per instruction).

## Next steps

1. Decide Quản trị visibility in personal workspaces (Finding 2). Update exec step 1 or the code.
2. Unblock step 5/6b: enable a dev email-verification channel or a dev fixture; then rerun. Script step 5 is ready to resume.
3. Re-run after the hot-reload churn settles, to confirm 1–4 are stable.

## Unresolved questions

- Should Quản trị be hidden in personal workspaces? Exec and plan disagree.
- How should email verification work in dev so Tạo workspace can be tested? Need approval before any DB fixup.
- Is the transient "Mất kết nối" banner from HMR or a real websocket drop?
