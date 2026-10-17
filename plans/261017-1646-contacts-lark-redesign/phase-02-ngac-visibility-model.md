---
phase: 2
title: "NGAC visibility model + test vectors"
status: pending
priority: P1
effort: "4h"
dependencies: [1]
---

# Phase 2: NGAC visibility model + test vectors

## Overview
Policy-model change (skill `ngac-policy-change`). Defines the graph shape for contacts visibility, the name helpers, and
PDP test vectors with DENY cases first. No PDP code change: the model uses associations and the existing prohibition
deny-override (`backend/services/policy/internal/ngac/pdp_decision_engine.go:150` `checkProhibitions`, `:187`
`matchProhibitions`; batch path `pdp_decision_batch.go:87`). Spec + vectors + helpers in **one commit**.

## Requirements
Decision "may caller C see person P in the directory" =
- **admin**: `Check(C, Directory_<ws>, manage)` ALLOW → sees everyone, hidden labelled; else
- `read` ALLOW on `Dir_<P.dept>` (or `DirectoryNoDept_<ws>` when P has no department) **or** on `DirPeer_<P.dept>`;
- C always sees themself (PEP rule, not graph).

Rules expressed:
- Default: everyone in the workspace sees all non-hidden departments and people without a department.
- Hide department D: only people in D's subtree + admins see D's subtree.
- Limit role R or department X: holders of R / people in X's subtree see only their own department's subtree.

## Architecture
Graph per workspace W (new nodes; names from `backend/ngac`):
```
PC_W
├─ W_Mgmt (OA, existing) ── Owners {all ops}, custom mgmt roles {manage, invite}
│   └─ Directory_W (OA)                         admins: manage inherited from Mgmt → admin shortcut
│       ├─ DirectoryOpen_W (OA)  ◄── Members_W {read}         (default visibility)
│       │    ├─ DirectoryNoDept_W (OA)                         people with no department
│       │    └─ Dir_<d> (OA) … mirrors department tree        (non-hidden top-level depts)
│       ├─ Dir_<h> (OA)          ◄── Dept_<h> {read}           hidden dept h: moved out of Open (subtree follows)
│       └─ DirPeer_<d> (OA) … mirrors department tree (top-level under Directory_W)
├─ DirectoryLimited_W (UA)  ◄─ role UA R / Dept_<x> UA assigned here (extra parent) = rule "limited"
│     prohibition DirectoryLimited_W: subject DirectoryLimited_W, ops [read], targets [DirectoryOpen_W], intersection=false
└─ DeptPeers_<d> (UA, flat, directly under PC_W) ◄─ users directly in d (mirror of U→Dept_<d>)
      association DeptPeers_<d> → DirPeer_<d> {read}
```
Why each piece:
- Peer tree + flat `DeptPeers` UA: `Dept_<d>` UAs chain upward (`department.go:88-95`), so any association on a parent
  dept UA would reach every sub-department's people; a flat UA reaches only direct members, and the DirPeer tree gives
  them their own subtree. The prohibition cannot touch DirPeer OAs (not under Open).
- `DirectoryLimited_W` as an extra parent of R/Dept_x: limitedness follows role assignment and department moves with no
  per-user upkeep; it carries no association, so it grants nothing.
- Directory under Mgmt: Owners read everything (AllOwnerOps on Mgmt), any `manage` holder gets the admin shortcut,
  role grants need no new code (`ngac/areas.go` management = manage, invite).
- Precedence (documented, plan Q1): a limited user inside hidden D still sees D's subtree (Dir_h is not under Open).

Helpers to add in `backend/ngac/ngac_ops.go` (ID-typed, next to `DeptUAName` at line 159):
`DirectoryOAName(WorkspaceID)`, `DirectoryOpenOAName(WorkspaceID)`, `DirectoryNoDeptOAName(WorkspaceID)`,
`DirectoryLimitedUAName(WorkspaceID)`, `DirectoryLimitedProhibitionName(WorkspaceID)`, `DeptDirOAName(DeptID)`,
`DeptPeerOAName(DeptID)`, `DeptPeersUAName(DeptID)`; property `PropTypeDirectory = "directory"` for these nodes so
`isRole`/classifiers ignore them.

## Related Code Files
- Create: `backend/services/policy/internal/ngac/pdp_contacts_visibility_test.go` (model on
  `pdp_vnpay_scenarios_test.go`, single + batch path)
- Create: `docs/specs/contacts-visibility/spec.md` (purpose, graph, decision, rules, scenarios incl. deny)
- Modify: `backend/ngac/ngac_ops.go`, `backend/ngac/ngac_ops_test.go`
- Modify: `docs/specs/tenant-ngac-init/spec.md` (new per-workspace and per-department nodes)

## Implementation Steps (TDD, deny first)
1. Write `pdp_contacts_visibility_test.go` building two workspaces W, V with depts Root→{A→{A1, A2}, H(hidden)→H1}, users:
   owner, member in A1, member in A2, limited-role holder in A1, member of H1, mgmt-role admin, user of V.
   DENY vectors first:
   - V user read `Dir_A` (different PC) → DENY.
   - A1 member read `Dir_H`, `Dir_H1` → DENY; read `DirPeer_A2` → DENY.
   - limited A1 read `Dir_A2`, `Dir_A1`, `DirectoryNoDept_W` → DENY with `ProhibitionDenied` set (proves override of an
     otherwise-ALLOW from Members).
   - limited A1 read `DirPeer_A` (parent) and `DirPeer_A2` (sibling) → DENY (one hop short / flat UA).
   - user removed from `DeptPeers_A1` read `DirPeer_A1` → DENY.
   - member read/manage `Directory_W` with `manage` → DENY.
   ALLOW vectors: A1 member read `Dir_A2`, `DirectoryNoDept_W`; H1 member read `Dir_H`; limited A1 read `DirPeer_A1`;
   owner read `Dir_H`; mgmt-role admin `manage` on `Directory_W`.
   Batch: same matrix via `CheckAccessBatch` + `DecideBatch` prohibition path.
   Freshness: ALLOW cached for A1 member on `Dir_A2`, then assign A1 member to limited role R → DENY after EPP
   invalidation (follow `epp_*` tests / `docs/specs/policy-decision-freshness`).
2. Run: fails only if graph helpers missing (compile) — add helpers + `ngac_ops_test.go` cases, then green.
3. Write spec `contacts-visibility` (graph diagram above, decision procedure, precedence, admin definition, deny scenarios)
   and tenant-ngac-init additions.
4. `make check-ngac`, `make test s=policy`.

## Success Criteria
- All vectors green on single and batch paths; any edit that hangs Dir OAs under Members directly, or drops the
  prohibition, turns at least one DENY vector red (verify by temporarily mutating the fixture).
- `make check-ngac` green; spec merged in the same commit.

## Risk Assessment
- First production use of prohibitions (Med×High): batch path and CTE fallback must apply them. Vectors cover single +
  batch; add one vector where `Dir_*` is absent from the in-memory shard to exercise CTE fallback → prohibition still
  applied (`pdp_decision_engine.go:76`). If not, stop and escalate (PDP change = new decision path needing its own deny tests).
- Shard/global prohibition source mismatch (Low×High): covered by checkProhibitions reading global prohibitions (`:160`).
- Rollback: helpers unused until phase 03; revert commit.
