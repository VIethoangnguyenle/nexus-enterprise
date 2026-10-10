# notifications

## Purpose
A person's notifications are listed, counted and marked read over REST by the messaging service, the same data the gRPC notification service serves and the WebSocket pushes live. Each notification records facts (who did what to which thing, in which workspace); the screen words them. Design source: `design/mockups/notifications.html`.

## Requirements

### Requirement: Notifications belong to a workspace and are listed for the open one
`GET /api/notifications` SHALL return the signed-in person's notifications **in the workspace of their token**, newest first, as `{"notifications": [...], "total", "unread_count"}`. `limit` (default 25, at most 50; anything else becomes 25) and `offset` (default 0) page the list. Each notification carries `id`, `type`, `read`, `created_at`, `workspace_id`, `params` (an object, `{}` when empty) and, where they apply, `actor_user_id`, `actor_name`, `target_type`, `target_id` and `target_name`. It carries no pre-built `title` or `body`. An empty list is `[]`, not null. `GET /api/notifications/unread-count` SHALL return `{"count"}` for the same scope. A token with no workspace is refused 403 and nothing is read.

#### Scenario: A page
- **WHEN** a person with 30 notifications in the open workspace asks for `limit=10&offset=10`
- **THEN** they get the second ten, `total` is 30 and `unread_count` counts all their unread notifications in that workspace

#### Scenario: Nobody else's, no other workspace's
- **WHEN** two people have notifications, or one person has notifications in two workspaces
- **THEN** each list, count and total covers only the caller's own in the workspace they have open

#### Scenario: Notifications from before workspaces
- **WHEN** a notification row has no workspace (it predates migration 037)
- **THEN** it is in no workspace's list, count or mark. It carried only English text and no actor or subject names, so it could not be shown by the rules below; the row is kept, not deleted.

### Requirement: Names are the workspace's, never an id
`actor_name` and `target_name` SHALL be the names recorded when the notification was raised: a person is named by their display name, else their login, and only if they are a member of the notification's workspace; a subject by the name its event carries (an approval's template, an asset's or request type's name). A person who is not a member, or a subject whose event carries no name, is **absent** (empty), never replaced by an id. Recipients are resolved inside the event's workspace: an event without a tenant, or naming a recipient who is not a member of it, stores nothing.

#### Scenario: A person who left
- **WHEN** the decider has since left the workspace
- **THEN** the notification still shows the name recorded at the time

### Requirement: What is notified
The consumer SHALL raise exactly these types, each for the person the event addresses, naming the actor:

| type | recipient | about (`target_type`) | notes |
|---|---|---|---|
| `approval_approved` | requester | `approval` | the request is complete (status no longer pending) |
| `approval_step_approved` | requester | `approval` | a step passed, the request continues |
| `approval_rejected` | requester | `approval` | `params.reason` holds the comment, when there is one |
| `asset_request_approved` / `asset_request_rejected` | requester | `asset_request` | actor is the approver |
| `asset_assigned` | the new holder | `asset` | |
| `asset_returned` | the previous holder | `asset` | |
| `workspace_invitation` | an existing account with a verified email matching the invited address | `workspace_invitation` (id: the invitation, name: the workspace) | personal, see below |

An approval event names people by NGAC node; the recipient and actor SHALL be resolved to people of the event's workspace, so the requester receives the notification under their user id. A person SHALL NOT be notified of their own act: the requester's own submission, the actor's own state transitions (`asset_lifecycle`) and a self-assignment raise nothing.

#### Scenario: The requester receives the decision
- **WHEN** an approver rejects a request whose `created_by` is the requester's NGAC node
- **THEN** the requester's list in that workspace holds an `approval_rejected` naming the approver and the request's template, and the approver's list is empty

#### Scenario: Another workspace
- **WHEN** an event of workspace A names a recipient who is a member of workspace B only
- **THEN** nothing is stored for them

