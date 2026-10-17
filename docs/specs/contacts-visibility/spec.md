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

### Requirement: The workspace service keeps the directory in step
The workspace service SHALL create the directory with every new workspace and mirror every department into it, as `tenant-ngac-init` specifies, so that the rules above hold without anyone maintaining the nodes by hand. Hiding a department moves both its `Dir` and its `DirPeer` and a department that is moved or created keeps that shape; a person removed from the workspace, or put in another department, loses the department edge and the peers edge with it; deleting a hidden department is refused while people are placed directly in it (they would be published in the parent), and once it is empty of people keeps its sub-departments hidden; a department whose directory nodes are missing is given them, per its parent's hidden state, by the first operation that needs them; moving a department whose directory nodes cannot follow takes the move back. Placing a person in a hidden department, or moving a department under one, hands out the `read` its UA holds on the hidden `Dir`: a directory administrator holds `manage` on that OA through Mgmt and SHALL be allowed to hand it out, and nobody else lacking `read` SHALL (`manage` stands in for `read` of a directory OA only, never for another operation or another OA).

#### Scenario: Ex-member
- **WHEN** a person who sat in a hidden department is removed from the workspace
- **THEN** they hold no edge to the department, its peers UA, `DirectoryLimited` or Members, and `read` of the hidden `Dir`, its `DirPeer` and `DirectoryOwners` is DENY

