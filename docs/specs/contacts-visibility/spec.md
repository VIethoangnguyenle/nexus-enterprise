# contacts-visibility

## Purpose
Decide, with NGAC and nothing else, which members of a workspace a person may see in Danh bạ: by default everyone, with two rules an administrator can add (hide a department, limit a role or a department to its own department), and owners always visible.

## Requirements

### Requirement: Membership is checked before any visibility decision
No visibility decision SHALL be made for a caller until the workspace has confirmed they are a member of it (a user node under `{workspace_id}_Members`, `policyclient.IsWorkspaceMember`). Nothing in the directory SHALL make anyone a member: no directory node is assigned under Members, Owners or a department UA, so reaching `DeptPeers_{department id}`, `DirectoryLimited_{workspace_id}` or a limited role confers no membership and no read of the directory.

#### Scenario: Peers and roles are not membership
- **WHEN** a user is only under `DeptPeers_{id}` or only in a role that is limited
- **THEN** `IsWorkspaceMember` is false and a read of any `Dir_`, `DirectoryOpen_` or `DirectoryOwners_` OA is DENY

#### Scenario: Someone who left
- **WHEN** a person is removed from `{workspace_id}_Members`, their department and `DeptPeers_{id}`
- **THEN** every read of the directory is DENY, the peer OA of their old department included

### Requirement: The directory is a subtree of Mgmt with one open branch
Each workspace that carries contacts visibility SHALL have the OA `Directory_{workspace_id}` under `{workspace_id}_Mgmt`, and under it: `DirectoryOpen_{workspace_id}` (with `DirectoryNoDept_{workspace_id}` and one `Dir_{department id}` per non-hidden top-level department mirroring the department tree), `DirectoryOwners_{workspace_id}`, one `Dir_{department id}` per hidden department (its sub-departments beneath it), and a peer tree of `DirPeer_{department id}` mirroring the department tree, in which a hidden department's `DirPeer` (its sub-departments' beneath it) sits directly under `Directory` like its `Dir`, not under its parent's `DirPeer`. `{workspace_id}_Members` SHALL hold `read` on `DirectoryOpen` and on `DirectoryOwners` and on nothing else of the directory. No new operation exists: the question is always `read` on a directory OA, and administration is `manage` on `Directory`.

#### Scenario: Default visibility
- **WHEN** no rule exists and a member reads `Dir_{department id}` of any department, `DirectoryNoDept_{workspace_id}` or `DirectoryOwners_{workspace_id}`
- **THEN** the decision is ALLOW

#### Scenario: Another workspace
- **WHEN** a member of workspace V reads any directory OA of workspace W, even if V's Members UA has been associated with it by mistake
- **THEN** the decision is DENY (different policy class)

#### Scenario: One hop short
- **WHEN** an ordinary member reads `Directory_{workspace_id}` itself, holds `manage` on it, writes any directory OA, or reads the peer OA of a department they are not directly in
- **THEN** the decision is DENY

### Requirement: Whoever holds manage on the directory sees everyone
A directory administrator SHALL be whoever holds `manage` on `Directory_{workspace_id}`: owners, and any role holding `manage` on Mgmt, through the Mgmt OA that Directory sits under. A caller who is an administrator is shown every member, a hidden department included and labelled as such; no rule limits them. An owner who sits in a limited department is still an administrator, because the limit prohibits `read` and the shortcut is `manage`.

#### Scenario: Owner and management role
- **WHEN** an owner, or a member of a role holding `manage` on Mgmt, asks `manage` on `Directory_{workspace_id}`
- **THEN** the decision is ALLOW, and DENY for an ordinary member and for a limited member

#### Scenario: Owner in a limited department
- **WHEN** an owner is in a department that is limited
- **THEN** `read` of an open department outside theirs is DENY (the limit), `manage` of `Directory_{workspace_id}` is ALLOW, and they are shown everyone

### Requirement: A hidden department is seen only by its own people and administrators
Hiding a department SHALL move its `Dir_{department id}` out of `DirectoryOpen`, and its `DirPeer_{department id}` out from under its parent's `DirPeer`, to sit directly under `Directory`, its sub-departments' `Dir` and `DirPeer` OAs following them (un-hiding moves both back), and associate the department's own UA with `read` on it. The department UA chains upward, so everyone in the department or any sub-department reads the hidden `Dir` and its subtree; nobody else does except an administrator. Owners placed in a hidden department stay visible to everyone (see the next requirement).

#### Scenario: Hidden department
- **WHEN** a member of an open department reads a hidden department's `Dir` or one of its sub-departments' `Dir`
- **THEN** the decision is DENY, and ALLOW for a member of that department or of one of its sub-departments reading the same OAs

#### Scenario: Hidden department and the peer tree
- **WHEN** a person placed directly in the parent department, or a limited person whose own department contains a hidden sub-department, reads that hidden department's `DirPeer` or its `Dir`
- **THEN** the decision is DENY, and ALLOW for the hidden department's own people reading the same OAs

#### Scenario: Un-hiding
- **WHEN** the department's `Dir` is moved back under its parent's `Dir` (or `DirectoryOpen`)
- **THEN** every member reads it again

