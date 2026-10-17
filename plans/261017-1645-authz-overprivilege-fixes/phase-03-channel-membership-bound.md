---
phase: 3
title: "Channel, workspace and calendar membership bound to <W>_Members (F2), migration 061"
status: completed (code + 061 applied on dev and ngac_ci; prod dry-run and release order pending)
priority: P1
effort: "5h"
dependencies: [1, 2]
---
# Phase 3: Membership bound to the workspace (F2)

## Overview
`AddMember` (`messaging/internal/domain/members.go:25-71`) authorizes only `invite` on the channel content OA and
assigns any client-sent node under the channel Members UA, which sits under the workspace PC (`channels.go:162`).
"Workspace member" is "PC among ancestors" in workspace (`authz.go:75-95`, `member_directory.go:150-161`,
`service.go:256-297`, `members.go:153-175` ListMembers) **and in calendar** (origin/main
`calendar/internal/domain/membership.go:50-62`, `inWorkspace`), where event-guest UAs are also assigned under the PC
(`attendees.go:153`). So a channel member can make anyone (or a whole UA) a workspace member, undo a removal, and the
same class reaches calendar.

Fixes (both): `AddMember` checks its target; every "is member" test becomes `policyclient.IsWorkspaceMember`
(under `<W>_Members`). Channel and event-guest UAs stay under the PC (needed for the intersection rule).

## Requirements
- `AddMember`: (1) existing `invite` check — any group member may add (Q2); (2) DM guard; (3) `GetNode(target)` type
  `U` else `ErrInvalidInput`; (4) `IsWorkspaceMember(target, ch.WorkspaceID)` else `ErrAccessDenied`; policy error →
  deny. Shared by gRPC `AddChannelMember`.
