# tenant-identity

## Purpose
Separate who a person is globally from who they are inside a tenant, so one human can hold memberships in many tenants without their identities leaking across the boundary.

## Requirements

### Requirement: Users have global identity
The system SHALL store a global user identity with `email` (unique, primary login), `union_id` (cross-tenant identifier), and `display_name`.

#### Scenario: User registers with email
- **WHEN** a user provides email, password, and display_name
- **THEN** the system creates a user record with a unique `union_id` and stores the email as the primary login credential

#### Scenario: Email uniqueness enforced
- **WHEN** a user attempts to register with an email that already exists
- **THEN** the system rejects the registration with an "already exists" error

### Requirement: Users have tenant-scoped identity
The system SHALL assign each user a unique `open_id` per tenant membership. The `open_id` is generated on join and stored in `tenant_users`.

#### Scenario: User joins a tenant
- **WHEN** a user is added to a tenant (via signup, invite, or domain auto-join)
- **THEN** the system creates a `tenant_users` record with a unique `open_id` for that user-tenant pair

#### Scenario: open_id is unique across all tenants
- **WHEN** the system generates an open_id
- **THEN** the open_id is globally unique (UUID) and can be used to look up the user-tenant pair

### Requirement: Users can belong to multiple tenants
The system SHALL support a user belonging to multiple tenants via the `tenant_users` table with `(tenant_id, user_id)` as the composite primary key.

#### Scenario: User belongs to two tenants
- **WHEN** a user is a member of Tenant A and Tenant B
- **THEN** the system returns both tenants when listing the user's memberships
- **AND** each membership has its own `open_id`, `role`, and `status`

### Requirement: Tenant membership has status tracking
The system SHALL track membership status as one of: `active`, `invited`, `disabled`, `removed`.

`removed` is a tombstone: a person taken out of the workspace (by an administrator, or by leaving) keeps their
`tenant_users` row with `status = 'removed'` and no department, instead of the row being deleted. Deleting it made
"removed" look like "never invited", so a Google Workspace sign-in for the company's domain joined them again.
Only `active` is membership: every reader that means "who is in this workspace" (contacts, approvers, free/busy
names, the department head-count, "anyone else left in a workspace being deleted") filters `status = 'active'`.
Display-only readers (a name on an old approval) keep the person. The row is deleted only when the workspace is.

