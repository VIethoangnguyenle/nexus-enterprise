# realtime-event-delivery

## Purpose
Show every change somebody else makes on the screen that is already open, without a refresh, and never show it to anyone outside the tenant or the audience it concerns. Events say that something changed and which ids it touched; the screen refetches under its own authorization.

## Requirements

### Requirement: Events are announced after the change commits
A service that changes state SHALL emit one event per committed change onto the Redpanda topic `<domain>.events` (domains: `drive`, `channel`, `workspace`, `document`, `permission`; assets keep `asset.lifecycle`, `asset.request`, `asset.assignment`; approvals keep `approval.events`). It SHALL emit only after the change is visible to readers, and SHALL NOT emit when the request is refused, fails, or is rolled back. The request path never calls the hub.

Every event SHALL carry `tenant_id` and `workspace_id`, taken from the verified caller or the stored row, never from the request body. An event without either is dropped by the producer.

#### Scenario: A refused or failed change
- **WHEN** a request is denied, invalid, conflicting, or its transaction rolls back
- **THEN** no event is produced

#### Scenario: A committed change
- **WHEN** a change commits
- **THEN** exactly one event for it is produced, and a reader that queries when the event is emitted already sees the new state

#### Scenario: The broker is down
- **WHEN** Redpanda cannot be reached
- **THEN** the request still succeeds and returns promptly: producers give each record 10 seconds, buffer a bounded number, and drop events past that instead of blocking; the event is lost and clients recover through resynchronisation

### Requirement: Events fan out to one of three audiences
The messaging consumer SHALL hand each event to the hub, which delivers it at the level the event is addressed to, always inside the event's tenant:

- **user**: the sessions of the named users (`user_node_ids`), and nobody else;
- **channel**: the sessions subscribed to that channel;
- **workspace**: the sessions subscribed to that workspace.

An event never reaches a session of another tenant, a session that did not subscribe to the audience, or (at user level) another user of the same tenant. Approval events keep their own targeted delivery: to the people they name, inside their tenant.

#### Scenario: Another tenant
- **WHEN** an event of tenant A is published and a session of tenant B has subscribed to the same workspace id
- **THEN** that session receives nothing

#### Scenario: Not subscribed
- **WHEN** a workspace event is published and a session of the same tenant has not subscribed to that workspace
- **THEN** that session receives nothing

#### Scenario: Addressed to one person
- **WHEN** a permission event names one user node
- **THEN** every session of that user in the tenant receives it, and no other session does

### Requirement: Following a workspace needs read on it
A session SHALL subscribe to a workspace explicitly. The subscription SHALL be allowed only when the workspace is the session's own tenant and the user holds `read` on the workspace's Documents OA, decided by the policy service as that user. Any failure to decide is a refusal. A grant is acknowledged with the last sequence number issued so far.

#### Scenario: A subscribe without read
- **WHEN** a session subscribes to a workspace on which its user has no `read`
- **THEN** it receives an error with code 403 and no event from that workspace afterwards

#### Scenario: A subscribe to another tenant's workspace
- **WHEN** the workspace is not the session's tenant
- **THEN** the subscription is refused before the policy service is asked

A refused subscription is answered with an acknowledgement marked `denied`, in addition to the 403 error, so a client that waits for the answer is released. The client then fetches what it missed (chat and notifications do not depend on the workspace stream) and asks again after 5, 15 and 45 seconds, since a refusal can be a policy replica a step behind.

### Requirement: A subscription ends when its justification does
A subscription was authorized when it opened and SHALL NOT outlive what justified it:

- When a member is removed or leaves, the hub SHALL drop, on every instance, that user's workspace subscription and every channel subscription held in the workspace's tenant, before anyone is told they left (the workspace `member_removed` event drives it).
- When a permission event names a user, the hub SHALL ask the policy service again about each workspace that user's sessions follow and drop the ones that no longer pass.
- A session SHALL be closed at the expiry of the token it authenticated with; the client reconnects with its refreshed token, and a client that cannot has lost its right to listen.

#### Scenario: Removed while connected
- **WHEN** a member is removed from the workspace while their tab is open
- **THEN** workspace and channel frames of that workspace stop reaching their sessions at once, and colleagues keep receiving them

#### Scenario: Read taken away
- **WHEN** a user's read on the workspace is revoked and the permission event reaches the hub
- **THEN** their next workspace subscription check fails and they receive no further workspace frames, while the socket stays open

#### Scenario: Token expiry
- **WHEN** a session's token expires
- **THEN** the server closes the connection

### Requirement: Workspace events are numbered
The hub SHALL stamp each workspace-level event with the workspace's next sequence number, monotonic per workspace (Redis `INCR` when present, an in-process counter otherwise). Events addressed to a channel or to users carry sequence 0 and are outside the stream: not every subscriber receives them, so their numbers could not be checked for holes.

#### Scenario: Numbering
- **WHEN** five workspace events are published for one workspace and one for another
- **THEN** the first workspace's events carry 1..5 and the other's carries 1

#### Scenario: Dropped events consume no number
- **WHEN** an event fails validation
- **THEN** it is not delivered and no sequence number is spent

### Requirement: The screen resynchronises when it cannot trust what it has
The frontend SHALL track the last sequence number per followed workspace. A repeat is ignored; the next number is applied; a hole, or a number lower than the last (the counter was lost), SHALL refresh every query family of the workspace. After any reconnect the frontend SHALL re-authenticate, re-subscribe, and refresh every query family once the subscription is acknowledged, so that nothing published between the refetch and the subscription is missed.

#### Scenario: A hole
- **WHEN** a frame arrives with a number more than one above the last
- **THEN** all workspace queries are refreshed

