# Phase 11 verification: Danh bạ kiểu Lark (2026-10-18)

Worktree `nexus-enterprise-contacts`, branch `feat/contacts-lark`, HEAD `82fdd27`. Dev stack from the worktree
(`COMPOSE_PROJECT_NAME=nexus-enterprise make -o dev-infra dev`, `.env.dev` linked read-only then removed, `make dev-stop` at end).
Seed: `scripts/seed-contacts-demo.sh` (idempotent, re-run: no change). Accounts: owner = duc.tran (Trần Minh Đức),
limited role = lan.nguyen (Nguyễn Thu Lan, Đối soát under Khối Vận hành, role Cộng tác viên), plain = vinh.le (Lê Quang Vinh,
Nền tảng), hidden dept = yen.pham (Phạm Hải Yến, Tuân thủ under hidden Pháp chế).
Screenshots: `/tmp/claude-1000/-home-zane-Desktop-projects-nexus-enterprise/fa5efd71-e455-4086-a504-8bc07c2afdf8/scratchpad/p11/`
(`root-<acct>-<width>-<light|dark>.png` ×32, `hidden-*`, `admin-*`, `pc-*`, `guest-*`, `action-*`, `probe-*`, etc.).

## Gates

| Gate | Result |
|---|---|
| `make build-check` | PASS |
| `make check-layering` / `check-ngac` / `check-docs` | PASS |
| `make test` (all services, TEST_DATABASE_URL = ngac_ci) | PASS, 0 skips, no notification drift failures seen |
| `make test-realpolicy` | PASS, 117 `TestRealPolicy_*` pass |
| `npm test` | PASS, 302 files / 3769 tests |
| `npm run lint` | PASS |
| `npm run build` | PASS, with router warnings (see F6) |

Dev DB, read-only SQL (no `db-migrate` run; 064 already applied, shown by `Directory_*` nodes):
- 118 workspaces. 116 have `Directory_<id>` + one `DirectoryLimitedRule_<id>` prohibition. The 2 without are
  `test-ws-*` leftovers from 2026-10-10 (not NovaPay). Personal workspaces: 90, all with directory nodes.
- `tenant_users.department_id` vs `departments.workspace_id` drift: 0 rows.
- NovaPay: 7 active members, 5 with a department.

## 1. Danh bạ

| Check | Result | Evidence |
|---|---|---|
| Categories Phòng ban / Vai trò / Nhóm, all 4 accounts | PASS | root-*.png |
| Root load: no fetch of group members or role lists | PASS | root load calls = contacts, departments, workspaces, chat-state, notifications, me only. Category clicks made 0 API calls. Opening a group calls `/channels/:id/members` only then. |
| Breadcrumbs | PASS | owner "Toàn NovaPay › Khối Công nghệ › Đối soát"; Lan "Phạm vi của bạn" |
| Search within scope | PASS | Lan "Vinh" → "0 người khớp", "trong phạm vi bạn được xem". Lan "Tùng" → 1. API `?q=`/`?name=` is ignored, so filtering is client-side over the already scoped list. |
| Nhắn tin | PASS | Vinh → Bảo opens `/channels/<dm>` |
| Gọi | PASS | Vinh → Bảo opens `/calls/<id>` even though Bảo is offline (mic blocked by headless, environment only). A call record was left open (F9). |
| Gửi email | PASS | `mailto:bao.hoang@example.test`; "Sao chép email" also present |
| Own profile | PASS | "Trần Minh Đức (bạn)" card shows "Sửa hồ sơ", Email, role Quản trị viên |
| Personal workspace | PASS | "Thành viên · 1 người trong không gian này" |
| Empty / no-match state | PASS | "Không có ai khớp … trong phạm vi bạn được xem" |
| Loading / 503 error state | NOT RUN | not forced |
| Layout: 1440, 1024, 820, 375 × light/dark × 4 accounts | PASS | 32/32 root shots: no horizontal scroll, no console error, no UUID in innerText |

## 2. Visibility (`GET /api/workspaces/:id/contacts`, same org)

