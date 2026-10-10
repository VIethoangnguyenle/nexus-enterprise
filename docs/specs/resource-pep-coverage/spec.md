# resource-pep-coverage

## Purpose
Record which policy decision guards each resource operation outside workspace administration, so
that an endpoint without a check is visible as a gap instead of silently trusting the session.
Workspace administration is covered by `workspace-admin-authorization`.

## Requirements

### Requirement: Every PEP fails closed
Each check listed here SHALL treat an empty caller, an unresolvable OA, a policy-service error,
or any decision other than ALLOW as a denial. The caller SHALL come from verified JWT claims at
the REST edge and from the signed identity token between services (see "Caller identity on gRPC").

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
the verified claims / signed identity token, and the request is looked up in the caller's tenant schema.

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

### Requirement: Caller identity on gRPC is a signed token
The REST edge SHALL put the caller verified from the JWT (user id, NGAC node id, tenant id) on the
request context (`httputil.SetClaims`). Every gRPC client SHALL mint, per call, a signed identity
token (`grpcauth.ClientInterceptor`, `StreamClientInterceptor`) carrying that caller, or its own
service name when the context holds no caller, and send it in the one metadata key
`x-nexus-identity`. The token is `base64url(payload).base64url(HMAC-SHA256)` under
`INTERNAL_IDENTITY_SECRET`; the payload holds the user id, NGAC node id and tenant id (or the service
name), the audience (the full gRPC method), issued-at, an expiry about 60 seconds later, and a random
nonce.

Every gRPC server SHALL, before building the caller (`grpcauth.ServerInterceptor`,
`StreamServerInterceptor`), verify the signature in constant time, that the audience equals the
method being served, and that the token is neither expired nor issued in the future (5 seconds of
clock skew allowed; a lifetime over 2 minutes is refused). A request with no token, or a token that
fails any check, SHALL be rejected with `Unauthenticated`, except on the methods its policy exempts.
The retired `x-caller-*` and `x-service-name` keys SHALL be ignored. Handlers SHALL take the caller
from `grpcauth.CallerFrom(ctx)` and never from a field of the request body. The caller fields of the
request messages (`user_ngac_node_id`, `requester_ngac_node_id`, `inviter_ngac_node_id`,
`current_owner_ngac_node_id`, `sender_*`, approval `user_node_id`, and the `user_id` that
accompanies them) are deprecated and ignored; where a body field and the token disagree, the token
wins. A request that carries no user reaches only the methods below.

| Server | Method | Accepted without a user | Why |
|---|---|---|---|
| every server | `grpc.health.v1.Health/Check`, `Watch` | no identity at all | liveness and readiness probes |
| auth | `IsTokenRevoked` | a token naming the service `messaging` | token validity is checked from a bare jti |
| policy | `PolicyWrite/CreateNode`, `PolicyWrite/CreateAssignment`, `PolicyRead/FindNodeByName` | a token naming the service `auth` | auth provisions user and tenant nodes during signup and sign-in, before any user is authenticated |
| policy | `PolicyWrite/DeleteNode` | a token naming the service `auth` | auth removes the node it just created when signup fails part-way, so no orphan stays in the graph |

A service token is accepted only on a method listed above and only from a service on that method's
allowlist (`grpcauth.ServiceOnly`); a valid token for any other service, or on any other method, is
`Unauthenticated`.

The secret is `INTERNAL_IDENTITY_SECRET`, read through `bootstrap.ConfigureInternalIdentity` at the
start of every service's `main`. Outside a development `APP_ENV` (`dev`, `development`, `local`,
`test`) it is required, at least 32 bytes, not the committed placeholder, and different from
`JWT_SECRET` (as must `INTERNAL_IDENTITY_SECRET_PREVIOUS`); a service that cannot satisfy this does not start. A process that never configured it
refuses every call in both directions, except the exempt health probes, which stay answerable.
`INTERNAL_IDENTITY_SECRET_PREVIOUS`, when set, is accepted for verification only. A rotation takes
three deploys, because a deploy replaces containers one at a time and a container that signs with a
secret its peers cannot yet verify is refused: (1) `PREVIOUS` = the new value, current = the old
(everything still signs with the old and now accepts both); (2) swap them (everything signs with the
new and still accepts the old); (3) drop `PREVIOUS`. After a suspected leak skip `PREVIOUS`, set the
new secret and accept a short burst of `Unauthenticated` during the single deploy.

