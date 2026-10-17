# chat-channel-avatar

## Purpose
A group (a channel that is not a direct message) can have a picture of its own, shown wherever the group is listed.
Same shape as a person's profile picture: re-encoded server side, three sizes, an unguessable public address.

## Requirements

### Requirement: Who may change the picture
Setting or removing a group's picture SHALL require `manage` on the channel's content OA, checked through the policy
service exactly as a rename is; the check fails closed (policy unreachable means no change). Without it the answer is
403 and nothing is stored or announced. A direct message has no group picture: the answer is 400 whoever asks.
The group info panel offers the picture change and the rename only to someone who holds `manage` (the same check the
server makes); other members see the name and picture without a control.

#### Scenario: A member who may only read and write
- **WHEN** a member holding `read` and `write` but not `manage` uploads or removes the picture
- **THEN** the answer is 403, the channel's `avatar_url` and stored picture are unchanged, and no event is sent

#### Scenario: A direct message
- **WHEN** a participant of a direct message uploads a picture for it
- **THEN** the answer is 400 and nothing is stored

### Requirement: Upload and removal
`PUT /api/channels/:chId/avatar` takes a multipart form whose `file` part is a JPEG or PNG of at most 5 MB and answers
`{"avatar_url": "/api/channels/avatars/<token>"}`. `DELETE /api/channels/:chId/avatar` answers `{"avatar_url": ""}`
and is idempotent. The picture is decoded by its content, never its name or declared type, and re-encoded: the centre
square, the right way up, on white where transparent, as JPEG at 64, 192 and 1024 px; nothing of the original file
but its pixels survives. Each upload gets a new random token, so replacing a picture changes its address and the old
address answers 404. `channels.avatar_url` and the stored picture are written in one transaction.

Refusals carry a machine-readable `code`: `avatar_not_image` (400), `avatar_too_many_pixels` (400),
`avatar_too_large` (413), `avatar_missing` (400, no `file` part), `avatar_busy` (503 with `Retry-After`, another
picture is being decoded), `rate_limited` (429 with `Retry-After`).

#### Scenario: Replacing a picture
- **WHEN** a manager uploads a second picture
- **THEN** `avatar_url` names a new token and the first token answers 404

### Requirement: Limits
One person MAY upload 10 pictures per hour (Redis key `channel_avatar_rl:<user id>`); one decode runs at a time in a
messaging process and an upload waits up to 5 seconds for it. The permission check comes first, so a person who may
not manage the group does not spend the allowance. Without Redis nothing is counted.

### Requirement: Serving the picture
`GET /api/channels/avatars/:token[/:size]`, size 64, 192 (default) or 1024, is public — an `<img>` sends no bearer
token — and answers the JPEG with `Cache-Control: public, max-age=31536000, immutable`, `nosniff` and a restrictive
CSP. The token is 32 lowercase hex characters; anything else, an unknown token or another size is 404 (cacheable for
a minute). Each address MAY fetch 1200 pictures per minute (Redis key `channel_avatar_pub_rl:<hash of the address>`),
then 429 with `Retry-After`. The address is the connection's own, never `X-Forwarded-For`. A failure of the counter
itself lets the request through.

### Requirement: The channel carries its picture
Every channel the service returns — workspace list, single read, and the direct-message list — carries `avatar_url`
(`""` or absent when the group has none). A client renders the initials then.

### Requirement: Announcing the change
After the change commits, the service SHALL emit the channel-domain event a rename emits (`channel` / `renamed`, to
the channel's own subscribers), carrying only the channel id. Clients fetch the channel again; the event carries no
address. Nothing is emitted for a refused or failed change.

## Data
`channel_avatars (channel_id PK → channels ON DELETE CASCADE, token UNIQUE, small, medium, large, created_at)` and
`channels.avatar_url TEXT NOT NULL DEFAULT ''` (migration `056_channel_avatars.sql`).
