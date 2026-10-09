# resource-pep-coverage

## Purpose
Record which policy decision guards each resource operation outside workspace administration, so
that an endpoint without a check is visible as a gap instead of silently trusting the session.
Workspace administration is covered by `workspace-admin-authorization`.

## Requirements

### Requirement: Every PEP fails closed
Each check listed here SHALL treat an empty caller, an unresolvable OA, a policy-service error,
or any decision other than ALLOW as a denial. The caller SHALL come from verified JWT claims.

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
| Revoke share | `share`, or being the share's creator | the shared item's node |
| Update quota | `manage` | the workspace Mgmt OA |

#### Scenario: Member revokes a share without the share op
- **WHEN** a member holding `write` but not `share` revokes a share they did not create
- **THEN** the request is denied and the share remains

#### Scenario: Creator revokes their own share
- **WHEN** the member who created a share revokes it, holding `write` but not `share`
- **THEN** the share is revoked

### Requirement: Asset reads and type administration are guarded

| Operation | Op | Object |
|---|---|---|
| List assets | `read` | each asset type OA (batch); unreadable types are filtered out, never a 403 for the list |
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