| Account | visibility | total | People returned |
|---|---|---|---|
| owner duc.tran | all | 7 | all, Yến flagged via dept list `hidden: true` on Pháp chế |
| lan.nguyen (limited) | limited | 3 | Lan (self), Tùng (same dept), Đức (owner) |
| vinh.le | all | 6 | all except Yến |
| yen.pham | all | 7 | all (member of hidden dept) |

- Departments: Vinh gets no Pháp chế / Tuân thủ. Lan gets only Đối soát (her own). Owner gets all 7.
- Probes, Vinh and Lan: `?department_id=<Pháp chế>` → 0; `?department_id=<Tuân thủ>` → 0 for Vinh and Lan. `/members` for Vinh → 6, no Yến.
- Owner rows expose no department for Đức (empty). Vinh's list shows no Yến name or department.
- `?person=<Yến id>` for Vinh and Lan: no card, no email, no hidden-message. No leak, but the URL probe fails silently (F7).
- PASS overall.

## 3. Hidden person in content

| Check | Result | Evidence |
|---|---|---|
| Vinh sees Yến in shared group "Pháp chế phối hợp": name + avatar | PASS | hidden-chat-*.png |
| Press Yến → "Hồ sơ này không còn hiển thị với bạn.", no email, no actions | PASS | hidden-card-vinh-1440-light.png, hidden-card-vinh-375-dark.png |
| Press visible Đức → full card (Nhắn tin, Gọi, Gửi email) | PASS | chat-vinh-open-duc.png |
| Owner press Yến → full card with email and department | PASS | chat-owner-open-yen.png |
| @mention: `@Ph` lists "Phạm Hải Yến @yen.pham.example" for Vinh | PASS | mention-vinh-1440.png (a first `fill()`-based try did not open the dropdown; retried with real keystrokes) |
| Approval / documents / calendar render hidden names | NOT RUN | not covered |

## 4. Admin: Quản trị › Danh bạ (owner)

| Check | Result | Evidence |
|---|---|---|
| Hide: "Ẩn một phòng" → picker (subtree counts) → "Ẩn phòng" confirm | PASS | admin-hide-picker.png, admin-hidden-after.png |
| Unhide: "Bỏ ẩn" → "Bỏ ẩn Pháp chế?" confirm | PASS | admin-directory-unhide-after.png |
| Vinh's Danh bạ after unhide: Pháp chế → "1 người · 1 phòng con" → Tuân thủ → Yến | PASS | pc-vinh-1440-unhidden.png, tc-vinh-1440-unhidden.png |
| Effect on Vinh live (no reload, ≤2.5 s) and after reload, both directions | PASS | live and reload checks |
| Limit / unlimit role Cộng tác viên, confirm dialogs | PASS | admin-role-unlimit.png; "Giới hạn" restores limited |
| Effect on Lan: unlimit → API visibility `all`, 6 people (no Yến); re-limit → `limited` again | PASS | live and reload checks |
| Counts include subtrees in the admin rules view and pickers | PASS | Pháp chế "1 người" (includes Tuân thủ); owner Danh bạ "Khối Công nghệ 2 người" |
| Non-manager has no Danh bạ tab (Vinh, Lan, Yến) | PASS | tabs show Tổng quan / Người dùng / Vai trò; direct URL → "Bạn chưa có quyền…" |

State restored at end: Pháp chế hidden, Cộng tác viên limited (both verified by API and UI).

## 5. Pickers for a limited user

- Calendar guest picker (Lan, `/calendar/new`): "Vinh" → none; "Tùng" → 1; "Đức" → 1. Narrowed as intended.
- Approval approver department picker: reads `GET /departments`, which returns only Lan's department for her (verified by API). Lan has no "Mẫu phê duyệt" tab, so the picker was not exercised in the UI. Partial, not a defect.

## 6. Performance (Vite :5173, seeded org of 7 people / 7 departments)

| Measure | n | p50 | p95 | max |
|---|---|---|---|---|
| `GET /contacts` client, owner | 60 | 4.3 ms | 5.1 ms | 23.0 ms |
| `GET /contacts` client, lan | 60 | 5.3 ms | 6.2 ms | 6.8 ms |
| `GET /contacts` client, vinh | 60 | 5.3 ms | 6.3 ms | 6.6 ms |
| `GET /contacts` client, yen | 60 | 5.1 ms | 5.9 ms | 6.4 ms |
| `GET /contacts` server (auth access log; includes the `GetDirectoryScope` gRPC) | 120 | 4.1 ms | 4.8 ms | 5.3 ms |
| `POST /contacts/resolve`, 200 ids (7 real + 193 unknown), owner | 60 | 1.8 ms | 2.9 ms | 3.1 ms |
| `POST /contacts/resolve`, 200 ids, lan | 60 | 1.8 ms | 2.3 ms | 2.4 ms |
| `POST /contacts/resolve` server | 100 | 0.9 ms | 1.2 ms | 1.6 ms |