Where the auth service has itself just authenticated a user (Google token, OTP, or an existing
session) it calls workspace and messaging as that user (`grpcauth.WithCaller`), so those servers
authorize the new user like any other caller.

Threat model. Protected: a party that can reach a gRPC port but does not hold the secret cannot
claim any user or service, cannot alter a captured token (it is signed), and cannot replay one on
another method or after it expires. Not protected, and accepted: (1) every service holds the same
symmetric secret, so the per-method service allowlists limit mistakes but do not authenticate one
service to another: compromising any one service, or stealing the secret, lets the holder mint any
user or service identity, and rotation is the remedy; (2) a captured token can be replayed on the
*same* method until it expires, because there is no replay cache (the nonce is for logging); (3) the
transport is plaintext, so the gRPC ports must stay unreachable from outside the platform. That
isolation comes from the Docker network layout, not from the nginx edge: the Go services sit only on
the internal `nexus` network and publish no ports, which `deploy/check-compose-env.sh` asserts. This
control is defence in depth behind that isolation, not a replacement for it. Health probes are
exempt from verification and answer even when no secret is configured.

#### Scenario: Request without a token
- **WHEN** a gRPC request reaches a server with no `x-nexus-identity` and the method is not exempt
- **THEN** it is rejected with `Unauthenticated` and no handler runs

#### Scenario: Retired caller metadata
- **WHEN** a request carries `x-caller-user-id`, `x-caller-ngac-node-id`, `x-caller-tenant-id` or `x-service-name` and no valid token
- **THEN** it is rejected with `Unauthenticated`

#### Scenario: Altered, expired or foreign token
- **WHEN** a token's payload or signature was altered, it is past its expiry plus skew, or it was signed with another secret
- **THEN** the request is rejected with `Unauthenticated`

#### Scenario: Token replayed on another method
- **WHEN** a token minted for one gRPC method is sent to a different method
- **THEN** the request is rejected with `Unauthenticated`

#### Scenario: Rotation window
- **WHEN** a token was signed with the previous secret and the verifier has it as `INTERNAL_IDENTITY_SECRET_PREVIOUS`
- **THEN** the token is accepted
- **AND** every token a service mints is signed with the current secret
- **AND** a rotation therefore sets the new value as `PREVIOUS` first (all signers still use the old), then swaps, then drops `PREVIOUS`; a single-step swap would have a recreated container signing with a secret its not-yet-restarted peers cannot verify

#### Scenario: Oversized or misshapen token
- **WHEN** a token is longer than 1024 bytes, or its signature part is not the 43 characters of a base64url SHA-256 MAC, or request headers exceed 16 KiB
- **THEN** it is refused before any decoding or signature work

#### Scenario: Body and token name different users
- **WHEN** a request body names user A and the token names user B
- **THEN** the decision is taken for user B and A never reaches the policy check

#### Scenario: Asset RPCs without a caller field
- **WHEN** an authorized caller calls `GetType`, `ListTypes`, `GetRequest` (asset) or `UpdateQuota` (drive) over the network
- **THEN** the call succeeds, because the caller travels in the signed token and not in a process-local context value

#### Scenario: Service identity on a guarded method
- **WHEN** a request carries only a valid service token and the method is not in the table above, or the service is not on that method's allowlist
- **THEN** it is rejected with `Unauthenticated`

#### Scenario: Missing secret outside development
- **WHEN** a service starts with `APP_ENV=production` and no `INTERNAL_IDENTITY_SECRET`, the placeholder, or the value of `JWT_SECRET`
- **THEN** it logs the reason and exits without serving

### Requirement: The admin screens' endpoints are guarded
All on the workspace service, with the caller from verified claims; `Mgmt OA` is `ngac.MgmtOAName(wsID)`.