#### Scenario: Thirty seconds offline
- **WHEN** the connection drops for 30 seconds and returns
- **THEN** the session reconnects on its own, the subscription is acknowledged, every query family is refreshed, and the next event in order is applied as an ordinary event

### Requirement: Events carry ids, not content
A frame SHALL contain the domain, the kind of change, the tenant and workspace, the ids the change touched, the containing folder and the folder an item left (drive), the acting user's id and the sequence number. It SHALL NOT contain names, titles, bodies or any field a viewer might not be allowed to read. The frontend maps each frame to the query keys it makes stale through the key factories and refetches. Chat messages, reactions, pins, polls and tasks keep patching the cache directly.

#### Scenario: Drive item moved
- **WHEN** a frame says an item moved from folder A to folder B
- **THEN** the listings of both folders and the item's detail are refreshed, and the quota is not

### Requirement: Permission events are narrow and wait for replicas
A permission event SHALL name only the users whose own standing a write changed: for an assignment, the users at or beneath the child (never the parent's other members); for an association, the users beneath the user attribute; for a prohibition, the users beneath its subject; for a deleted node, the users it held. An object attribute has no audience: a folder created, moved or deleted, or a channel created, reaches people through its own event, and the permission cache is dropped on a move.

Policy read replicas apply a graph change from their own feed, separately from this event. To keep the refetch an event triggers from reading the graph as it was, the consumer SHALL hold a permission event for a short settle time (750 ms) before delivering it, and the frontend SHALL repeat the refresh once more after 2 seconds. A replica later than that is repaired by the next event or reconnect.

#### Scenario: Someone creates a folder
- **WHEN** a member creates a folder
- **THEN** subscribers receive the drive `created` event and no permission event

#### Scenario: One person joins a role
- **WHEN** a user is assigned to a role that others already hold
- **THEN** only that user receives a permission event

### Requirement: Invalidations are batched
The frontend SHALL hold the invalidations of events for one frame (100 ms) and run each distinct key once, so a burst of events costs one refetch per affected query.

#### Scenario: A burst
- **WHEN** ten events touching the same folder arrive within a frame
- **THEN** that folder's listing is invalidated once

### Requirement: Presence is tenant-scoped and does not flicker
Presence SHALL reach only sessions of the user's own tenant. A user's last session leaving SHALL be announced as offline only after a grace period (3 seconds), cancelled by a new session of the same user in the tenant. The frontend SHALL apply presence changes in one batch per frame.

#### Scenario: A reload
- **WHEN** a user's only session closes and a new one opens inside the grace period
- **THEN** colleagues receive no offline and no second online

### Requirement: A change by somebody else is shown on the row
When a list refetches after an event whose actor is somebody other than the signed-in user, the changed row SHALL be washed in the actor's person hue for the realtime duration and tagged "vừa cập nhật", as the drive, approval and asset tables do. A change by the signed-in user, or by an unknown actor, gets no wash. Reduced motion keeps the wash (a change of colour) and drops the flash.

#### Scenario: Another person renames a document
- **WHEN** a document's title changes through another user and the list refetches
- **THEN** its row is washed in that user's hue and then settles

## Event catalogue

| Domain | Kinds | Ids | Audience |
|---|---|---|---|
| `drive` | `created`, `updated`, `moved`, `deleted`, `share_created`, `share_revoked` | the item | workspace; a share only refreshes the share list and the item, the people it concerns learn their permissions from the permission event |
| `document` | `created`, `updated`, `deleted` | the document (list level only) | workspace |
| `channel` | `created`, `renamed`, `member_added`, `member_removed` | the channel | user (created: the people who can read it at birth, the creator and any DM participant), channel (renamed, roster), user (the person added or removed) |
| `workspace` | `invitation_accepted`, `member_removed`, `role_changed`, `department_changed`, `updated` | member node, role, department or workspace | workspace |
| `workspace` (not for browsers) | `invitation_created` | the invitation | none: the notification consumer reads it to tell the invited account; the hub drops it, because members are not told who has been invited |
| `asset` | `updated`, `request_changed`, `assigned` | asset or request | workspace |
| `permission` | `changed` | none | user (the users whose own standing the write changed; see "Permission events are narrow") |
| notification (`NotificationEvent` frame) | one per stored notification | the notification | the recipient's sessions in the notification's workspace only; a personal notification (a workspace invitation) reaches every session of the recipient. It carries the type, actor id and name, target type, id and name, workspace, params and creation time, never pre-built text |
| approval (`ApprovalEvent`) | created, approved, rejected, step_advanced | the request | the people the request names |

There is no channel "archive" in the system, so there is no archive event.

## Delivery guarantee

Delivery is best effort with resynchronisation, not an outbox. A service that dies between commit and produce loses that one event; a hub that is down or a session whose buffer is full misses it. Workspace-level events expose such a loss as a sequence hole on the next event, and every reconnect refreshes everything, so a screen is stale for at most the time to the next event of its workspace or the next reconnect. An outbox would close the remaining window (a quiet workspace whose only event was lost) at the cost of a table and a relay in every producing service; the system does not need that guarantee because every event is a hint to refetch, not data.

Asset events are keyed by workspace, so one workspace's events share a partition and keep their order.

Events older than 30 seconds are dropped by the consumer, and a new consumer group starts at the end of each topic, so a restart replays no history.

## Known limits
- Channel- and user-addressed events are not numbered; a miss there is repaired by the next reconnect or the next workspace event that exposes a hole.
- A workspace event names the ids of items a viewer may not be able to read. It carries no name or content, and the refetch is authorized as the viewer.
- A session whose user holds no `read` on the Documents OA cannot follow a workspace even if they hold rights elsewhere.
