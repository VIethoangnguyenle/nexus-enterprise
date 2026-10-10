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