| Endpoint | Op | Object |
|---|---|---|
| `GET /workspaces/:id/permission-areas` | membership | the workspace PC |
| `GET /workspaces/:id/roles` | membership | the workspace PC |
| `POST`, `DELETE /workspaces/:id/roles[/:roleId]` | `manage` | Mgmt OA |
| `GET /workspaces/:id/roles/:roleId` | `manage` | Mgmt OA |
| `PUT /workspaces/:id/roles/:roleId/permissions/:area` | `manage`, plus each added operation | Mgmt OA, then the area's OA |
| `GET /workspaces/:id/admin/members` | `manage` | Mgmt OA |
| `POST /workspaces/:id/members` (email) | `invite`; a role or department attached also needs `manage` and what it confers | Mgmt OA, then each OA the role or department (and its ancestors) is associated with |
| `GET`, `DELETE /workspaces/:id/invitations[/:invitationId]` | `invite` | Mgmt OA |
| `GET /invitations`, `POST /invitations/:id/accept`, `POST /invitations/:id/decline` | the caller's own account address; accepting needs the inviter to still hold `invite` | Mgmt OA (as the inviter) |
| `PUT`, `DELETE /workspaces/:id/members/:nodeId/roles/:roleId` | `manage`, plus what the role confers when assigning | Mgmt OA, then each OA the role is associated with |
| `PUT /workspaces/:id/members/:nodeId/department` | `manage`, plus what the department confers | Mgmt OA, then each OA the department is associated with |
| `DELETE /workspaces/:id/members/:nodeId` | `invite`; an owner also needs `manage` and the caller to be an Owner | Mgmt OA, Owners UA |
| `POST /workspaces/:id/permissions` | removed; there is no free-form grant | none |
| `POST /workspaces/:id/invite` | removed; nothing adds a person by node ID | none |

#### Scenario: Member calls an admin endpoint
- **WHEN** a workspace member without `manage` calls any `manage` endpoint above
- **THEN** the answer is 403 and nothing is read or written beyond the check


### Requirement: Text documents and workspace details are guarded
Text documents follow the table in `text-documents` (read or write on the OA of the folder the document sits in). The workspace's own details are on the workspace service, with the caller from verified claims.

| Endpoint | Op | Object |
|---|---|---|
| `GET /workspaces/:id/details` | membership | the workspace PC (`can_manage` is reported from `manage` on the Mgmt OA) |
| `PATCH /workspaces/:id/details` | `manage` | Mgmt OA |
| `POST /workspaces/:id/leave` | membership; the person is always the caller | the workspace PC (the Owners UA is read under the owner lock) |
| `POST /workspaces` | removed (410): creation is `POST /api/me/workspaces` on the auth service | none |

#### Scenario: Member renames the workspace
- **WHEN** a member without `manage`, or a person of another workspace, calls `PATCH /workspaces/:id/details`
- **THEN** the answer is 403 and the name is unchanged

#### Scenario: Unwritable save
- **WHEN** the name is empty, longer than 100 characters or contains NUL, or the description is longer than 500 characters
- **THEN** the answer is 400 and nothing is written

#### Scenario: Leaving
- **WHEN** a member calls `POST /workspaces/:id/leave`
- **THEN** they are detached from every attribute of the workspace through the policy writer, their listing is removed and any invitation open to their verified address is revoked
- **AND** nobody else is touched, whatever the body says

#### Scenario: Leaving without belonging, or when the policy cannot answer
- **WHEN** the caller is not in the workspace, has no identity, or the policy call fails
- **THEN** the answer is 403 and nothing is written

#### Scenario: The last Owner
- **WHEN** the only Owner calls leave
- **THEN** the answer is 409 with `reason: "last_owner"` and nothing is detached; the Owners are read and the removal done under the workspace's owner lock, so two Owners leaving together cannot both be the other one

#### Scenario: Editing two fields at once
- **WHEN** one manager sets the name while another sets the description
- **THEN** both land, because each is a single statement that sets only the fields it names
