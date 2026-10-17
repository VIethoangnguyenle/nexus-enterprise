---
phase: 2
title: "NGAC visibility model + test vectors"
status: pending
priority: P1
effort: "5h"
dependencies: [1]
---

# Phase 2: NGAC visibility model + test vectors

## Overview
Policy-model change (skill `ngac-policy-change`). Defines the graph shape for contacts visibility, the name helpers, and
PDP test vectors with DENY cases first. No PDP code change: the model uses associations and the existing prohibition
deny-override (`backend/services/policy/internal/ngac/pdp_decision_engine.go:66` `Decide` → `:150` `checkProhibitions`
(prohibitions from the global graph, `:160`) → `:187` `matchProhibitions`; batch path `pdp_decision_batch.go:87`
`applyProhibitionsBatch`). Prohibition writes already take the EPP path (`policy/internal/grpc/write_server.go:350-369`,
`MutationCreateProhibition`). Spec + vectors + helpers in **one commit** (CLAUDE.md §5).

## Requirements
Membership precondition (PEP, not graph): the caller is a workspace member = `U` under `<ws>_Members`
(`policyclient.IsWorkspaceMember`, `backend/pkg/policyclient/membership.go:81`). Workspace `authorizeMember`
(`authz.go:76`) runs it before any directory decision. No directory node is placed under `<ws>_Members`, so no directory
edge can make anyone a member (vector below).

Decision "may caller C see person P in the directory" =
- **admin**: `Check(C, Directory_<ws>, manage)` ALLOW → sees everyone, hidden labelled; else
- `read` ALLOW on `Dir_<P.dept>` (or `DirectoryNoDept_<ws>` when P has no department) **or** on `DirPeer_<P.dept>`;
- **or** P holds `<ws>_Owners` and `read` ALLOW on `DirectoryOwners_<ws>` (owners always visible — Zane 2026-10-17);
- C always sees themself (PEP rule, not graph).

Rules expressed:
- Default: every member sees all non-hidden departments, people without a department, and the owners.
- Hide department D: only people in D's subtree + admins see D's subtree (owners stay visible to all, plan Q1).
- Limit role R or department X: holders of R / people in X's subtree see only their own department's subtree + owners.

## Architecture
Graph per workspace W (new nodes; names from `backend/ngac`):
```
PC_W
├─ W_Members (UA, existing) — users, and W_Owners under it (membership = under here; unchanged)
├─ W_Mgmt (OA, existing) ── Owners {AllOwnerOps}, custom mgmt roles {manage, invite} (ngac/areas.go:55-56)
│   └─ Directory_W (OA)                         admins: manage inherited from Mgmt → admin shortcut
│       ├─ DirectoryOpen_W (OA)  ◄── W_Members {read}          (default visibility)
│       │    ├─ DirectoryNoDept_W (OA)                          people with no department
│       │    └─ Dir_<d> (OA) … mirrors department tree         (non-hidden top-level depts)
│       ├─ DirectoryOwners_W (OA) ◄── W_Members {read}         owners always visible; prohibition never reaches it
│       ├─ Dir_<h> (OA)          ◄── Dept_<h> {read}            hidden dept h: moved out of Open (subtree follows)
│       └─ DirPeer_<d> (OA) … mirrors department tree (top-level under Directory_W)
├─ DirectoryLimited_W (UA, under PC, NOT under Members) ◄─ role UA R / Dept_<x> UA assigned here (extra parent) = "limited"
│     prohibition DirectoryLimited_W: subject DirectoryLimited_W, ops [read], targets [DirectoryOpen_W], intersection=false
└─ DeptPeers_<d> (UA, flat, directly under PC_W, NOT under Members) ◄─ users directly in d (mirror of U→Dept_<d>)
      association DeptPeers_<d> → DirPeer_<d> {read}
```
Why each piece:
- Peer tree + flat `DeptPeers` UA: `Dept_<d>` UAs chain upward (`department.go:87-95`: sub-dept UA under parent dept UA,
  top-level under PC), so any association on a parent dept UA would reach every sub-department's people; a flat UA reaches
  only direct members, and the DirPeer tree gives them their own subtree. The prohibition cannot touch DirPeer OAs.
- `DirectoryLimited_W` as an extra parent of R/Dept_x: limitedness follows role assignment and department moves with no
  per-user upkeep; it carries no association, so it grants nothing.
- `DirectoryOwners_W` outside Open: limited users keep reading it (the prohibition targets Open only); hiding a department
  does not hide an owner placed in it.
- Directory under Mgmt: Owners read everything (AllOwnerOps on Mgmt), any `manage` holder gets the admin shortcut,
  role grants need no new code. An owner who is also in a limited department stays admin (shortcut is `manage`, the
  prohibition is `read` only).
- Precedence (documented, C1): a limited user inside hidden D still sees D's subtree (Dir_h is not under Open).
- Existing classifiers ignore the new nodes: `classifyRole` (`roles_admin.go:56`) accepts only `type=role`, Owners, Members;
  `IsAdminFolder` (`ngac_ops.go:328`) needs a `Folder_` name; `grantsByArea` (`permission_areas.go:78`) maps area OAs only.

