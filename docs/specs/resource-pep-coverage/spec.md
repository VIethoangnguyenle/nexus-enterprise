# resource-pep-coverage

## Purpose
Record which policy decision guards each resource operation outside workspace administration, so
that an endpoint without a check is visible as a gap instead of silently trusting the session.
Workspace administration is covered by `workspace-admin-authorization`.

## Requirements

### Requirement: Every PEP fails closed
Each check listed here SHALL treat an empty caller, an unresolvable OA, a policy-service error,
or any decision other than ALLOW as a denial. The caller SHALL come from verified JWT claims at
the REST edge and from gRPC request metadata between services (see "Caller identity on gRPC").

#### Scenario: Policy service error
- **WHEN** the PDP call for any guarded operation errors
- **THEN** the operation is denied and nothing is written

### Requirement: Messaging operations are guarded

| Operation | Op | Object |
|---|---|---|
| Create channel | `create_channel` | the workspace Channels OA (`ngac.ChannelsOAName(wsID)`), ID-keyed only |
| Update channel (`PATCH /api/channels/:chId`) | `manage` | the channel's content OA |
| WebSocket subscribe to a channel | `read` | the channel's content OA |

#### Scenario: Subscribe without read
- **WHEN** an authenticated WebSocket client sends `SubscribeRequest` for a channel whose OA it cannot read
- **THEN** it receives an `ErrorEvent` with code 403
- **AND** it receives none of that channel's messages

#### Scenario: WebSocket token with another algorithm
- **WHEN** a client authenticates the WebSocket with a token not signed HS256, or without an expiry
- **THEN** the connection is not authenticated

#### Scenario: Typing into an unsubscribed channel
- **WHEN** a client sends a typing event for a channel it is not subscribed to
- **THEN** it receives an `ErrorEvent` 403 and nothing is broadcast

#### Scenario: Removed member's live feed
- **WHEN** a member is removed from a channel
- **THEN** their subscriptions to that channel are dropped on every hub instance
- **AND** they receive no further messages from it

#### Scenario: Create channel without create_channel
- **WHEN** a caller with no `create_channel` grant on the workspace Channels OA creates a channel
- **THEN** the request is denied and no graph node is written

### Requirement: Live events stay within their audience
Presence events SHALL be delivered only to sessions of the same tenant. Approval events SHALL
carry the acting user's `tenant_id`, the requester (`created_by`) and the approvers still pending
on the current step (`assignee_node_ids`), and SHALL be delivered only to those users and the
actor, within that tenant.

#### Scenario: Approval event in another tenant
- **WHEN** an approval event for tenant A is consumed
- **THEN** no session of tenant B receives it

### Requirement: Drive sharing and quota are guarded

| Operation | Op | Object |
|---|---|---|
| Create share | `share` (not `write`) | the shared item's node |
| Revoke share | `share`, or being the share's creator | the shared item's node |
| Update quota | `manage` | the workspace Mgmt OA |

#### Scenario: Caller creates a share without the share op
- **WHEN** a caller holding `write` but not `share` on the item (for example the grantee of a write share) creates a share
- **THEN** the request is denied and nothing is written

#### Scenario: Member revokes a share without the share op
- **WHEN** a member holding `write` but not `share` revokes a share they did not create
- **THEN** the request is denied and the share remains

#### Scenario: Creator revokes their own share
- **WHEN** the member who created a share revokes it, holding `write` but not `share`
- **THEN** the share is revoked

### Requirement: The approval audit trail is limited to those who can see the request
The audit trail of an approval request (`GET /api/approval/requests/:id/audit`, gRPC `GetAuditLog`)
SHALL be returned only to a caller who could see the request through the list endpoints: its
requester (`created_by`), a user assigned to any step of it, current or past and of any assignment
status, or a caller whose `read` scopes (the department-requests scope set) include the request's
`scope_oa_id`. Anyone else SHALL be denied (PermissionDenied / 403), and a request that does not
exist SHALL get the same answer, so the trail's existence is not revealed. A request id that is not
a UUID is a bad request (InvalidArgument / 400), not a server error. Looking up a caller's assignment
on a request is served by the index `(request_id, user_node_id)` on `approval_assignments`
(`provision_tenant_schema`, and migration 021 for existing tenants). A failure to resolve the caller's scopes SHALL deny, never grant. The caller comes from
the verified claims / request metadata, and the request is looked up in the caller's tenant schema.

