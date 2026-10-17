---
title: "Danh bạ kiểu Lark — đợt 1"
description: "Category-first Danh bạ (Phòng ban, Vai trò, Nhóm của tôi), NGAC-decided contacts visibility with admin rules, and a name resolver separate from the directory."
status: pending
priority: P2
effort: 67h
branch: feat/contacts-lark
base: main (1dbbbe7)
tags: [contacts, ngac, frontend, workspace, auth, design-first]
created: 2026-10-17
blockedBy: []
resolvedBlockers: [261017-1645-authz-overprivilege-fixes (main a9a19b4 #30, d342941 #31, 1dbbbe7 #32), 261017-1516-search-suggestions (main 43a1509)]
blocks: []
brainstorm: ../reports/brainstorm-261017-1542-contacts-lark-redesign.md
mockup: design/mockups/contacts-lark.html
specs:
  new: [contacts-visibility]
  modified: [contacts-documents-settings-screens, admin-screens, tenant-ngac-init, workspace-admin-authorization, calendar]
---

# Danh bạ kiểu Lark — đợt 1

Inputs (authoritative): brainstorm above (every bullet), mockup `design/mockups/contacts-lark.html` §1–§8 (approved
Zane 2026-10-17), security audit `plans/reports/security-261017-1630-authz-overprivilege-audit.md`, and what the
security plan actually shipped (see "Sync with main").

## Foundation this plan builds on (merged, not re-done here)
From `261017-1645-authz-overprivilege-fixes` (all on main):
- **Membership = a `U` node under `<ws>_Members`** (Owners sit under Members). One helper,
  `backend/pkg/policyclient/membership.go`: `MembersUA` (:44, process-wide id cache, anti-spoof PC check),
  `IsWorkspaceMember` (:81), `MemberWorkspaces` (:105), `SharedWorkspaces` (:138), `IsNotMembersUA` (:35). Read-after-write
  checks pass the **writer** client (it now carries `FindNodeByName`/`IsAssigned`, `proto/policy/policy_write.proto:43-44`).
- Workspace: `authorizeMember` (`workspace/internal/domain/authz.go:76`) and `memberUsers` (:106) use it over the writer;
  `ListMembers` (`members.go:233`) and admin `ListMemberDirectory` (`member_directory.go:137`, manage only) list the Members UA.
- Auth: `requireActiveMember`/`activeMemberRow` (`auth/internal/domain/membership.go:25,33`): row not `removed|disabled|invited`
  AND graph member via writer. `ListContacts` itself still checks only the active row (`users.go:114-120`, security choice 7:
  "No GetMe or ListContacts gate"); readers of `tenant_users` filter `status='active'` (`store/directory.go:130`).
- `tenant_users.status` allows `removed` (`062`); `063` tombstoned drift rows and cleared their department.
- Groups are private (`channel_type='private'`), `#general` is the one `channel_type='workspace'` channel with direct per-user
  edges; `ListChannels` returns only readable channels; members list carries `can_remove_members`
  (`frontend/src/api/messaging.ts:34`).
- DM/call gate: only between people sharing a workspace (`useCreateDM` sends `target_user_id`, `api/messaging.ts:274-277`).
- Real-policy harness: `backend/testutil/policy_process.go` + `policy_graph.go` (`//go:build realpolicy`,
  `StartPolicy`, `SeedWorkspaceGraph`), `make test-realpolicy` (`Makefile:469`) against `TEST_DATABASE_URL` = `ngac_ci`.
- Operator tool `policy invalidate-cache --all-workspaces`, `make policy-invalidate` (`Makefile:475`), `deploy.sh` step;
  runbook in `docs/deployment.md` (two-release rule, re-runnable/guarded/backed-up migrations, prod dry-run).
- Migrations end at `063_tenant_users_tombstone_reconcile.sql`; this plan owns **`064_directory_visibility.sql`**.

## Key decisions (made here; details in phase files)
1. **Visibility graph model** (phase 02): per-department directory OAs under a workspace `Directory` OA placed under the
   Mgmt OA; open subtree read via `<ws>_Members → DirectoryOpen {read}` (exactly the workspace's members under the new
   membership rule); hidden department = its Dir OA moved out of DirectoryOpen + `Dept UA → Dir OA {read}`; "own department
   only" = one static **prohibition** (subject `DirectoryLimited` UA, read, target DirectoryOpen) + a parallel peer-OA tree
   reached through a flat per-department `DeptPeers` UA; **owners always visible** = `DirectoryOwners` OA (under Directory,
   not under Open) with `<ws>_Members → DirectoryOwners {read}`, so the prohibition never reaches it; directory admins =
   `manage` on the Directory OA (inherited from Mgmt). One PDP change (the SQL fallback matches prohibitions with database ancestors before promoting an ALLOW, phase 02), no new operation. No directory UA sits under
   `<ws>_Members`, so none of them can make anyone a member.
2. **Workspace owns the scope, auth owns the people rows**: workspace computes `DirectoryScope` (NGAC checks, after its
   graph membership check) and serves it to auth over gRPC; auth filters `tenant_users` by it. One implementation of the
   rules (DRY).
3. **Browse vs resolve split**: `GET /contacts` = visibility-filtered directory (+ `department_id`, `roles`, `visibility`);
   `POST /contacts/resolve` = name + avatar for IDs the client met in content, any member status. Same `/contacts`
   prefix → no new vite/nginx/Traefik route entry.
4. **Roles ride the contacts payload** (brief item 2); Vai trò category groups loaded contacts client-side. Departments and
   groups load on open (own API).
5. **No schema columns; node ids found by name** (as `MembersUA` does): directory nodes are looked up with ID-keyed
   `backend/ngac` names, verified under the workspace PC, cached process-wide. Hidden state is read from the graph. This
   keeps release A free of any schema dependency (two-release rule).
6. **Existing data via migration 064 (graph-writing, release B)**, not a start-up Go backfill: the policy service refuses
   graph writes from a bare service identity (`policy/internal/grpc/authpolicy.go:22-37`, ServiceOK only for `auth` and
   `policy-ctl`), and the security plan's pattern for existing graph data is a guarded, re-runnable, backed-up SQL migration
   followed by policy restart + `invalidate-cache`.

## Phases

| # | Phase | Pri | Effort | Depends | Spec |
|---|---|---|---|---|---|
| 01 | [Design + UI specs](phase-01-design-and-specs.md) | P1 | 4h | — | contacts-documents-settings-screens, admin-screens |
| 02 | [NGAC visibility model + test vectors](phase-02-ngac-visibility-model.md) | P1 | 5h | 01 | contacts-visibility (new), tenant-ngac-init |
| 03 | [Workspace: directory nodes, upkeep, migration 064](phase-03-workspace-directory-graph.md) | P1 | 9h | 02 | tenant-ngac-init, contacts-visibility |
| 04 | [Workspace: scope, filtered reads, admin rules API, demo seed](phase-04-workspace-scope-api.md) | P1 | 8h | 03 | contacts-visibility, workspace-admin-authorization |
| 05 | [Auth: NGAC-decided contacts + resolver](phase-05-auth-contacts.md) | P1 | 5h | 04 | contacts-visibility |
| 06 | [Frontend data layer + PersonLink/HiddenPersonCard primitives](phase-06-frontend-data.md) | P1 | 6h | 05 | — |
| 07 | [Danh bạ desktop](phase-07-contacts-desktop.md) | P2 | 10h | 06 | contacts-documents-settings-screens |
| 08 | [Danh bạ tablet + phone](phase-08-contacts-tablet-phone.md) | P2 | 6h | 07 | contacts-documents-settings-screens |
| 09 | [Hidden person card across the app](phase-09-hidden-person-card.md) | P2 | 5h | 06 | contacts-visibility, calendar |
| 10 | [Quản trị › Danh bạ](phase-10-admin-directory-scope.md) | P2 | 5h | 04, 06 | admin-screens |
| 11 | [Verify, rollout, spec close-out](phase-11-verify-rollout.md) | P1 | 4h | 07–10 | all above |

Parallel after 06: {07→08}, 09, 10 — disjoint file ownership (07/08 `components/contacts/*` + `routes/_workspace/contacts.tsx`;
09 chat/approval/drive/calendar callsites only (primitives owned by 06); 10 `components/admin/DirectoryScope*`,
`routes/_workspace/admin/directory.tsx`, `AdminFrame.tsx`). 03 owns `data/migrations/064_*`, 04 owns `scripts/seed-contacts-demo.sh`.

## Data flow (end state)
```
admin (Quản trị) ─PUT rule→ workspace domain ─policyWrite→ policy (PAP→EPP invalidation)
member opens Danh bạ ─GET /contacts→ auth (active row) ─gRPC GetDirectoryScope(caller)→ workspace
        workspace: authorizeMember (writer, Members UA) → BatchCheck(read Dir/Peer/NoDept/Owners OAs; manage Directory) → PDP
        auth: SQL tenant_users(active) ⨝ users WHERE dept ∈ scope OR owner ∈ scope OR self → rows + department_id + roles + visibility
               ─GET /departments→ workspace: departments ∩ scope (+hidden flag for admins)
               ─GET /workspaces/:id/channels (private groups I am in), /channels/:id/members→ messaging (lazy)
chat/approval/docs/calendar ─displayName miss→ resolver batch ─POST /contacts/resolve→ auth: requireActiveMember(caller),
        tenant_users(any status) name+avatar
```

## Release train (two-release rule, security-plan runbook)
| Release | Ships | Works on |
|---|---|---|
| A | all code (phases 02–10): new workspaces/departments get directory nodes; scope/filters/rules/resolver; frontend | pre-064 data: a workspace with no `Directory_<ws>` has no rules, so everyone sees every member (true today); rule endpoints answer 503 "chưa sẵn sàng" there |
| B | `064_directory_visibility.sql` alone (graph-writing: nodes, edges, prohibition, peers, dept-row drift) | deploy recreates `policy` + `policy-read`, then `policy invalidate-cache --all-workspaces` |

Rollback B→A safe (A reads both shapes). Rolling back past A: run 064's reverse SQL first (phase 03).

## Acceptance (whole plan)
- Mockup sections 1, 1b, 1c, 2, 2b, 3, 4, 5, 6, 7 reproduced at 1280, 1024, 800, 390; §8 and "Bên ngoài"/"Danh bạ chia sẻ"
  not rendered; no "Tất cả thành viên" on org category home.
- Two same-named departments in different branches: tree, breadcrumbs and profile path place people correctly.
- Hidden department: absent from tree, search, counts, role lists and `GET /members` for non-members; admins see it labelled
  "Ẩn khỏi danh bạ"; its people still render by name/avatar in chat, approvals, docs, calendar; clicking shows
  "Hồ sơ này không còn hiển thị với bạn" with no details/actions.
- Limited role/department: Phòng ban reads "Chỉ phòng của bạn", search says "trong phạm vi bạn được xem"; owners
  ("Quản trị viên") always listed; people of other departments not.
- Nhắn tin, Gọi (enabled even offline), Gửi email work for real.
- PDP test vectors incl. DENY (other PC, one hop short, prohibition overrides ALLOW, cache freshness, non-member) green;
  workspace/auth tests green under `make test s=…` and `make test-realpolicy` (env below); vitest, lint, build green;
  `make check-ngac`, `make check-layering`, `make build-check` green; 064 applied twice with `make db-migrate` (second run
  0 rows) and observed; demo org seeded with graph nodes.
- Specs: `contacts-visibility` (new), `contacts-documents-settings-screens`, `admin-screens`, `tenant-ngac-init`,
  `workspace-admin-authorization`, `calendar` describe the shipped behaviour.

Test env (no credentials in files): `export PATH="/usr/local/go/bin:$HOME/go/bin:$PATH"`,
`TEST_DATABASE_URL=postgres://<user>:<pass>@localhost:5433/ngac_ci?sslmode=disable`, `REDIS_ADDR=localhost:6379`,
`TEST_REDIS_ADDR=localhost:6379`. Never the dev `ngac` DB for tests.

## Rollout
1. Release A = one PR from `feat/contacts-lark` (code + specs), approval-gated deploy (`docs/deployment.md`). Inside the deploy:
   workspace before auth before frontend (auth asks workspace for scope; until workspace answers, contacts return 503 and the
   UI shows "Không tải được danh bạ · Thử lại": fail closed, never fall back to tenant_users-only).
2. Prod dry-run of 064 in the migrate container (`docker compose --profile tools run --rm migrate psql …`), paste counts in the
   PR, set thresholds. Release B = PR carrying only `064_directory_visibility.sql`; deploy recreates policy + policy-read and
   runs `invalidate-cache`.
3. Default after rollout = today's behaviour (everyone sees everyone) because no rules exist; rules are opt-in in Quản trị.
4. Smoke on prod: owner, plain member, and a test limited role in a scratch org workspace.

## Rollback
- Frontend: revert; old UI ignores new fields; `/contacts` stays compatible (filters by name still accepted).
- Auth: revert to tenant_users listing — **re-exposes hidden people**; acceptable only as an emergency step, announce to
  admins who set rules.
- Workspace: revert leaves directory nodes/edges/prohibition; nothing else reads them; the prohibition affects only
  directory OAs. Re-deploy restores behaviour with rules intact (rules live in the graph).
- 064: reverse SQL from `_reconcile_064_*` (phase 03), then policy restart + `invalidate-cache`.
- PDP: reverting the CTE-fallback change re-opens a window where a limited person reads a directory OA (or a ghost user is unlimited) that memory has not loaded yet; revert it only together with the directory rules (it does nothing while the graph holds no prohibition).

## Unresolved questions (original Q1–Q9: answered, see "Quyết định" below)
New after the sync (Claude defaults applied, confirm or override):
1. Owner in a hidden department: visible to every member through `DirectoryOwners` (owners-always-visible wins over hiding).
2. "Nhóm của tôi" lists only `private` groups; `#general` (`workspace` type = everyone) is left out.
3. Calendar backend (`requirePerson`, free/busy, add guest by node id) stays scoped to workspace membership, not to directory
   visibility; pickers become visibility-scoped automatically (they read `usePeople().list`). OK?
4. Between release A and B, rule endpoints answer 503 in existing workspaces (no directory nodes yet). OK, or hide the
   Quản trị › Danh bạ nav entry until ready?

## Quyết định cho câu hỏi mở (Zane, 2026-10-17)
- Người bị giới hạn "chỉ thấy phòng mình" **luôn thấy Quản trị viên** (owners). Phase 02 (mô hình + test vector), 04 (`DirectoryScope` luôn gồm owners) và 05 (bộ lọc SQL) phải phản ánh; thêm test ALLOW cho limited→owner và DENY cho limited→người phòng khác.
- `/contacts/resolve` trả tên + avatar cho **mọi thành viên cùng workspace** (mọi trạng thái), không kiểm tra chung nội dung — đúng như phase 05.
- Mặc định (Claude chọn): limited nằm trong phòng bị ẩn vẫn thấy cây con của phòng mình (C1); "Quản trị viên" cho phạm vi hiển thị = mọi người giữ `manage` (C4); câu lỗi §5 đổi thành "Bạn vẫn tìm được người bằng ô tìm." (C5); Vai trò đọc từ danh bạ đã tải là chấp nhận được (C6); bấm tên người thấy được ở chat/phê duyệt/tài liệu mở thẻ hồ sơ Danh bạ (C7); email hiện cho người được xem (C8, đã chốt ở brainstorm); thành viên bị ẩn trong nhóm chung vẫn có trong gợi ý @mention (C9).
- Migration đổi sang `064_directory_visibility.sql`: `origin/main` đã có `057_calendar.sql`, `058_calendar_occurrence_answers.sql`; plan bảo mật dùng `059`–`063` sau red-team (062 chỉ mở rộng CHECK `removed`, 063 là reconcile tenant_users). Plan này cũng phải dựa trên `origin/main` (có service `calendar`): khi bắt đầu, đối chiếu lại các file được nêu với `origin/main` và xem lịch có cần lọc theo phạm vi hiển thị không (người dự lịch, chọn người). Kiểm tra thành viên dùng helper của plan bảo mật phase 07.

## Ghi chú dữ liệu demo (từ Phase 04 của plan bảo mật)
Các contact demo trên dev `a1000000-*` (Trần Minh Đức, Lê Quang Vinh, Nguyễn Thu Lan, Phạm Hải Yến) **không có ngac node** nên migration 063 đã đánh dấu `removed` (tombstone) trên dev. Khi dựng dữ liệu demo cho danh bạ, hãy seed lại với node đồ thị đúng (user node dưới `<ws>_Members`), không dùng người không có node. Bản sao lưu trước khi đổi nằm trong `_reconcile_063_tenant_users`.
→ Thực hiện bởi `scripts/seed-contacts-demo.sh` (phase 04): đi qua API thật (đăng nhập OTP chế độ mã cố định của dev,
mời + nhận lời mời, gán phòng/vai trò), nên mọi người có user node dưới `<ws>_Members`; không ghi SQL vào `ngac_*`, không
dùng lại các hàng `a1000000-*`.

## Sync with main (2026-10-17)
Plan re-verified against main `1dbbbe7` in worktree `nexus-enterprise-contacts`. What changed and why:

| # | Change | Why (evidence) |
|---|---|---|
| 1 | `blockedBy` emptied, `branch: feat/contacts-lark`, `base: main (1dbbbe7)`; "Why blocked" → "Foundation this plan builds on" | security fixes merged (#30–#32), search-suggestions `status: completed` (43a1509) |
| 2 | Membership everywhere = `policyclient` helper over the **writer**; scope call is graph-gated by workspace `authorizeMember` | `membership.go:81`, `authz.go:76-104` |
| 3 | Phase 05 no longer claims ListContacts already has a graph check; auth keeps the active-row check, workspace scope adds the graph check, resolver uses `requireActiveMember` | `users.go:114-120`; security choice 7 |
| 4 | Startup Go backfill **replaced** by migration `064_directory_visibility.sql` (release B) | policy refuses bare service identity for graph writes (`authpolicy.go:22-37`); security runbook pattern |
| 5 | Schema columns (`workspaces.directory_*`, `departments.dir_*`, `directory_hidden`) **dropped**; ids by name + cache; hidden from graph | two-release rule (security plan also refused a schema column for the Members-UA cache) |
| 6 | Owners-always-visible put into the model: `DirectoryOwners` OA + vectors (limited→owner ALLOW, limited→other dept DENY) | Zane decision 2026-10-17 (phase files had not been updated) |
| 7 | Workspace teardown must `RemoveProhibition` explicitly | `ngac_prohibitions` has no FK/cascade (`017_ngac_prohibitions.sql:7-15`); `deleteGraph` deletes nodes only (`workspace_delete.go:156`) |
| 8 | New service tests use the realpolicy harness (`*_realpolicy_test.go`, build tag), env above | security Phase 01 harness; `make test-realpolicy` |
| 9 | Group category = `channel_type === 'private'`; `#general` excluded | 060 retyped groups to `private`, `#general` is `workspace` |
| 10 | Phase 06 realtime step removed | `frontend/src/lib/realtime.ts:122-123` already invalidates `keys.contacts.every()` on every workspace event (incl. `department_changed`, `role_changed`) |
| 11 | Phase 09 MentionDropdown edit removed (verify only); calendar callsites added | `MentionDropdown.tsx:21` already lists channel members; calendar pickers read `usePeople().list` (`GuestsPanel.tsx:74`, `AddCalendarPicker.tsx:28`) |
| 12 | Demo data re-seed script (phase 04) | dev `a1000000-*` tombstoned by 063 (no graph node) |
| 13 | Stale `file:line` fixed: `DeptUAName` `ngac_ops.go:185`; `requireRole` `authz.go:154`; `detachMember` `members.go:197`; `CreateWorkspace` `service.go:159-260`; `MoveDepartment` edge swap `department.go:246-268`; `applyDepartment` row write `:456-461`; nginx contacts `deploy/nginx/default.conf:116`; DESIGN.md "Gợi ý trong ô tìm" `:795`; `actingAs` `auth/internal/domain/provisioning.go:28`; `people.ts` `displayName :60`, `avatarOf :68`; `colleaguesOf` `contacts-model.ts:54` | re-grep on main |
| 14 | Spec list gains `calendar` (hidden guest rendering) | calendar service on main |

Consistency sweep (2026-10-17, `grep -rn` over plan.md + phase-01..11):

| Search | Result |
|---|---|
| `061_` / `063_` / `062_` as this plan's migration | none; only as the security plan's files ("Foundation", "Quyết định", demo note) and as patterns cited by phase 03 |
| `064` | one migration, `064_directory_visibility.sql`, owned by phase 03; referenced by plan.md, 03, 11 |
| `directory_oa_id`, `dir_oa_id`, `directory_hidden` columns | none (only "dropped" in this table) |
| `BackfillDirectories` / start-up backfill | none (only "replaced" in this table) |
| tenant_users-only membership (`status='active'` as the whole check) | only as auth's cheap early check, paired with the workspace graph check |
| `TenantMember` as a dependency | none |
| `blockedBy` search-suggestions / "Land 06 first" / "rebase" on 1516 | none |
| `after F2 fix` / "F7 matches the graph" claims | none |
| `websocket.store.ts` realtime edit | none |
| `:5433` without `ngac_ci` | none; test DB is `ngac_ci` |
| `channel_type group` | none; `private` |
| `plan Q<n>` references | Q1–Q4 = the new questions above (phases 01, 02, 06, 07, 09, 10); old Q5–Q9 cited as C5–C9 decisions |
| `start-up backfill` / `cmd/main.go` hook | only as the rejected option (plan decision 6, phase 03 overview) |
| stale `file:line` (`authz.go:131`, `members.go:117`, `ngac_ops.go:159`, `default.conf:111`, `tenants.go:73`, DESIGN.md `~705`) | none left |

## Quyết định sau đồng bộ (Claude chọn theo uỷ quyền của Zane, 2026-10-17)
- Chấp nhận thay start-up backfill bằng migration `064_directory_visibility.sql` (release A code, release B migration) và bỏ các cột schema mới.
- Q1: owner nằm trong phòng bị ẩn vẫn hiện với mọi thành viên ("luôn thấy owners" thắng ẩn).
- Q2: "Nhóm của tôi" không liệt kê `#general`.
- Q3: kiểm tra phía server của lịch (free/busy, thêm khách theo id) giữ theo tư cách thành viên workspace; chỉ bộ chọn người lọc theo phạm vi hiển thị.
- Q4: giữa release A và B, ẩn mục Quản trị › Danh bạ (quy tắc phạm vi) ở workspace chưa có node danh bạ, thay vì trả 503.
- Bước "Zane duyệt thiết kế" ở phase 01: Claude tự chốt theo mockup đã duyệt `design/mockups/contacts-lark.html`, ghi lại trong phase file để Zane xem sau.
- Sau Phase 01: Danh bạ giữ chữ "Quản trị viên" cho nhóm owners (quyết định của Zane), màn Quản trị giữ "Chủ sở hữu"; các thao tác ẩn/giới hạn phòng giữ hộp xác nhận; chữ "Tất cả thành viên" còn sót trong mockup sửa ở phase frontend.