- All under the 50 ms target. Vite live check of `/api/workspaces/<id>/contacts/resolve`: 200 OK, 7 people returned.
- `GetDirectoryScope` alone was not timed (gRPC only, no timing log). The `/contacts` server p95 (4.8 ms) is an upper bound for it.
- No caching added. Org is small, so these numbers say nothing about larger orgs.

## 7. Console, scroll, raw ids

- Console errors: none on Danh bạ, chat, or profile flows. Only 403 console errors when a non-manager loads `/admin/directory` directly (F5).
- Horizontal scroll: none in any shot.
- UUIDs on screen: none (regex on innerText). The API payload carries `user_id`, `ngac_node_id`, `department_id`. None reach the screen.

## Defects and concerns

- **F1 (minor).** Lan's "Phòng của tôi" row repeats its own name as sub-label ("Đối soát · Đối soát"), because the parent is blanked for a limited user. Evidence: root-lan-1440-light.png. Fix: show no sub-label, or show the scoped path only when it adds information.
- **F2 (minor, decision needed).** Counts differ by screen. Quản trị › Tổng quan and `GET /departments` `member_count` are direct only (Khối Công nghệ 0, Pháp chế 0). Danh bạ and the admin rules view include subtrees (Khối Công nghệ 2, Pháp chế 1). The plan says "counts include subtrees". Confirm whether Tổng quan should follow.
- **F3.** Same as F2, for the `member_count` field in `GET /departments`.
- **F4 (minor).** Each Danh bạ load sends `GET /contacts?limit=500` 2–3 times. Probably duplicate consumers; not located.
- **F5 (minor).** A non-manager who loads `/admin/directory` directly gets two 403 console errors. Expected denial, but it breaks the "no console errors" bar.
- **F6 (build, likely pre-existing).** `npm run build` warns that `*.test.tsx`/`*.test.ts` under `src/routes/` are not route files (six or more). Not in contacts files. Fix via router `routeFileIgnorePattern`.
- **F7 (info).** `?person=<hidden id>` for a limited or plain user is silently ignored. No leak.
- **F8 (question).** Lan's search "Yến" returns "Nguyễn Thu Lan (bạn)". Accent-folded substring match ("yen" inside "nguyen"). Expected for substring search. Should it match word starts only?
- **F9 (test data).** Dev data changed: group "Pháp chế phối hợp" (private; Yến, Vinh, Đức), 3 messages, one call record (Vinh → Bảo). Not deleted. Remove?
- **F10 (by design, note).** `/contacts/resolve` returns the hidden Yến to Lan. Names in content resolve for any workspace member (plan decision C9).
- **F11 (info).** Leftover `test-ws-*` workspaces (2026-10-10) without directory nodes. Harmless.
- **F12 (dev only).** The dev TanStack devtools button sits over the bottom-left card on 375 px. Not product.

## Not covered

- Approval, documents and calendar rendering of hidden names.
- Danh bạ loading and 503 error states.
- Dark-mode detail views beyond the root matrix.
- Approval department picker in the UI (only the `/departments` data was checked).
- Release A / B deploy, prod 064 dry-run, prod smoke (controller).
- `db-migrate` second run: not done, 064 already applied.

## Status

DONE_WITH_CONCERNS. Gates green. Visibility, hidden-person, admin-live and perf checks pass. Concerns: F1 to F5 (minor), F2 needs a decision, and the gaps above.

## Unresolved questions

1. Should Quản trị › Tổng quan and `GET /departments` counts include subtrees (F2/F3)?
2. Should the hidden-person names in approval, documents and calendar be checked before release A (not covered here)?
3. Remove the fixture group and call record on dev (F9)?
4. Word-start matching for search (F8)?