#### Scenario: Requester, assignee, department scope
- **WHEN** the requester, a user assigned to any step of the request, or a caller whose scopes include the request's scope reads its audit trail
- **THEN** the entries are returned

#### Scenario: Unrelated caller
- **WHEN** a caller who is none of the above reads the audit trail
- **THEN** the request is denied and no entry is returned

#### Scenario: Body names another user
- **WHEN** the request body names a user holding a department scope and the metadata caller holds none
- **THEN** scopes are resolved for the metadata caller only and the result is empty

### Requirement: Asset reads and type administration are guarded

| Operation | Op | Object |
|---|---|---|
| List assets | `read` | each asset type OA (batch); unreadable types are filtered out, never a 403 for the list |
| Get / update / delete / transition one asset | `read`, `write`, `manage`, the transition's op | the OA of the asset's type — an asset has no node of its own (see `asset-authorization`) |
| List / get asset requests | — | visible to the requester, or to holders of `approve` on the request's type OA |
| Create asset type | `manage` | the workspace Assets OA; for the first type, before that OA exists, `manage` on the Mgmt OA |
| Update type schema | `manage` | the workspace Assets OA |
| Get / list types | `read` | the workspace Assets OA |

#### Scenario: Member lists assets
- **WHEN** a member with read on one asset type of two lists assets
- **THEN** only assets of the readable type are returned and `total` counts only those

#### Scenario: Unrelated caller gets a request
- **WHEN** a caller who neither created a request nor can approve its type fetches it
- **THEN** the request is denied with PermissionDenied

### Requirement: Caller identity on gRPC comes from metadata
The REST edge SHALL put the caller verified from the JWT (user id, NGAC node id, tenant id) on the
request context (`httputil.SetClaims`), and every gRPC client SHALL forward it as request metadata
(`grpcauth.ClientInterceptor`). Every gRPC server SHALL reject a request that carries no complete
caller with `Unauthenticated` (`grpcauth.ServerInterceptor`), except the methods its policy lists,
and handlers SHALL take the caller from `grpcauth.CallerFrom(ctx)` and never from a field of the
request body. The caller fields of the request messages (`user_ngac_node_id`, `requester_ngac_node_id`,
`inviter_ngac_node_id`, `current_owner_ngac_node_id`, `sender_*`, approval `user_node_id`, and the
`user_id` that accompanies them) are deprecated and ignored; where a body field and the metadata
disagree, the metadata wins. A request that carries no caller reaches only the methods below.

| Server | Method | Accepted without a user | Why |
|---|---|---|---|
| every server | `grpc.health.v1.Health/Check` | no identity at all | liveness and readiness probes |
| auth | `Register`, `Login`, `Signup`, `Signin` | no identity at all | the request carries the credentials that authenticate it |
| auth | `IsTokenRevoked` | a named service | token validity is checked from a bare jti |
| policy | `PolicyWrite/CreateNode`, `PolicyWrite/CreateAssignment`, `PolicyRead/FindNodeByName` | a named service | auth provisions user and tenant nodes during signup and sign-in, before any user is authenticated |

A named service is the `x-service-name` metadata a client interceptor adds when the context holds no
caller. It attributes the call; it is not a credential. Authenticating services to each other (mTLS or
an internal token) is not part of this requirement, and the gRPC ports are internal-only.

Where the auth service has itself just authenticated a user (password, Google token, OTP, or an
existing session) it calls workspace and messaging as that user (`grpcauth.WithCaller`), so those
servers authorize the new user like any other caller.

#### Scenario: Request without a caller
- **WHEN** a gRPC request reaches a server with no caller metadata and the method is not exempt
- **THEN** it is rejected with `Unauthenticated` and no handler runs

#### Scenario: Body and metadata name different users
- **WHEN** a request body names user A and the metadata names user B
- **THEN** the decision is taken for user B and A never reaches the policy check

#### Scenario: Asset RPCs without a caller field
- **WHEN** an authorized caller calls `GetType`, `ListTypes`, `GetRequest` (asset) or `UpdateQuota` (drive) over the network
- **THEN** the call succeeds, because the caller travels in metadata and not in a process-local context value

#### Scenario: Service identity on a guarded method
- **WHEN** a request carries only a service identity and the method is not in the table above
- **THEN** it is rejected with `Unauthenticated`