#### Scenario: Hidden department and the structure around it
- **WHEN** a hidden department is moved under another, or a sub-department is created under it
- **THEN** its `Dir` and `DirPeer` stay directly under `Directory` (the sub-department's under the hidden ones), and a person placed directly in the new parent still reads neither

#### Scenario: Deleting a hidden department
- **WHEN** a hidden department with a person placed directly in it is deleted
- **THEN** the request is refused and nothing changes; with nobody placed directly in it the department goes and its sub-departments stay hidden, their people still reading them

#### Scenario: A department without nodes
- **WHEN** a department of a workspace that has a directory has no directory nodes and a person is placed in it
- **THEN** its nodes are built (its parent's first) under the parent's, a hidden parent's making it hidden, and the people already in it get its peers edge

#### Scenario: Directory administrator places a person in a hidden department
- **WHEN** a role holding `manage` and `invite` on Mgmt, without `read` of the hidden `Dir`, puts a member in the hidden department
- **THEN** it succeeds; a department whose UA grants `write` on the hidden `Dir`, or `read` on an OA that is not a directory node, is still refused

### Requirement: Existing workspaces get their directory from a migration
Workspaces and departments that exist when the code ships have no directory nodes, and so no rules; migration 064 (`tenant-ngac-init`) builds them in a later release, leaving what the code already made as it is. The code works before and after: a workspace without the directory is skipped by every operation above.

### Requirement: The workspace serves each member's scope
The workspace service SHALL compute, for the verified caller of a request, a `DirectoryScope` from the membership check and policy decisions alone, and serve it to the auth service as the gRPC method `GetDirectoryScope(workspace_id)`. The caller SHALL be a member (`policyclient.IsWorkspaceMember` over the writer) before any decision is asked; a caller who is not gets PermissionDenied. The scope SHALL say: whether the directory is `ready`; whether the caller is `admin` (holds `manage` on `Directory_{workspace_id}`); whether they are `limited` (display only: they sit beneath `DirectoryLimited_{workspace_id}`); the departments they may see; whether people with no department and the owners are visible; the owners' node ids; for an administrator the hidden departments; and the owners group and the custom roles with who holds each directly (never Members). For anyone who is not an administrator one batch of `read` decisions on `DirectoryNoDept`, `DirectoryOwners` and every department's `Dir_` and `DirPeer_` OA decides: a department is visible when either is ALLOW. A department of a workspace that has a directory but whose nodes are missing is not visible to a non-administrator (it is logged at error level), and an administrator sees every department. The method is in no `ServiceOK` list, so a service speaking only for itself, the auth service included, is refused (Unauthenticated): the scope is a person's, and the auth service asks it on behalf of the person whose contacts it is about to list. A decision or lookup that cannot be had, a directory missing one of its five nodes, and a limited caller with no prohibition behind the limit are an outage (Unavailable, 503 at the edge), never a wider scope. A workspace with no `Directory_{workspace_id}` is `ready: false`: no rule can exist there, so every department, the people with no department and the owners are visible and the caller is neither administrator nor limited.

#### Scenario: Stranger and peers-only caller
- **WHEN** a user of another workspace, or one who sits only under `DeptPeers_{id}`, asks for the scope
- **THEN** the answer is PermissionDenied and no policy decision about the directory is asked for them

#### Scenario: Service on its own identity
- **WHEN** the auth service (or any service) calls `GetDirectoryScope` with no person on the call
- **THEN** the answer is Unauthenticated; with the person's identity forwarded it is their scope

#### Scenario: Limited caller
- **WHEN** a person limited to their own department asks for the scope
- **THEN** `limited` is true, the departments are their own and those beneath it, the owners are visible and people with no department are not; with the prohibition removed the answer is Unavailable

#### Scenario: Outage is not a wide view
- **WHEN** the batch check, the administrator check or the directory lookup fails
- **THEN** the answer is Unavailable, not an empty or a full scope; a plain "not found" of `Directory_{workspace_id}` is `ready: false`

### Requirement: Reads of people and departments follow the scope
Every read of the workspace service that a member may make and that names people or departments SHALL return only what the caller's scope allows: `GET /api/workspaces/:id/departments` the visible departments (a department whose parent is not visible is a root in the caller's list; only an administrator gets the `hidden` flag, true for a department hidden from the directory); `GET /api/workspaces/:id/members` (and the gRPC `ListMembers`) the people of the visible departments, the people with no department when that is visible, the owners and the caller; `GET /api/workspaces/:id/roles` counts only the holders the caller may see. An administrator and a workspace that is not ready see everything. The admin people table (`GET /api/workspaces/:id/admin/members`) needs `manage` and is unchanged.

#### Scenario: Hidden department
- **WHEN** a member of an open department lists departments, members and role counts while another department is hidden
- **THEN** the hidden department, its people and their share of the counts are absent, an owner who sits in it is present, and the same calls by a member of the hidden department, or by an administrator (who also sees the `hidden` flag), include them

#### Scenario: Limited person
- **WHEN** a person limited to their own department lists members and departments
- **THEN** they get their department's people and subtree, the owners and themself, and no one from a sibling department or from no department

### Requirement: Administrators set the rules through the workspace service
Whoever holds `manage` on the Mgmt OA SHALL be able to read and change the rules in an organization workspace whose directory exists, and nobody else. `GET /api/workspaces/:id/admin/directory-scope` answers `{ready, hidden_departments: [{id, name, parent_id, member_count}], limited: [{kind: "role"|"department", id, name, parent_id, member_count}]}` (people counted beneath a department, directly in a role); `ready: false` with status 200 for a workspace that cannot carry rules yet (no directory nodes, or a personal workspace), which is not an outage. `PUT` and `DELETE` on `.../hidden/:deptId` hide and unhide a department (both its `Dir` and `DirPeer` move, with their subtrees, and its own UA is granted or loses `read` of its `Dir`; unhiding puts both back under the parent's nodes, or `DirectoryOpen` and `Directory` for a top-level department, also when that parent is itself hidden), and on `.../limited/roles/:roleId` and `.../limited/departments/:deptId` limit and free a custom role or a department by assigning its UA under, or removing it from, `DirectoryLimited_{workspace_id}`. The subject SHALL belong to this workspace: a department of another workspace, an unknown id, the Owners or Members UA, a department UA given as a role and the Limited UA itself are 404; a write on a workspace that cannot carry rules is 503 (no directory yet) or 400 (personal). Each write is idempotent, takes effect for the people concerned as soon as it returns (the policy write invalidates their cached decisions) and announces a workspace event so open Danh bạ screens refetch.

#### Scenario: Plain member or another workspace's administrator
- **WHEN** a plain member, a limited member, or the owner of another workspace calls any rule endpoint
- **THEN** it is 403 and nothing in the graph changes

#### Scenario: Foreign or built-in subject
- **WHEN** an administrator names a department or role of another workspace, the Owners UA, the Members UA, or an id that does not exist
- **THEN** it is 404 and neither workspace's graph changes

#### Scenario: Hide, unhide and the exact parent
- **WHEN** a department is hidden and unhidden, including a child of a hidden department hidden and unhidden before and after its parent
- **THEN** each node returns to the parent it had, and a member who is not in it sees it only while it is not hidden

#### Scenario: Before the migration
- **WHEN** an administrator opens the rules of a workspace that has no directory nodes
- **THEN** the read answers `ready: false` and a write is refused as not ready, while every member still sees every member

### Requirement: The auth service lists only what the scope allows
`GET /api/workspaces/:id/contacts` SHALL answer from `tenant_users` rows that are `active`, narrowed by the caller's `DirectoryScope`, and by nothing weaker: the caller's row must be active, then the auth service asks the workspace service for the scope **as the caller** and a person is listed only if their department is in the scope's visible departments, or they have no department and people with no department are visible, or they are an owner and owners are visible, or they are the caller. An administrator, and a workspace whose scope is `ready: false`, see every active member. The list, the `total`, the keyset paging and every filter (`search` by name, e-mail, title or department; `department_id`; `department`; `location`) SHALL apply inside that one predicate, so a hidden person is neither listed nor counted nor found by searching, and an e-mail address reaches only a caller who may see its person. The answer carries `department_id` (the assigned one, or empty), `roles` (the roles of the scope the person holds directly; the owners group has kind `owners` and no name; always a list) and a top-level `visibility`, `"limited"` when the caller is limited to their own department (the scope's `limited`, for a caller who is not an administrator) and `"all"` otherwise. A department outside the scope SHALL NOT be named anywhere in the answer, even on a row the caller may see (an owner who sits in it): its name and id are empty there, and `department_id`, `department` and `search` match only departments the scope shows (the caller's own department excepted, on their own row). A query without a scope SHALL show nobody but the caller.

#### Scenario: Hidden person
- **WHEN** a member of an open department lists, counts and searches (by name and by e-mail) while another department is hidden
- **THEN** none of the hidden department's people is listed, counted or found, the caller's own department, the owners and the caller are, and `visibility` is `all` or `limited` as the scope says

#### Scenario: Limited caller
- **WHEN** a person limited to their own department lists the directory
- **THEN** they get their department's people, the owners wherever the owners sit, and themself; not a person of another department, and not people with no department unless the scope shows them

#### Scenario: The workspace cannot say
- **WHEN** the workspace service is unavailable, times out, fails, or answers no scope
- **THEN** the answer is 503 and no row is read; when it answers PermissionDenied the answer is 403; the unfiltered list is never the fallback

#### Scenario: Hidden department on a visible row
- **WHEN** an owner who sits in a hidden department is listed to a limited caller, who also filters by that department's id or name, or searches for its name
- **THEN** the owner's row carries no department name or id, and each of those three queries returns nothing and counts nothing; the same queries by an administrator find the owner

#### Scenario: Not an active member
- **WHEN** the caller has no membership row, or a `removed`, `disabled` or `invited` one
- **THEN** the answer is 403 and the workspace service is not asked

### Requirement: Names can be resolved apart from the directory
`POST /api/workspaces/:id/contacts/resolve` with `{user_ids?, node_ids?}` (at most 200 in all) SHALL return `{people: [{user_id, ngac_node_id, display_name, avatar_url}]}` for the people named who have a `tenant_users` row in that workspace **with any status** (`active`, `invited`, `disabled`, `removed`) and for no one else: never an e-mail address, title or department, ids of another workspace or unknown ids are left out, and no one can be listed this way. This is a deliberate exception to "readers of `tenant_users` filter on `active`": a person the directory hides, or who left, keeps a name wherever their work stays (chat, approvals, documents, calendar). The caller SHALL pass `requireActiveMember`: a member in the graph (the membership helper over the policy writer, so a removal counts at once) whose row, when there is one, is not `removed`, `disabled` or `invited` (a graph member whose row is missing is let in, as in sign-in and refresh); a policy outage is 503, not a denial. A body over 64 KiB is 413. The workspace service is not asked, so names keep working when it is down. A display name is empty until the person saved their profile.

#### Scenario: Hidden and removed people have names
- **WHEN** a member resolves a person of a hidden department and one who was removed
- **THEN** both come back with name and avatar, while the directory lists neither

#### Scenario: Other workspace and non-members
- **WHEN** a user of another workspace is named, or a person with an active row but no place under Members, or a removed or disabled member, asks
- **THEN** the named user is omitted, and the asker gets 403 with no name in the body

#### Scenario: Too many
- **WHEN** more than 200 ids are sent, across both lists
- **THEN** the answer is 400 and nothing is looked up

## Status
Built. The graph model and its names are verified by test vectors (`backend/services/policy/internal/ngac/pdp_contacts_visibility_test.go`, and on the real policy service by `backend/testutil/policy_contacts_visibility_test.go`). The workspace service creates the directory with each new workspace, mirrors departments and their people into it and tears it down (`directory_graph*.go`); migration 064 builds it for what predates the code (`scripts/test-migration-064.sh`). It computes each caller's scope, filters departments, members and role counts by it and serves the rules and `GetDirectoryScope` (`directory_scope*.go`, proved against the real policy service in `directory_scope_realpolicy_test.go`). The auth service lists Danh bạ through the scope and resolves names apart from it (`contacts_visibility_test.go`, `contacts_visibility_realpolicy_test.go`, `internal/rest/contacts_resolve_test.go`, and the SQL predicate against a real database in `internal/store/contacts_scope_test.go`); the screens do not yet read the new fields. `scripts/seed-contacts-demo.sh` builds a demo organization through the APIs for development.
