---
title: "Danh bạ kiểu Lark — đợt 1"
description: "Category-first Danh bạ (Phòng ban, Vai trò, Nhóm của tôi), NGAC-decided contacts visibility with admin rules, and a name resolver separate from the directory."
status: pending
priority: P2
effort: 63h
branch: feat/search-suggestions (cut feat/contacts-lark from main when unblocked)
tags: [contacts, ngac, frontend, workspace, auth, design-first]
created: 2026-10-17
blockedBy: [261017-1645-authz-overprivilege-fixes, 261017-1516-search-suggestions]
blocks: []
brainstorm: ../reports/brainstorm-261017-1542-contacts-lark-redesign.md
mockup: design/mockups/contacts-lark.html
specs:
  new: [contacts-visibility]
  modified: [contacts-documents-settings-screens, admin-screens, tenant-ngac-init, workspace-admin-authorization]
---

# Danh bạ kiểu Lark — đợt 1

Inputs (authoritative): brainstorm above (every bullet), mockup `design/mockups/contacts-lark.html` §1–§8 (approved
Zane 2026-10-17), security audit `plans/reports/security-261017-1630-authz-overprivilege-audit.md`.

## Why blocked
- **261017-1645-authz-overprivilege-fixes** (F1 groups private, F2 AddMember checks workspace membership + membership =
  "under Members UA", F3/F5 removal = `status='removed'` + session cut, F4 DM only within a shared workspace, F7
  tenant_users reconciled with NGAC). This plan assumes all of them; it does not re-fix any. Concretely relied on:
  F1 (`ListChannels` = my groups only), F5 tombstones (name resolver serves removed people), F7 (`tenant_users.department_id`
  matches the graph), F2 (dept-adjacent UAs never imply membership).
- **261017-1516-search-suggestions**: phases 01–05 are merged; phase 06 still edits
  `docs/specs/contacts-documents-settings-screens/spec.md`. This plan rewrites `ContactsScreen.tsx` and that spec, and must
  keep the suggestion panel (`lib/suggestions/contacts.ts`, `SearchSuggestions`). Land 06 first, then rebase. Its
  frontmatter now has `blocks: [261017-1646-contacts-lark-redesign]`.

## Key decisions (made here; details in phase files)
1. **Visibility graph model** (phase 02): per-department directory OAs under a workspace `Directory` OA placed under the
   Mgmt OA; open subtree read via `Members UA → DirectoryOpen {read}`; hidden department = its Dir OA moved out of
   DirectoryOpen + `Dept UA → Dir OA {read}`; "own department only" = one static **prohibition** (subject
   `DirectoryLimited` UA, read, target DirectoryOpen) + a parallel peer-OA tree reached through a flat per-department
   `DeptPeers` UA; directory admins = `manage` on the Directory OA (inherited from Mgmt). No PDP code change, no new
   operation. Rejected: per-user derived "DirectoryReaders" assignment (8+ reconcile sites, a missed one silently
   widens access), PDP complement support (still needs per-user state).
2. **Workspace owns the scope, auth owns the people rows**: workspace computes `DirectoryScope` (NGAC checks) and serves it
   to auth over gRPC; auth filters `tenant_users` by it. One implementation of the rules (DRY).
3. **Browse vs resolve split**: `GET /contacts` = visibility-filtered directory (+ `department_id`, `roles`, `visibility`);
   `POST /contacts/resolve` = name + avatar for IDs the client met in content, any member status. Same `/contacts`
   prefix → no new vite/nginx/Traefik route entry.
4. **Roles ride the contacts payload** (brief item 2); Vai trò category groups loaded contacts client-side, so it shows only
   roles with people the caller may see, with no extra endpoint. Departments and groups load on open (own API).

## Phases