### Requirement: Owners are always visible
`DirectoryOwners_{workspace_id}` SHALL sit under `Directory`, beside `DirectoryOpen` and never under it, and `{workspace_id}_Members` SHALL hold `read` on it, so that no limit reaches it. A person holding `{workspace_id}_Owners` is shown to every member whatever their department, hidden or limited, and a limited person always sees the owners.

#### Scenario: Limited person and the owners
- **WHEN** a person limited to their own department reads `DirectoryOwners_{workspace_id}`
- **THEN** the decision is ALLOW

#### Scenario: Owner in a hidden department
- **WHEN** an ordinary member reads a hidden department's `Dir` and `DirectoryOwners_{workspace_id}`
- **THEN** the first is DENY and the second ALLOW

### Requirement: A limited role or department sees only its own department
A role, or a department, SHALL be limited by assigning its UA to `DirectoryLimited_{workspace_id}` as an extra parent. `DirectoryLimited` is a UA under the policy class that holds no association and so grants nothing. The workspace SHALL hold one prohibition, named by `DirectoryLimitedProhibitionName(workspace id)`, whose subject is `DirectoryLimited_{workspace_id}`, whose operation is `read` alone, whose only target is `DirectoryOpen_{workspace_id}` and whose intersection flag is off. Everyone beneath a limited role or a limited department's subtree is limited, and stops being limited when they leave it, with no per-person upkeep. A limited person reads their own department through the peer tree: every user placed directly in a department SHALL also be assigned to the flat UA `DeptPeers_{department id}`, which holds `read` on `DirPeer_{department id}`; `DeptPeers` UAs are assigned directly under the policy class, never chained, so a person reaches their own department's peer subtree and no sibling or parent. The peer tree sits outside `DirectoryOpen`, so the prohibition never reaches it.

#### Scenario: Limited role holder
- **WHEN** a person in a limited role, in department A1, reads `Dir` of a sibling department, of their own department, of the root, or `DirectoryNoDept_{workspace_id}`
- **THEN** each is DENY, with the prohibition named as the cause, although the graph alone would ALLOW it through Members

#### Scenario: Limited person with no department
- **WHEN** a limited person has no department
- **THEN** they see only the owners and themself: every open department, `DirectoryNoDept_{workspace_id}` and every peer OA is DENY, `DirectoryOwners_{workspace_id}` is ALLOW

#### Scenario: Limited person's own tree
- **WHEN** the same person reads `DirPeer` of their own department or of a sub-department of it, or `DirectoryOwners_{workspace_id}`
- **THEN** each is ALLOW

#### Scenario: Parent and sibling peers
- **WHEN** the same person reads the `DirPeer` of their parent department or of a sibling
- **THEN** each is DENY (the `DeptPeers` UA is flat)

#### Scenario: Limited department
- **WHEN** a department's UA is assigned to `DirectoryLimited_{workspace_id}`
- **THEN** its people and its sub-departments' people are limited, and people of other departments are not

#### Scenario: Moving out
- **WHEN** a limited department's person is moved to a department that is not limited
- **THEN** they read the open tree again

#### Scenario: Limited person inside a hidden department
- **WHEN** a limited person sits in a hidden department
- **THEN** they still read that department's subtree (it is outside `DirectoryOpen`) and not the open tree

### Requirement: The limit holds on every decision path and is fresh
The prohibition SHALL be applied on the single check, on the batch check, wherever the object is resolved (for an OA or a person missing from memory the SQL fallback resolves the prohibition from the database's ancestors, see `policy-decision-freshness`), and whatever the decision cache holds: creating it, removing it, and limiting or freeing a role or department SHALL take effect for the people concerned as soon as the write returns, and a cached ALLOW the change contradicts SHALL be dropped by the invalidation the policy write performs. A workspace's teardown SHALL remove its prohibition by name, because a prohibition has no foreign key to the graph and survives the deletion of its nodes. The rule is per workspace: another workspace's limited people and rule are unaffected.

#### Scenario: Batch agrees with the single check
- **WHEN** the same person asks `read` on the same directory OAs one at a time and in one batch
- **THEN** the answers are identical, the limit applied in both

#### Scenario: Department limited while an ALLOW is cached
- **WHEN** a member's ALLOW on an open department is cached and their department is then assigned to `DirectoryLimited_{workspace_id}`
- **THEN** after the write's invalidation the same question is DENY, and an unrelated person's cached decision is untouched

#### Scenario: Rule created while an ALLOW is cached
- **WHEN** a limited role holder's ALLOW is cached before the prohibition exists and the prohibition is then created
- **THEN** after the write's invalidation the same question is DENY, while their own peer OA stays ALLOW

#### Scenario: Rule removed
- **WHEN** the prohibition is removed
- **THEN** limited people read the open tree again, and another workspace's rule is unchanged

### Requirement: A workspace without a directory has no rules
A workspace that has no `Directory_{workspace_id}` node SHALL have no visibility rules: every member sees every member, as before the directory existed, and nothing is denied for want of the nodes.

## Status
Specified ahead of the code, in part (no known divergence). The graph model, its names and the decisions above are verified by test vectors (`backend/services/policy/internal/ngac/pdp_contacts_visibility_test.go`, and on the real policy service by `backend/testutil/policy_contacts_visibility_test.go`); no service creates the directory nodes or the prohibition yet, and no endpoint filters Danh bạ by them.