- Workspace service: `authorizeMember` (`authz.go:75`), `reachesPC` → `isMember` (`member_directory.go:150`, callers
  `requireInPC :140`, `admit invitations.go:424`), `ListMembers` (`members.go:153-175`: people = U descendants of
  `<W>_Members`), `ListMemberDirectory` people set (`member_directory.go:166-171`), `ListAccessibleWorkspaces`
  (`service.go:256`: `policyclient.MemberWorkspaces(user)` → intersect with `store.ListAll()` ids; cost bounded by the
  user's workspaces via the mandatory Members-UA cache). `detachMember` keeps stripping every UA under the PC.
- Calendar: `inWorkspace` (`calendar/internal/domain/membership.go:50-62`) → `IsWorkspaceMember`. `requirePerson`
  stays (row + graph). Event-guest UA assignment under the PC unchanged; guests are already required to be members.
- **061 `data/migrations/061_channel_membership_reconcile.sql`** (release B after this phase's code):
  1. Guard: `RAISE EXCEPTION` if any workspace lacks `<id>_Members`.
  2. Backup to `_reconcile_061_assignments` and `_reconcile_061_channel_members`.
  3. Delete `child → Ch_*_Members` for channels of W where child is a U not reaching `<W>_Members`, or any UA.
  4. Delete `channel_members` rows with no matching graph assignment (dev 6) and rows for deleted edges.
  5. Threshold guard: `RAISE EXCEPTION` if step 3 would delete more than N rows (default 50; set from prod dry-run).
  Re-runnable (fixpoint). Header NOTE: recreate policy + policy-read, invalidate, restart messaging.

## Architecture
```
AddMember: invite on Ch content ─> target is U ─> IsWorkspaceMember(target, W) ─> assign ─> cache row
member? (workspace, calendar, messaging): <W>_Members id ∈ ancestors(user)
```

## Related Code Files
- Modify: `messaging/internal/domain/members.go` (AddMember; stale comment `:19-24`),
  `workspace/internal/domain/authz.go`, `member_directory.go`, `members.go` (ListMembers), `service.go`
  (ListAccessibleWorkspaces), `invitations.go` (rename call), `calendar/internal/domain/membership.go`
- Create: `data/migrations/061_channel_membership_reconcile.sql`,
  `messaging/internal/domain/add_member_realpolicy_test.go`, `workspace/internal/domain/membership_realpolicy_test.go`,
  `calendar/internal/domain/membership_realpolicy_test.go` (tag `realpolicy`); vectors appended to
  `policy/internal/ngac/pdp_private_groups_test.go`
- Spec: `docs/specs/tenant-ngac-init/spec.md` (channel membership target rules), `docs/specs/workspace-admin-authorization/spec.md`
  (membership = Members UA), `docs/specs/calendar/spec.md` (calendar membership = Members UA)

## Implementation Steps
1. Deny tests first, paired allows on the same fixture: AddMember target from another tenant DENY / active member
   ALLOW; target = another workspace's Members UA `ErrInvalidInput`; target only in a channel UA of W DENY; target
   detached from W DENY (removal reversal). Workspace: user reaching PC_W only via `Ch_x_Members` or a `CalEv_*_Guests`
   UA → `GetWorkspace`, `ListMembers` (absent), `ListDepartments` DENY; `ListAccessibleWorkspaces` omits W; owner and
   TenantMember allowed. Calendar: channel-only user denied workspace calendar actions; member allowed.
2. Implement.
3. 061 with dry-run block; `make db-migrate` twice; recreate policy services; `make policy-invalidate`.
4. Verify: users reaching a workspace PC without reaching `<W>_Members` = 0, counting paths through `Ch_*_Members` and
   `CalEv_*_Guests` explicitly (report each separately).
5. `make test s=messaging|workspace|calendar|policy`; `make test-realpolicy`; `make check-ngac`, `make check-layering`.

## Success Criteria
- Every F2 exploit (invite bypass, removal reversal, whole-tenant injection) and the calendar variant has a
  failing-then-passing test. Step 4 query = 0 on dev.

## Risk Assessment
| Risk | L×I | Mitigation |
|------|-----|------------|
| Legit people only via role/dept UA lose membership | L×H | dev: none; prod dry-run lists them |
| Deploy window with old code after 061 | M×M | 061 only deletes edges old code also treats as illegit once Phase 03 code runs; release A first |
| 061 deletes real data | L×M | backup tables + threshold guard + prod dry-run |
| Members-UA cache miss cost in ListAccessibleWorkspaces | L×L | bounded by the user's workspaces |

## Rollback
Code: revert (release A). Data: `INSERT INTO ngac_assignments SELECT * FROM _reconcile_061_assignments ON CONFLICT DO
NOTHING;` and same for `channel_members`; recreate policy services + invalidate.

## Added by the Phase 02 review (2026-10-17)

Migration 061 also strips the `#general` direct edges (`U -> Ch_<general>_Members`) and the `channel_members` cache
rows of anyone who is not a real workspace member, i.e. whose user node does not reach `<ws>_Members`. Phase 02's 060 and
`JoinWorkspaceChannel` only ever write the edge for a member, but an edge written while someone was a member survives if
they were detached without the consumer's cache cleanup running; 061 is the reconcile that removes them. Same rules as
the rest of 061: backup into `_reconcile_061_*` first, guard and threshold, re-runnable (second run 0 rows). Add its
deny/allow pair to Phase 03's real-policy tests: a detached person with a leftover `#general` edge is removed and denied;
a member keeps theirs. The workspace-wide channel is identified by `channel_type = 'workspace'` alone once
`channels_one_workspace_wide` exists (Phase 02 review), so 061 must not use the name heuristic.

## Implementation status (2026-10-17)

- [x] AddMember: invite, DM guard, target is a `U` node (else `ErrInvalidInput`), `IsWorkspaceMember` (else `ErrAccessDenied`; policy error denies); shared with gRPC `AddChannelMember`
- [x] Workspace: `authorizeMember`, `isMember` (replaces `reachesPC`; used by `requireMemberUser` and `admit`), `ListMembers`, `ListMemberDirectory`, `ListAccessibleWorkspaces` via the Members UA with a mandatory process-wide id cache
- [x] Calendar: `inWorkspace` via `policyclient.IsWorkspaceMember` (so `requireMember` and `requirePerson`)
- [x] Migration 061 (`data/migrations/061_channel_membership_reconcile.sql`), `scripts/test-migration-061.sh`
- [x] Real-policy tests (messaging, workspace, calendar) and a PDP vector; specs updated
- [ ] Prod: run the pre-flight in the migrate container, set the 061 thresholds from it, release A (code) before B (061)

Code review fixes (2026-10-17):
- [x] M1: `PolicyWriteService` gained `FindNodeByName` and `IsAssigned` (proto + `make proto`; the TS `proto:gen` covers only `messaging/ws.proto`, no frontend diff); messaging `AddMember` (target check and the `existed` probe) and calendar named-person checks (`requirePerson`, group expansion) ask the writer, so a removal is seen at once
- [x] M2: workspace uses `policyclient.IsWorkspaceMember`, `MembersUA` and `MemberWorkspaces` over `policyWrite`; helper comment fixed; runbook line in `docs/deployment.md`
- [x] L2: `AddMember` refuses and writes nothing when `GetNode` or `GetAncestors` fails
- [x] L3: 061 guard and `_m061_below` require the Members UA to sit under `PC_<id>`
- [x] L4: reverse SQL filters on existing nodes and channels
- [x] L6: `testutil.CreateWorkspace` builds PC_<id>, Members, Owners, Channels and their edges

061 skips DM channels (Phase 05 owns the DM rule).