| # | Phase | Pri | Effort | Depends | Spec |
|---|---|---|---|---|---|
| 01 | [Design + UI specs](phase-01-design-and-specs.md) | P1 | 4h | — | contacts-documents-settings-screens, admin-screens |
| 02 | [NGAC visibility model + test vectors](phase-02-ngac-visibility-model.md) | P1 | 4h | 01 | contacts-visibility (new), tenant-ngac-init |
| 03 | [Workspace: directory nodes, upkeep, backfill](phase-03-workspace-directory-graph.md) | P1 | 8h | 02 | tenant-ngac-init, contacts-visibility |
| 04 | [Workspace: scope, filtered reads, admin rules API](phase-04-workspace-scope-api.md) | P1 | 6h | 03 | contacts-visibility, workspace-admin-authorization |
| 05 | [Auth: NGAC-decided contacts + resolver](phase-05-auth-contacts.md) | P1 | 5h | 04 | contacts-visibility |
| 06 | [Frontend data layer + PersonLink/HiddenPersonCard primitives](phase-06-frontend-data.md) | P1 | 6h | 05 | — |
| 07 | [Danh bạ desktop](phase-07-contacts-desktop.md) | P2 | 10h | 06 | contacts-documents-settings-screens |
| 08 | [Danh bạ tablet + phone](phase-08-contacts-tablet-phone.md) | P2 | 6h | 07 | contacts-documents-settings-screens |
| 09 | [Hidden person card across the app](phase-09-hidden-person-card.md) | P2 | 5h | 06 | contacts-visibility |
| 10 | [Quản trị › Danh bạ](phase-10-admin-directory-scope.md) | P2 | 5h | 04, 06 | admin-screens |
| 11 | [Verify, rollout, spec close-out](phase-11-verify-rollout.md) | P1 | 4h | 07–10 | all above |

Parallel after 06: {07→08}, 09, 10 — disjoint file ownership (07/08 `components/contacts/*` + `routes/_workspace/contacts.tsx`;
09 chat/approval/drive callsites only (primitives owned by 06); 10 `components/admin/DirectoryScope*`, `routes/_workspace/admin/directory.tsx`, `AdminFrame.tsx`).

## Data flow (end state)
```
admin (Quản trị) ─PUT rule→ workspace domain ─policyWrite→ policy (PAP→EPP invalidation)
member opens Danh bạ ─GET /contacts→ auth ─gRPC GetDirectoryScope(caller)→ workspace ─BatchCheck(read Dir/Peer OAs, manage root)→ policy PDP
                     auth: SQL tenant_users ⨝ users WHERE dept ∈ scope → rows + department_id + roles(visible) + visibility
               ─GET /departments→ workspace: departments ∩ scope (+hidden flag for admins)
               ─GET /workspaces/:id/channels, /channels/:id/members→ messaging (my groups, lazy)
chat/approval/docs ─displayName miss→ resolver batch ─POST /contacts/resolve→ auth: tenant_users(any status) name+avatar
```

## Acceptance (whole plan)
- Mockup sections 1, 1b, 1c, 2, 2b, 3, 4, 5, 6, 7 reproduced at 1280, 1024, 800, 390; §8 and "Bên ngoài"/"Danh bạ chia sẻ"
  not rendered; no "Tất cả thành viên" on org category home.
- Two same-named departments in different branches: tree, breadcrumbs and profile path place people correctly.
- Hidden department: absent from tree, search, counts, role lists and `GET /members` for non-members; admins see it labelled
  "Ẩn khỏi danh bạ"; its people still render by name/avatar in chat, approvals, docs; clicking shows
  "Hồ sơ này không còn hiển thị với bạn" with no details/actions.
- Limited role/department: Phòng ban reads "Chỉ phòng của bạn", search says "trong phạm vi bạn được xem".
- Nhắn tin, Gọi (enabled even offline), Gửi email work for real.
- PDP test vectors incl. DENY (other PC, one hop short, prohibition overrides ALLOW, cache freshness) green; Go tests
  (TEST_DATABASE_URL on :5433) for workspace + auth green; vitest, lint, build green; `make check-ngac`,
  `make check-layering`, `make build-check` green; migration applied with `make db-migrate` and observed.
- Specs: `contacts-visibility` (new), `contacts-documents-settings-screens`, `admin-screens`, `tenant-ngac-init`,
  `workspace-admin-authorization` describe the shipped behaviour.

## Rollout
1. Merge after both blockers land. One PR, deploy via the normal approval-gated pipeline (`docs/deployment.md`).
2. Order inside the deploy: migration (nullable columns) → workspace (synchronous idempotent directory backfill **before**
   gRPC/REST listen) → auth → frontend. Auth asks workspace for scope; until workspace answers, contacts return 503 and the
   UI shows "Không tải được danh bạ · Thử lại" (fail closed, never fall back to tenant_users-only).
3. Default after rollout = today's behaviour (everyone sees everyone) because no rules exist; rules are opt-in in Quản trị.
4. Smoke on prod: owner, plain member, and a test limited role in a scratch org workspace.