An invitation reactivates only a `removed` row. A `disabled` person (an administrator's decision) and an `invited`
one stay as they are. The graph (`ngac_*`) is the truth for what a person may do; `tenant_users` is its listing and is
written after the graph. Migration 062 widens the CHECK constraint (it ships with or before the code that writes `removed`); migration 063 gives every row its `ngac_node_id`, lists the graph
members who had no row, and tombstones `active` rows whose person is not under `<ws>_Members` (only in workspaces whose
graph holds at least one member; a workspace whose graph holds none is reported and left alone, because its graph was
never populated, not emptied). Until 062 runs a removal still detaches the person (every decision and every refresh
reads the graph) but reports that the listing could not be updated.

### Requirement: tenant_users is a projection of the graph, written after it
`tenant_users` SHALL be written only after the graph holds the person, and a tenant-scoped token SHALL be minted only for a person the graph holds.

Membership is the edge under the workspace's own `<ws>_Members` UA (an owner: under `<ws>_Owners`, which is beneath it).
Every workspace has them. `TenantMember_<ws>` and `TenantOwner_<ws>` are not required: the workspaces made at sign-up
never had them, and some old organizations were never given any. A join assigns them as well only where they exist and
sit under `Members`/`Owners`, and a missing one is never an error. A join therefore (1) checks `IsAssigned` for each
target UA and assigns only where the edge is missing, (2) inserts the row, (3) if the row fails, removes only the edges
this call made (an edge the person already had is theirs) and reports the failure. Joining by invitation does the same:
the row is ensured even for someone the graph already holds (so a lost row catches up; it reactivates only `removed`),
and a failed listing takes back only an edge that call made.

A row that cannot be written after the edge leaves the graph holding a person with no row. The next sign-in lists
them: a graph member with no row at all gets one (owner if under `<ws>_Owners`, else member); a row in any state
(`removed`, `disabled`, `invited`) is never overwritten. Taking an edge back is guarded against racing joins: it is not
done when the failure is a cancellation or deadline (the write may have committed), and after any undo the row is read
again and, if another call has made it active, the edges are put back.

An invitation is refused (403, the offer stays open, nothing written to the graph) for a person whose row is
`disabled`; a `removed` person and a new one are admitted.

#### Scenario: Joining a workspace that has no TenantMember UA
- **WHEN** a person joins (sign-up, a Google `hd` domain join, an accepted invitation) a workspace without a `TenantMember_<ws>` UA
- **THEN** they are assigned under the workspace's Members UA (Owners for an owner) and then listed, and they can sign in and refresh into it

#### Scenario: The graph write fails
- **WHEN** the edge cannot be written
- **THEN** no active row is written and the join reports the failure; sign-in continues without that tenant

#### Scenario: The row fails after the edge
- **WHEN** the row cannot be written after the edge was made
- **THEN** the edge this call made is removed; an edge that was there before the call stays

#### Scenario: The row failed and the next sign-in
- **WHEN** a join's edge was written but its row was not, and the person signs in again
- **THEN** the row is written and the workspace is listed; a `removed` or `disabled` row is left alone

#### Scenario: A suspended person is invited again
- **WHEN** the person's row is `disabled` and they accept an invitation
- **THEN** it is refused and they are not put under Members

#### Scenario: Creating a workspace on purpose
- **WHEN** a person creates a workspace and the owner edge or the row cannot be written
- **THEN** the workspace is removed through the workspace service and no row remains

#### Scenario: Invited user before signup
- **WHEN** an admin invites an email that has no registered user
- **THEN** the system creates a `tenant_users` record with status `invited`

#### Scenario: Invited user completes signup
- **WHEN** an invited user registers with the invited email
- **THEN** the system updates the `tenant_users` status from `invited` to `active`

#### Scenario: A removal leaves a tombstone
- **WHEN** an administrator removes a member, or a member leaves
- **THEN** the person is detached in the graph first, `member_removed` is announced at once, and then the row is set to `removed` with no department
- **AND** if the row cannot be written the announcement has already gone out and the failure is returned

#### Scenario: An invitation brings back a removed person only
- **WHEN** a removed person accepts a new invitation
- **THEN** their row becomes `active` again
- **BUT** a `disabled` or `invited` row is left as it is

### Requirement: A refresh checks that the person still belongs to its tenant
`POST /api/auth/refresh` SHALL look the refresh token up without spending it, then check that the person is a
member of the tenant the token names, then rotate. A member is a person who sits under `<ws>_Members` in the graph
(asked of the policy writer, so a removal made a moment ago counts) and whose row, if there is one, is not `removed`,
`disabled` or `invited`; a graph member with no row is allowed until migration 063 lists them. A tenant that no longer
exists, or that has no Members UA, holds nobody.

- A member gets a token for the same tenant, as before.
- A person who is not a member keeps their session: the successor refresh token names the tenant they still belong to
  (the default one, an owned workspace first, as at sign-in), or none, and the answer carries `tenant_id` and
  `moved: true` so the client can follow. `moved` is present only for such a membership move; a client reacts to
  that flag and never to a mere difference of tenants (another tab may have switched workspace on purpose). The
  session family stays; their other tenants are unaffected. A tenant whose Members UA is gone or is only a lookalike
  (not under the workspace's policy class) holds nobody: that is a denial, not an outage.
- A refresh token that was spent a few seconds ago (10 s) answers with the successor it already produced, and the
  same `moved` answer, instead of being treated as a replay: two tabs of one browser share the cookie and a removal
  makes both ask at once. One spent longer ago is a replay and revokes the whole family. Which of the two it is
  is decided by the refresh store alone, without asking the policy. The SPA also takes a browser-wide lock
  (`navigator.locks`, name `nexus-refresh`) around a refresh, so the second tab presents the cookie the first has
  replaced; without Web Locks it waits a small random time first.
- When the answer cannot be had (the policy, the database or Redis does not respond) the response is `503` with code
  `unavailable`, the refresh cookie is neither cleared nor changed and the token is not spent, so the same cookie works
  once the service is back. Only a token that is actually refused (`401`) clears the cookie.

The SPA treats `401`/`403` from refresh as the end of the session, and anything else (a `5xx`, a network failure) as
"could not be asked": the session is kept and the request that wanted a new token fails with a `503` of its own instead
of signing the person out. When a refresh puts the session in another workspace, the SPA drops what it holds of the old
one, opens the new one and says so without naming an id. A WebSocket closed with code `4403` (see `resource-pep-coverage`)
is not reconnected with the same token; the client asks for a new one (once a minute at most, backing off while the server cannot answer). A person who left on purpose is not told they were removed.

The exposure of a removed person is bounded by the access-token lifetime (15 minutes) for services that trust the
tenant in the JWT; it is zero for approval and for WebSocket connections, which ask the graph on every call/connect.

#### Scenario: A removed person is moved to their other workspace
- **WHEN** a person detached from workspace W refreshes with a token for W and still belongs to V
- **THEN** the new access token is for V, the answer says `tenant_id: V`, and the next refresh keeps them in V

#### Scenario: A removed person with no other workspace
- **WHEN** a person removed from W, which was their only workspace, refreshes
- **THEN** the session continues without a tenant

#### Scenario: The graph and the table disagree
- **WHEN** the row says `active` but the graph no longer holds the person, or the graph holds them but the row says `removed`, `disabled` or `invited`
- **THEN** they are treated as not a member of that tenant

#### Scenario: The policy is down during a refresh
- **WHEN** a refresh arrives while the policy cannot answer
- **THEN** it answers 503, the cookie is untouched, and the same cookie refreshes once the policy is back

### Requirement: A workspace's type is decided when it is made
Every workspace SHALL have a type, `personal` or `organization` (`workspaces.type`, NOT NULL,
CHECK-constrained, no default — an insert that does not state it fails), set by how it came to be:

- every new account gets a `personal` workspace of its own when it is created (OTP or Google, a
  company account with a Google Workspace domain included); signing in never creates an organization.
  That workspace is named after the person (their display name, nothing added) and follows their name
  when they change it — first of all at the profile step, from the handle sign-up gave them — for as
  long as it still carries their previous name; a workspace someone renamed keeps its name;
- a workspace created on purpose ("Tạo workspace", `POST /api/me/workspaces` with `type`) has the type
  the person chose, `personal` or `organization`; a person may have several of either. A request with
  no `type` (a client older than the choice) creates an `organization`; any other value is refused (400)
  and nothing is created.

The workspace service's `CreateWorkspace` SHALL refuse a missing or unknown type (`InvalidArgument`)
before writing anything to the graph or the database. The type is returned with the workspace on
`GetWorkspace`, `ListWorkspaces` (`GET /api/workspaces`), `GET /api/me/workspaces` and
`GET /api/me` (`current_tenant.type`); the frontend reads it through `useWorkspaceType()`, which
reports nothing (not `personal`) while the type is unknown.

Workspaces that existed before the type was set at creation were backfilled once by migration 044:
a workspace named "<someone>'s Workspace" with exactly one active member and no claimed domain is
`personal`; every other one is `organization`.

#### Scenario: First sign-in makes a personal workspace
- **WHEN** a new account is created by a one-time code or by Google, with or without a company domain
- **THEN** the workspace made for them has type `personal`
- **AND** a company account whose domain an organization already owns also joins that organization

#### Scenario: A workspace created on purpose has the chosen type
- **WHEN** a signed-in person creates a workspace and chooses `personal` or `organization`
- **THEN** it has that type

#### Scenario: A type nobody stated is refused
- **WHEN** `CreateWorkspace` is called with no type, or with any value other than the two
- **THEN** it fails with `InvalidArgument` and nothing is created

### Requirement: A personal workspace has no assets and no approvals
The workspace type SHALL be enforced by the services that own the features, not only hidden by the screens. In a
`personal` workspace the asset service and the approval service refuse every operation (PermissionDenied, REST 403;
see `asset-authorization` and `approval-screens`), and the messaging service refuses a message that links an
approval request (`linked_entity_type` `approval`; a direct message is judged by the workspace the sender is signed in
to). Each reads `workspaces.type` itself, before it asks the policy, and fails closed: an unknown workspace or a
type that is not `organization` is a refusal, and a failure to read it is an internal error. Nothing is deleted:
what a personal workspace already holds is kept, hidden, and is usable again if it becomes an organization. The type is
not a node of the policy graph and no decision of the policy service depends on it. The document service has no
approval operation of its own (a text's `draft`/`active`/`archived` status is not an approval), so it has nothing to refuse.

#### Scenario: Assets and approvals refuse in a personal workspace
- **WHEN** any asset or approval operation is called in a personal workspace, by anyone, whatever they hold in the graph
- **THEN** it fails with PermissionDenied / 403 and changes nothing

#### Scenario: An approval card in a personal workspace
- **WHEN** a message linking an approval request is sent in a personal workspace
- **THEN** it is refused (403) and not stored; other messages and cards are sent as usual

#### Scenario: Organization unaffected
- **WHEN** the same operations are called in an organization workspace
- **THEN** they are decided by the policy as before

### Requirement: The owner converts a personal workspace to an organization, once
A personal workspace becomes an organization through `POST /api/workspaces/{id}/convert-to-organization` (workspace
service). The caller SHALL hold `manage` on the workspace's management OA **and** be an owner of it (a member of its Owners
group, the same test the owner changes and the last-owner rule use, so an owner added later can convert); either missing is a refusal (403), and a caller who may not convert is refused before anyone
learns the workspace's type. A workspace that is an organization already, whether made one or converted, answers 409
`{"reason":"already_organization"}`; an unknown workspace answers 404. Conversion is one way and cannot be repeated.

It changes only `workspaces.type`, in one conditional statement (`... WHERE id = $1 AND type = 'personal'`), together
with `converted_by` (the user who converted) and `converted_at` (migration 046; both stay empty for a workspace that was
made an organization). The owner check, the type check and the update run under the workspace's owner lock (the one owner changes take), so a conversion cannot land after the caller stopped being an owner. Of two requests at once exactly one converts and the other gets 409. Nothing is written to the
policy graph: an organization needs no node a personal workspace lacks (an asset type's OA is created on first use,
approvals use the tenant schema, which the approval service creates on first use). Nothing is deleted: what the
workspace held while personal is kept and is usable now. The workspace service announces a workspace update in realtime
so every open session refreshes. The workspace service has no administrator audit log; `converted_by` and `converted_at`
are the record of who converted and when, shown by name in the settings screen.

Success answers `200 {"type":"organization","converted_by_name":"<display name>","converted_at":"<RFC 3339>"}`.
`GET /api/workspaces/{id}/details` also returns `type`, `is_owner` (the caller is in the Owners group), `owner_name` (the names of the current owners joined with ", ", never an id),
`owner_names` (the names of the current owners as an array, never an id), `converted_by_name` and `converted_at` (RFC 3339, or empty when never converted).

#### Scenario: The owner converts
- **WHEN** an owner of a personal workspace (the creator or one added later) converts it
- **THEN** its type becomes `organization`, the answer names who converted it and when, and open sessions refresh
- **AND** assets and approvals, refused until now, are decided by the policy as in any organization

#### Scenario: Not the owner
- **WHEN** a member, or someone holding `manage` who is not an owner, or a person from another workspace, converts it
- **THEN** it fails with 403 and the type is unchanged

#### Scenario: Already an organization
- **WHEN** the owner converts a workspace that is an organization, or converts it a second time
- **THEN** it fails with 409 `already_organization` and nothing changes

#### Scenario: Details after conversion
- **WHEN** a member reads the workspace details after it was converted
- **THEN** they see type `organization`, the owner's and the converter's names, and the time of conversion