### Requirement: A workspace invitation is a personal notification
`workspace_invitation` SHALL be created when an invitation is created or refreshed (the workspace service announces `invitation_created` with the invitation id, for every address alike). The consumer reads the stored invitation and notifies each account whose `lower(email)` equals the invited address and whose email is verified, exactly the rule of `GET /api/invitations`. An address with no account, or only an unverified one, produces nothing, and nothing in any response reveals which happened (the inviter's answer is unchanged). Because the invitee is not yet a member, this one type is the **exception to workspace scoping**: it is listed, counted and marked whichever workspace the recipient has open, and pushed to all their sessions. A new invitation to the same offer replaces the earlier notice. The click-through opens `/workspace-select`, which shows the offer. Accepting or declining the invitation SHALL mark its notice read (`POST /api/notifications/read-about {"type":"workspace_invitation","id":<invitation id>}`).

#### Scenario: Verified account
- **WHEN** an admin invites an address that belongs to a verified account
- **THEN** that person gets one unread `workspace_invitation` in their list in any workspace, and the inviter sees the same answer as for any other address

#### Scenario: No verified account
- **WHEN** the address belongs to nobody, or to an unverified account
- **THEN** no notification exists

### Requirement: Marking read is the caller's own, in the workspace
`POST /api/notifications/{id}/read` and `POST /api/notifications/read-all` SHALL change only notifications belonging to the signed-in person in the workspace of their token (plus personal ones). Marking another person's or another workspace's notification matches nothing and changes nothing, and still answers `{"status":"ok"}`; mark-all leaves other workspaces unread. A request with no session is 401. `POST /api/notifications/read-about` marks, under the same scope, the caller's notifications about one subject (`type`, `id` both required, else 400).

#### Scenario: Someone else's notification
- **WHEN** a person marks a notification that belongs to someone else as read
- **THEN** it stays unread for its owner

#### Scenario: Another workspace's notification
- **WHEN** a person with the other workspace's token marks it, or marks all
- **THEN** it stays unread

### Requirement: A live push reaches only the recipient in the workspace
A new notification SHALL be pushed as a `NotificationEvent` frame only to the recipient's WebSocket sessions whose token is for the notification's workspace (a personal notification: all the recipient's sessions), after the row is stored. Another person's session, or the recipient's session in another workspace, receives nothing.

### Requirement: A failure is generic
A notification request that fails for a reason of the server's SHALL answer `500 {"message":"internal error","request_id"}` and never the database's text.

### Requirement: A page that is not a number is refused
`limit` or `offset` that is present but not a whole number SHALL answer 400 `{"message": "limit must be a number"}` (or `offset`) and read nothing. Absent means the default; a number out of range is still clamped as above.

### Requirement: The screen
The client SHALL build the sentence from `type` and the names, never print an id, and never show server text. Entry points, panel, rows, states and realtime behaviour follow `design/mockups/notifications.html` and DESIGN.md §5–§7: a "Thông báo" row with the unread count first in the sidebar's bottom group (opens a 380px panel); below 1024 a dot on Thêm and a "Thông báo" row at the top of the Thêm sheet; a toast with "Xem" on arrival while the panel is closed; three or more arrivals within 2000 ms become one toast; groups "Hôm nay" and "Trước đó"; mark one and mark all are optimistic and roll back with a toast on failure. The workspace switchers (sidebar and Thêm sheet) show a "Lời mời đang chờ (N)" row to `/workspace-select` when `GET /api/invitations` returns offers.

#### Scenario: Unavailable subject
- **WHEN** a person opens a notification whose subject was deleted or whose access they lost
- **THEN** it is marked read, they stay where they are, and a toast says it cannot be opened

## Follow-ups (not in this capability yet)
Document-access requests and decisions, telling the current approver "chờ bạn duyệt", telling asset approvers of a new request, the rejection reason of an asset request (the event carries none), naming who a step is waiting on, withdrawing a notice when its invitation is revoked or expires, per-type preferences and email digests.