Helpers to add in `backend/ngac/ngac_ops.go` (ID-typed, next to `DeptUAName` at :185 and the workspace names at :159-166):
`DirectoryOAName(WorkspaceID)`, `DirectoryOpenOAName(WorkspaceID)`, `DirectoryNoDeptOAName(WorkspaceID)`,
`DirectoryOwnersOAName(WorkspaceID)`, `DirectoryLimitedUAName(WorkspaceID)`, `DirectoryLimitedProhibitionName(WorkspaceID)`,
`DeptDirOAName(DeptID)`, `DeptPeerOAName(DeptID)`, `DeptPeersUAName(DeptID)`; property `PropTypeDirectory = "directory"`
(next to `PropType*` at :236-238) on every node above, plus `ngac.DirectoryProps(ws)`.

## Related Code Files
- Create: `backend/services/policy/internal/ngac/pdp_contacts_visibility_test.go` (model on `pdp_vnpay_scenarios_test.go`
  and the prohibition vector in `pdp_private_groups_test.go:166`; single + batch path)
- Create: `docs/specs/contacts-visibility/spec.md` (purpose, graph, decision, rules, precedence, scenarios incl. deny)
- Modify: `backend/ngac/ngac_ops.go`, `backend/ngac/ngac_ops_test.go`
- Modify: `docs/specs/tenant-ngac-init/spec.md` (new per-workspace and per-department nodes; none under Members)

## Implementation Steps (TDD, deny first, each deny paired with an allow on the same fixture)
1. Write `pdp_contacts_visibility_test.go` building two workspaces W, V with depts Root→{A→{A1, A2}, H(hidden)→H1}, users:
   owner (placed in A2), member in A1, member in A2, limited-role holder in A1, member of H1, mgmt-role admin, user of V,
   ex-member (under DeptPeers_A1 at first, detached).
   DENY vectors first:
   - V user read `Dir_A` (different PC) → DENY.
   - A1 member read `Dir_H`, `Dir_H1` → DENY; read `DirPeer_A2` → DENY.
   - limited A1 read `Dir_A2`, `Dir_A1`, `DirectoryNoDept_W` → DENY with `ProhibitionDenied` set (proves override of an
     otherwise-ALLOW from Members).
   - limited A1 read `DirPeer_A` (parent) and `DirPeer_A2` (sibling) → DENY (one hop short / flat UA).
   - ex-member after removal from `DeptPeers_A1` and `W_Members` read `DirPeer_A1`, `DirectoryOwners_W` → DENY.
   - member `manage` on `Directory_W` → DENY.
   - user only under `DeptPeers_A1` is **not** a member: graph helper check (`IsWorkspaceMember` fake reader in
     `backend/pkg/policyclient/membership_test.go` style) → false; documents that peers never imply membership.
   ALLOW vectors: A1 member read `Dir_A2`, `DirectoryNoDept_W`, `DirectoryOwners_W`; H1 member read `Dir_H`; limited A1
   read `DirPeer_A1` and `DirectoryOwners_W` (owners always visible); owner read `Dir_H`; owner in a limited dept
   `manage` on `Directory_W`; mgmt-role admin `manage` on `Directory_W`.
   Batch: same matrix via `CheckAccessBatch` (`pdp_batch.go:20`) + `DecideBatch` prohibition path.
   Freshness: ALLOW cached for A1 member on `Dir_A2`, then assign A1's dept UA to `DirectoryLimited_W` → DENY after EPP
   invalidation (follow `epp_cache_keys_test.go` / `docs/specs/policy-decision-freshness`).
2. Run: fails only if graph helpers missing (compile) — add helpers + `ngac_ops_test.go` cases, then green.
3. Write spec `contacts-visibility` (graph diagram above, decision procedure, membership precondition, precedence, admin
   definition, deny scenarios, "a workspace without `Directory_W` has no rules: every member sees every member") and
   tenant-ngac-init additions.
4. `make check-ngac`, `make test s=policy`.

## Success Criteria
- All vectors green on single and batch paths; any edit that hangs Dir OAs under Members directly, puts DirectoryOwners
  under Open, or drops the prohibition, turns at least one DENY/ALLOW vector red (verify by temporarily mutating the fixture).
- `make check-ngac` green; spec merged in the same commit.

## Risk Assessment
- First production use of prohibitions (no service creates one today; only `write_server.go:350` and tests) (Med×High):
  batch path and CTE fallback must apply them. Add one vector where `Dir_*` is absent from the in-memory shard to exercise
  CTE fallback → prohibition still applied (`pdp_decision_engine.go:74` then `:77`). If not, stop and escalate (PDP change = new
  decision path needing its own deny tests).
- Shard/global prohibition source mismatch (Low×High): covered by `checkProhibitions` reading `e.graph` (global, `:159`).
- Rollback: helpers unused until phase 03; revert commit.