## Rollback
- Frontend: revert; old UI ignores new fields; `/contacts` stays compatible (filters by name still accepted).
- Auth: revert to tenant_users listing — **re-exposes hidden people**; acceptable only as an emergency step, announce to
  admins who set rules.
- Workspace: revert leaves directory nodes/edges and nullable columns; nothing else reads them; prohibition affects only
  directory OAs. Re-deploy restores behaviour with rules intact (rules live in the graph).
- No PDP code change to roll back. Optional cleanup: `DeleteNode` on `Directory_*` nodes per workspace (script in phase 03).

## Unresolved questions
1. Limited user inside a hidden department sees that hidden department's subtree (room membership wins). OK?
2. Limited user sees nobody outside their department subtree, incl. owners and people with no department; with no
   department they see only themselves. Should "Quản trị viên" always be visible to everyone?
3. Resolver scope: any member of the workspace (any status) by ID, vs stricter "shares a channel/approval/doc".
4. Directory admin = anyone holding `manage` (Owners + custom roles with Quản lý). Confirm.
5. §5 partial-tree-failure copy says "Bạn vẫn xem được Tất cả thành viên", which no longer exists; proposed
   "Bạn vẫn tìm được người bằng ô tìm." (phase 01 fixes the mockup).
6. Vai trò category reads loaded contacts (no own API call) — deviation from "mỗi danh mục gọi API riêng". OK?
7. Clicking a visible person's name in chat/approvals/docs: open their Danh bạ profile (new behaviour) or do nothing?
8. Email stays visible to everyone who can see the person (F7 note "treat email as scope-controlled"). OK?
9. Hidden members of a group the caller shares: still offered in @mention ("trả lời vẫn được") or not? Phase 09 verifies
   the current mention source; default = offer channel members incl. hidden ones by resolved name.

## Quyết định cho câu hỏi mở (Zane, 2026-10-17)
- Người bị giới hạn "chỉ thấy phòng mình" **luôn thấy Quản trị viên** (owners). Phase 02 (mô hình + test vector), 04 (`DirectoryScope` luôn gồm owners) và 05 (bộ lọc SQL) phải phản ánh; thêm test ALLOW cho limited→owner và DENY cho limited→người phòng khác.
- `/contacts/resolve` trả tên + avatar cho **mọi thành viên cùng workspace** (mọi trạng thái), không kiểm tra chung nội dung — đúng như phase 05.
- Mặc định (Claude chọn): limited nằm trong phòng bị ẩn vẫn thấy cây con của phòng mình (C1); "Quản trị viên" cho phạm vi hiển thị = mọi người giữ `manage` (C4); câu lỗi §5 đổi thành "Bạn vẫn tìm được người bằng ô tìm." (C5); Vai trò đọc từ danh bạ đã tải là chấp nhận được (C6); bấm tên người thấy được ở chat/phê duyệt/tài liệu mở thẻ hồ sơ Danh bạ (C7); email hiện cho người được xem (C8, đã chốt ở brainstorm); thành viên bị ẩn trong nhóm chung vẫn có trong gợi ý @mention (C9).
- Migration đổi sang `064_directory_visibility.sql`: `origin/main` đã có `057_calendar.sql`, `058_calendar_occurrence_answers.sql`; plan bảo mật dùng `059`–`063` sau red-team (062 chỉ mở rộng CHECK `removed`, 063 là reconcile tenant_users). Plan này cũng phải dựa trên `origin/main` (có service `calendar`): khi bắt đầu, đối chiếu lại các file được nêu với `origin/main` và xem lịch có cần lọc theo phạm vi hiển thị không (người dự lịch, chọn người). Kiểm tra thành viên dùng helper của plan bảo mật phase 07.

## Ghi chú dữ liệu demo (từ Phase 04 của plan bảo mật)
Các contact demo trên dev `a1000000-*` (Trần Minh Đức, Lê Quang Vinh, Nguyễn Thu Lan, Phạm Hải Yến) **không có ngac node** nên migration 063 đã đánh dấu `removed` (tombstone) trên dev. Khi dựng dữ liệu demo cho danh bạ, hãy seed lại với node đồ thị đúng (user node dưới `<ws>_Members`), không dùng người không có node. Bản sao lưu trước khi đổi nằm trong `_reconcile_063_tenant_users`.
