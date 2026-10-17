---
phase: 6
title: "Workspace admin guards (F6, F10)"
status: completed
priority: P2
effort: "2h"
dependencies: [4]
---
# Phase 6: Workspace admin guards (F6, F10)

## Overview
- F6: `RemoveMember` (`workspace/internal/domain/members.go:72-115`) needs only `invite` on Mgmt; it escalates only when
  the target is an Owner (`:82-95`). `manage` is grantable per area (`ngac/areas.go:52-60`), so an invite-only role can
  remove an Assets/Documents/Channels manager or a non-owner Mgmt admin.
- F10: `DeleteFolder` (`workspace/internal/domain/folders.go:81-94`) accepts any OA under the PC
  (`requireInWorkspace(..., TypeOA)`): Mgmt, Channels, Documents, channel content OAs, asset OAs. gRPC-only.

## Requirements
- F6 (decision 4 + choice 2): inside the owner-lock closure after the owner branch: if the target holds `manage` on
  **any** area OA of W (Mgmt, Documents, Channels, Assets — resolved via `ngac.AreaOAName`; one `BatchCheckAccess` for
  the target), the caller must hold `manage` on `<W>_Mgmt`, else `ErrAccessDenied`. Policy error → deny. Owner rule
  unchanged. Comment `areas.go:40` ("invite (add and remove members)") states the limit.
- F10: `DeleteFolder` accepts only a folder OA: `ngac.IsFolderNodeName(name)` (new, beside `FolderNodeName`
  `ngac_ops.go:280-282`) **and** `properties.workspace_id == wsID` (`folders.go:43`). Else `ErrInvalidInput`. Reuse the
  node `nodeInWorkspace` returns (`authz.go:106`).

## Architecture
```
RemoveMember: invite(Mgmt) ─> [owner? owner+manage+last-owner] ─> [target manage on any area? caller manage(Mgmt)] ─> detach
DeleteFolder: manage(Mgmt) ─> node in PC, OA ─> IsFolderNodeName ∧ workspace_id ─> DeleteNode (EPP)
```

## Related Code Files
- Modify: `workspace/internal/domain/members.go` (RemoveMember authorization only; Phase 04 owns the tail),
  `folders.go`, `backend/ngac/ngac_ops.go`, `backend/ngac/areas.go` (comment)
- Create: `workspace/internal/domain/admin_guards_realpolicy_test.go` (tag), `backend/ngac/ngac_ops_test.go` (extend)
- Spec: `docs/specs/workspace-admin-authorization/spec.md`

## Implementation Steps
1. Deny tests first, paired allows: invite-only removes Mgmt-manage holder, Assets-manage holder, Channels-manage holder
   → DENY (target still attached); invite-only removes a plain member → ALLOW; Mgmt-manage caller removes each → ALLOW;
   policy error → DENY. DeleteFolder on Mgmt / Channels / Documents / channel content / asset OA / other workspace's
   folder → refused, node present; real folder → deleted.
2. Implement. 3. `make test s=workspace`; `make test-realpolicy`; `make check-ngac`.

## Success Criteria
- Tests green; folder predicate test covers `Folder_<id>` vs `<ws>_Assets`, `<ws>_Type_*`, `<ws>_Channels`.

## Risk Assessment
| Risk | L×I | Mitigation |
|------|-----|------------|
| HR roles relied on removing managers | L×M | decision; spec documents it |
| Legacy folders not named `Folder_` | L×L | dev 2026-10-17: 91 `Folder_*`; only other OAs with `workspace_id` are 6 asset OAs (also deletable today → the name check closes it). Re-run on prod. |

## Rollback
Code-only; revert.

## Status (implemented, not committed)
- [x] Deny tests first, each paired with an allow on the same fixture (`admin_guards_realpolicy_test.go`; fake-based `admin_guards_test.go`; `TestIsFolderNodeName`), seen red before the change.
- [x] F6: `requireManageToRemoveManager` in `RemoveMember` (one `BatchCheckAccess` on the target over the area OAs; any error denies).
- [x] F10: `DeleteFolder` accepts `ngac.IsFolderNodeName` + `workspace_id == wsID`, else `ErrInvalidInput`; `ngac.IsFolderNodeName` added.
- [x] `areas.go` comment; `workspace-admin-authorization` spec updated.
- Deviation: the unit fixture folder (`authz_test.go`) is now a real folder (`Folder_<id>` with `workspace_id`), and the fake read client gained `BatchCheckAccess`.

### Review fixes (same worktree, uncommitted)
- [x] M1: admin folders carry `type: admin_folder` (`ngac.AdminFolderProps`, set by `CreateFolder`); `DeleteFolder` and `ListFolders` use `ngac.IsAdminFolder` (name + `workspace_id` + type `admin_folder` or none). Dev check: Folder_ nodes are 78 without a type (admin folders made earlier) and 13 `drive_folder`, so untyped ones stay deletable and the drive's are refused. Real-policy refusals for a Documents drive folder and a group's drive folder, paired with the admin-folder allow.
- [x] L1: spec line that the rule covers area-level manage only.
- [x] L2: `requireManageToRemoveManager` reuses `areaOAs` (no extra `GetChildren`); a policy that cannot answer is `ErrUnavailable` (503 / gRPC Unavailable, new shared sentinel in `httputil`/`grpcutil`), still fail-closed.
