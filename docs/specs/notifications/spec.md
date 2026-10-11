# notifications

## Purpose
A person's notifications are listed, counted and marked read over REST by the messaging service, the same data the gRPC notification service serves and the WebSocket pushes live.

## Requirements

### Requirement: Notifications are listed with their totals
`GET /api/notifications` SHALL return the signed-in person's notifications, newest first, as `{"notifications": [...], "total", "unread_count"}`. `limit` (default 25, at most 50; anything else becomes 25) and `offset` (default 0) page the list. Each notification carries `id`, `type`, `title`, `body`, `read`, `created_at` and, when it concerns an entity, `entity_type` and `entity_id`. An empty list is `[]`, not null. `GET /api/notifications/unread-count` SHALL return `{"count"}`.

#### Scenario: A page
- **WHEN** a person with 30 notifications asks for `limit=10&offset=10`
- **THEN** they get the second ten, `total` is 30 and `unread_count` counts all their unread notifications

#### Scenario: Nobody else's
- **WHEN** two people have notifications
- **THEN** each list, count and total covers only the caller's own

### Requirement: Marking read is the caller's own
`POST /api/notifications/{id}/read` and `POST /api/notifications/read-all` SHALL change only notifications belonging to the signed-in person. Marking another person's notification matches nothing and changes nothing, and still answers `{"status":"ok"}`. A request with no session is 401.

#### Scenario: Someone else's notification
- **WHEN** a person marks a notification that belongs to someone else as read
- **THEN** it stays unread for its owner

### Requirement: A failure is generic
A notification request that fails for a reason of the server's SHALL answer `500 {"message":"internal error","request_id"}` and never the database's text.

### Requirement: A page that is not a number is refused
`limit` or `offset` that is present but not a whole number SHALL answer 400 `{"message": "limit must be a number"}` (or `offset`) and read nothing. Absent means the default; a number out of range is still clamped as above.

### Requirement: Each person chooses how much of a conversation counts
Every person SHALL have, per conversation they read, a notification level (`channel_notification_levels`,
migration 045): `all` — every message and every reply by someone else counts toward the badge; `main` (the
default, also when no row exists) — the main messages and the replies in threads they follow; `off` — nothing
counts. `GET /api/channels/{chId}/notification-level` answers `{"level"}`; `PUT` with `{"level"}` sets it.
Both need `read` on the channel's content OA (403 otherwise); an unknown level is 400. `GET
/api/channels/unread` reports each channel's `notification_level` with its counts, and under `off` both
`unread_count` and `thread_unread_count` are 0. Every channel also reports `mention_count`, the unread main messages
that mention the caller, at every level — a muted group still shows its "@". The level is the person's own and never widens what they may
read. A direct message is only on (`main`) or `off`: `all` on a DM is 400. Web push follows the level as
`docs/specs/web-push/spec.md` describes.

#### Scenario: Muted conversation
- **WHEN** a person sets a group to `off` and others keep writing in it
- **THEN** its badge stays at 0 and nothing from it is counted

#### Scenario: Everything counts
- **WHEN** a person sets a group to `all` and someone replies in a thread they do not follow
- **THEN** that reply counts as unread for them

#### Scenario: Not a reader
- **WHEN** someone who cannot read a conversation asks for or sets its level
- **THEN** the answer is 403 and nothing is stored
