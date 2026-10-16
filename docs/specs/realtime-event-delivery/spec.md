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
The hub SHALL stamp each workspace-level event with the workspace's next sequence number, monotonic per workspace (taken in Redis together with the log append when present, an in-process counter otherwise). Events addressed to a channel or to users carry sequence 0 in the workspace stream: not every subscriber receives them, so their numbers could not be checked for holes. User-addressed events have their own per-user numbers (see "Missed events are replayed from a log"); channel-addressed events have none.

#### Scenario: Numbering
- **WHEN** five workspace events are published for one workspace and one for another
- **THEN** the first workspace's events carry 1..5 and the other's carries 1

#### Scenario: Dropped events consume no number
- **WHEN** an event fails validation
- **THEN** it is not delivered and no sequence number is spent

### Requirement: The screen resynchronises only when it cannot be caught up
The frontend SHALL track the last sequence number per followed workspace. A repeat is ignored; the next number is applied. After any reconnect the frontend SHALL re-authenticate and ask to be caught up from its last numbers (workspace and user stream, see "Missed events are replayed from a log"); it refreshes every query family of the workspace only when the answer is `too_far`, when it holds no number yet, or when a hole or a number lower than the last (the counter was lost) shows up in live frames that replay cannot repair.

#### Scenario: A hole
- **WHEN** a frame arrives with a number more than one above the last and a replay from the last number answers `too_far`
- **THEN** all workspace queries are refreshed

#### Scenario: Thirty seconds offline
- **WHEN** the connection drops for 30 seconds and returns
- **THEN** the session reconnects on its own, the missed frames are replayed in order and applied as ordinary events, "synced" is received, and no query family is refreshed wholesale

### Requirement: Missed events are replayed from a log
Every numbered event SHALL be appended to a Redis Stream in the same atomic step that takes its number, so a number never exists without its frame:

- workspace events: stream `rt:ws:<workspace>`, counter `wsseq:<workspace>`;
- user-addressed events (the `permission` domain and every other event naming user nodes): stream `rt:user:<tenant>:<userNode>`, counter `useq:<tenant>:<userNode>`, one number per addressed user per event.

Each stream is capped at 10 000 entries (approximate trim), entries older than 24 hours are trimmed as the stream grows (every 50th append), and an idle stream expires after 24 hours. Entries hold the frame as built before its number was known (ids only, never names or bodies); the number is the stream ID `<n>-0`. Channel-addressed events are not in any stream. Without Redis nothing is logged and nothing is replayed.

A session SHALL ask for a replay with JSON text frames, beside the binary protobuf frames (the protobuf schema is unchanged):

- client to server `{"type":"sync","workspace_id":"<ws>","since":<seq>}` follows the workspace and replays after `<seq>`;
- client to server `{"type":"user_sync","since":<useq>}` replays the session's own user stream;
- server to client `{"type":"synced","scope":"workspace"|"user","workspace_id":"<ws>","seq":<n>,"replayed":<k>}` after the missed frames, in order, as the ordinary binary frames (byte-identical to the live frames); `seq` is the cursor to continue from;
- server to client `{"type":"too_far","scope":…,"workspace_id":…,"seq":<n>}` when the log cannot cover the gap: no Redis, `since` above the counter (counter lost), more than 2 000 frames behind, or entries already trimmed. Nothing is replayed; `seq` is the cursor to restart from after refreshing;
- server to client `{"type":"user_seq","seq":<n>}` after each live user-addressed frame that has a number, so the session keeps its cursor. User-addressed frames themselves keep `seq` 0. A frame and its `user_seq` SHALL be queued for a session together or not at all: when its send queue has no room for both, both are dropped, so the number never arrives without its frame and the session finds the hole at the next `user_seq` and syncs. A user's number is taken and published under that user's own ordering, never under the workspace sequence lock, so a slow append for one recipient does not delay workspace events or other users.

A `sync` without `since`, an unknown type, or any control frame before authentication is answered with an error (400, 401). A session that sends no `since` (the binary subscribe) behaves as before.

Authorisation comes first and is unchanged: the workspace must be the session's tenant and the user must hold `read` on its Documents OA, decided by the policy service, fail-closed. A refused `sync` is answered exactly like a refused subscribe (403 and a `denied` acknowledgement) and replays nothing. The user stream is addressed only by the session's verified tenant and node, never by a client-supplied id, so no other user or tenant can read it. A stored frame whose tenant differs from the session's is never replayed. Live frames of the workspace arriving during a replay are held and delivered after it, so the session sees one ordered sequence. The held frames are released with the same bounded wait as the replay, in order, and frames arriving while they are released queue behind them; they are dropped only when the session stops reading.

#### Scenario: Back within the window
- **WHEN** a session that last saw workspace number 2 syncs while the workspace is at 5
- **THEN** it receives frames 3, 4, 5 in order, then `synced` with `seq` 5, and the next live event is 6

#### Scenario: Outside the window
- **WHEN** the session is more than 2 000 frames behind, or its cursor is above the counter, or the frames were trimmed
- **THEN** it receives `too_far` and no frame

#### Scenario: No read on the workspace
- **WHEN** a session without `read` syncs a workspace whose stream holds frames
- **THEN** it receives a 403 and a `denied` acknowledgement, no frame, no `synced`, and nothing afterwards

#### Scenario: Another person's user stream
- **WHEN** a user syncs the user stream, even naming another node, or the same node id in another tenant syncs it
- **THEN** each receives only the stream of their own tenant and node

### Requirement: The event log is observable and bounded
Each hub instance SHALL log, once a minute, the number of event streams it has written, their total entries and the longest (`realtime event log`), and publish the same as the expvar `realtime_stream_len`.

Memory budget: an entry is a protobuf frame of about 60 to 120 bytes plus about 100 bytes of stream overhead, so a stream at its 10 000 cap is about 2 MB, and a typical one (hundreds of events a day) tens to a few hundred KB. A workspace stream exists only for a workspace with traffic in the last 24 hours; a user stream only for a user addressed in the last 24 hours. The worst case is therefore 2 MB times the number of active workspaces and users (1 000 active streams: about 2 GB at the cap, in practice well under 200 MB); the cap lowers if the metric shows otherwise. Counters (`wsseq:`, `useq:`) are a few bytes per workspace or user and do not expire.

### Requirement: Events carry ids, not content
A frame SHALL contain the domain, the kind of change, the tenant and workspace, the ids the change touched, the containing folder and the folder an item left (drive), the acting user's id and the sequence number. It SHALL NOT contain names, titles, bodies or any field a viewer might not be allowed to read. The frontend maps each frame to the query keys it makes stale through the key factories and refetches. Chat messages, reactions, pins, polls and tasks keep patching the cache directly.

#### Scenario: Drive item moved
- **WHEN** a frame says an item moved from folder A to folder B
- **THEN** the listings of both folders and the item's detail are refreshed, and the quota is not

### Requirement: Thread replies carry a summary; thread state goes to its owner only
A thread reply SHALL reach the channel's subscribers as a `thread_reply` frame carrying the reply and the thread's
new summary: reply count, last reply time and the users of the three most recent distinct repliers — the same for
every viewer, and nothing a channel reader could not already see. Each user who follows the thread SHALL receive a
`thread_state` frame with their own follow state and unread count on the user-addressed route: every session of
that user's NGAC node in the channel's workspace (for a direct message, the tenant the reply was sent from), on
every hub instance, and no other session. Following is not access: each recipient SHALL be checked for `read` on the
channel's content OA when the frame is built, and one who fails the check — a mentioned non-member, a member since
removed — or whose check cannot be answered SHALL receive nothing. Each state carries the time it was read
(`as_of`); a client keeps the newest per thread, and ignores a summary whose reply count is below the one shown.
The frames are built after the reply commits and off the request path; a reply whose summary cannot be built is
still delivered, without it. A follow, unfollow
or read of a thread SHALL send the caller's new state the same way, so their other devices agree within a second.
A state without a tenant SHALL NOT be sent. Root messages keep their `chat_message` frame.

#### Scenario: A reply in a followed thread
- **WHEN** B replies to a thread A follows while A and C both watch the channel
- **THEN** A and C see the new summary; only A's sessions receive A's unread count; a session of the same node id
  in another tenant receives nothing

#### Scenario: Mentioned without access
- **WHEN** a reply mentions D, who cannot read the channel
- **THEN** D receives no `thread_state` for it, now or on later replies

#### Scenario: Read on the phone
- **WHEN** A opens the thread on their phone
- **THEN** A's laptop shows the thread with nothing unread

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

### Requirement: A cached read is revalidated by a version that moves with the events
A read a client may keep (today only `GET /api/workspaces/:id/chat-state`) SHALL carry an `ETag` and answer `304 Not Modified`, without reading the store or the policy service, to an `If-None-Match` that names it. The tag is derived from Redis counters, the validity window and the request URI. Counters are named by every scope the payload varies by: the tenant (`ver:chat:<tenant>`) and, for what belongs to the person across workspaces (read marks, stars, levels, sections, follows, direct messages), the user (`ver:chat:user:<user>`). A counter SHALL NOT be shared between tenants, nor the per-user one between users, so a tag earned by one person or workspace never matches another's.

A counter SHALL be bumped by the code that announces the change, not by a separate step a write path could omit:
- `Service.emit` in the messaging domain (every channel event), and the sites that change a conversation's content without an event (`SendMessage`, edit, delete, a call card rewritten, a new direct message), bump the tenant, and the people of a direct message individually, since a direct message is read from every workspace of its people;
- the messaging REST group bumps the tenant and the caller after any request that changed something and succeeded, just before the status line is written, so the client's next read sees the new version; a refused or failed request bumps nothing;
- the realtime consumer bumps the tenant for `channel`, `workspace` and `permission` events announced by other services, a permission event only after the settle delay at which clients are told.

The tag is computed before the data is read, so data read across a concurrent write is labelled with the older version and the next request refetches. A tag SHALL expire after five minutes even if nothing moved, so a forgotten bump costs a stale list for that long, never indefinitely. Without Redis, or when the counters cannot be read, no `ETag` is sent and every answer is a full 200. An answer other than 200 carries no `ETag`.

`/api/workspaces/:id/contacts`, `/api/me/workspaces` and `/api/workspaces` are not revalidated this way: their data changes through paths that announce nothing (presence in `is_online`, profile edits, invitation acceptance, role and membership changes across the auth and workspace services, graph-derived reachability), so a counter could not be guaranteed to move with every write.

#### Scenario: Nothing changed
- **WHEN** a client repeats `GET /api/workspaces/:id/chat-state` with the `ETag` it was given and nothing was announced
- **THEN** the answer is 304 with no body and neither the store nor the policy service is asked

#### Scenario: A change was announced
- **WHEN** a message is sent, a mark-read succeeds, or a permission event settles
- **THEN** the next `If-None-Match` with the old tag gets a full 200 with a new tag

#### Scenario: Another person or workspace
- **WHEN** a tag is presented by a different user, for a different workspace, or for a workspace that is not the caller's own tenant
- **THEN** it does not match and the request is judged as any other

## Event catalogue

| Domain | Kinds | Ids | Audience |
|---|---|---|---|
| `drive` | `created`, `updated`, `moved`, `deleted`, `share_created`, `share_revoked` | the item | workspace; a share only refreshes the share list and the item, the people it concerns learn their permissions from the permission event |
| `document` | `created`, `updated`, `deleted` | the document (list level only) | workspace |
| `channel` | `created`, `renamed`, `member_added`, `member_removed` | the channel | user (created: the people who can read it at birth, the creator and any DM participant), channel (renamed, roster), user (the person added or removed) |
| `workspace` | `invitation_accepted`, `member_removed`, `role_changed`, `department_changed`, `updated` | member node, role, department or workspace | workspace |
| `asset` | `updated`, `request_changed`, `assigned` | asset or request | workspace |
| `permission` | `changed` | none | user (the users whose own standing the write changed; see "Permission events are narrow") |
| approval (`ApprovalEvent`) | created, approved, rejected, step_advanced | the request | the people the request names |
| call (`CallEvent`, envelope field 23) | `ring`, `cancel`, `answered`, `declined`, `missed`, `state`, `invited` | the call, its conversation, who is in it | both people of a direct call on every session (ringing), the conversation's subscribers (`state`); see `chat-calls` |

There is no channel "archive" in the system, so there is no archive event.

## Delivery guarantee

Delivery is best effort with resynchronisation, not an outbox. A service that dies between commit and produce loses that one event; a hub that is down or a session whose buffer is full misses it. Workspace-level events expose such a loss as a sequence hole on the next event, and a reconnect replays what the log holds or refreshes everything when it cannot, so a screen is stale for at most the time to the next event of its workspace or the next reconnect. An outbox would close the remaining window (a quiet workspace whose only event was lost) at the cost of a table and a relay in every producing service; the system does not need that guarantee because every event is a hint to refetch, not data.

Asset events are keyed by workspace, so one workspace's events share a partition and keep their order.

Events older than 30 seconds are dropped by the consumer, and a new consumer group starts at the end of each topic, so a restart replays no history.

## Known limits
- Channel-addressed events are not numbered or logged; a miss there is repaired by the next reconnect (chat state is synchronised by its own ETag) or the next workspace event that exposes a hole.
- The log is best effort like the events themselves: an event lost between commit and the hub is in no stream. A replay is complete only relative to what the hub numbered.
- A workspace event names the ids of items a viewer may not be able to read. It carries no name or content, and the refetch is authorized as the viewer.
- A session whose user holds no `read` on the Documents OA cannot follow a workspace even if they hold rights elsewhere.
