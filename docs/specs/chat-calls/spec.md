# chat-calls

## Purpose
Voice and video calls inside a conversation. A direct message rings the other person; a group conversation holds a
call of up to 20 people that anyone who writes there may start or join. The media runs on a self-hosted LiveKit server;
the messaging service owns the lifecycle, the card in the stream, and the rule that only someone who may write in the
conversation is in its call. A call is not an NGAC node: like a message, it is checked on its channel's content OA.

## Requirements

### Requirement: Starting a call needs write on the conversation
`POST /api/channels/{id}/calls` with `{"kind": "audio" | "video"}` SHALL require `write` on the channel's content OA
(403 otherwise) and a known kind (400). In a direct message the call SHALL ring (`status: ringing`) until
`ring_expires_at`, 45 seconds later; in any other conversation it SHALL be `active` at once. Starting a call SHALL post
its card (below) and open a LiveKit room named by the call's id, limited to 20 participants. The answer (201) is a
ticket: `{call, token, url}`.

#### Scenario: Reader
- **WHEN** someone with `read` but not `write` on the channel starts a call
- **THEN** the answer is 403, no room is opened and no ticket is signed

### Requirement: One open call per conversation
A conversation SHALL have at most one call that is `ringing` or `active`. Starting a call while one is open SHALL
answer (200) with a ticket into that call and post no card, including when two people start one in the same instant.

### Requirement: A ticket is granted on a fresh check
`POST /api/calls/{id}/token` SHALL check `write` on the call's conversation at that moment (403 otherwise) — never
relying on an earlier check — and answer `{call, token, url}`. The token is a LiveKit access token for that room only,
valid 10 minutes, with identity = the user's id and name = their display name, allowed to publish camera, microphone
and screen, to send data and to subscribe, and not to administer the room. A call that is over SHALL answer 409 with
code `call_ended`; a call with 20 people in it SHALL answer 409 `call_full` to anyone not already in it (a person
already inside — another device, a reconnect — takes no new place). Signalling reaches the browser at `url`
(`LIVEKIT_PUBLIC_URL`, same origin under `/livekit`).

#### Scenario: Removed member
- **WHEN** a member is removed from the conversation after the call began and asks for a ticket
- **THEN** the answer is 403

### Requirement: Seeing a call needs read
`GET /api/calls/{id}` SHALL require `read` on the conversation (403 otherwise; 404 for an unknown call) and answer the
call: `kind`, `status`, who started it (`started_by_name`, `started_by_avatar_url`), `created_at`, `started_at`,
`ended_at`, `ring_expires_at`, `participants` (who is in it now: `display_name`, `avatar_url`, `joined_at`) and
`taker_count` (everyone who was ever in it). Ids in the answer are for the client's own use and never shown.

### Requirement: Ringing ends one way
A ringing direct call SHALL end exactly once:
- **answered** — the other person enters the room: `active`, `started_at` set;
- **declined** — `POST /api/calls/{id}/decline` by someone other than the caller, with `write` (403 for the caller or
  an outsider): `declined`;
- **cancelled** — `POST /api/calls/{id}/cancel` by the caller, with `write` (403 for anyone else): `cancelled`;
- **missed** — 45 seconds pass, or the room closes first: `missed`. Every instance sweeps every 5 seconds; each call is
  expired by exactly one.

Declining or cancelling a call that no longer rings SHALL answer 409 `call_ended`. Declined, cancelled and missed
calls close their room.

### Requirement: Only someone who may write stays in the room
LiveKit reports to `POST /internal/livekit/webhook`, which is never routed from outside the network. A request not
signed with the LiveKit API secret over the body's SHA-256 SHALL be refused (401) before anything in it is read.
- `participant_joined`: the person's `write` on the conversation SHALL be checked again, as the NGAC node and workspace
  they were handed their last ticket as (recorded with the ticket; a person has one node per workspace), on their own
  behalf; without it — or with no ticket from this service at all — they are removed from the room and not recorded. Otherwise their stay is recorded, and a ringing call is answered when they are not
  the caller.
- `participant_left`: the stay ends.
- `room_finished`: every stay ends; an `active` call becomes `ended` (`ended_at` set), a ringing one `missed`.
Events for an unknown room are ignored.

When someone is removed from a conversation or a workspace, the open calls of that conversation (or of every
conversation of the workspace) SHALL be swept 1 second and again 3 seconds later, and each person in them who no
longer holds `write` removed from the room — within 5 seconds of the change.

#### Scenario: Ticket signed before removal
- **WHEN** a person removed from a group enters the room with a ticket signed before the removal
- **THEN** they are removed from the room and do not appear among the participants

### Requirement: A lost webhook does not leave a call open forever
An `active` call whose room has been empty in LiveKit for longer than its empty timeout plus a minute SHALL be taken
as ended when someone starts a call or asks for a ticket, as if `room_finished` had arrived.

### Requirement: The card
Starting a call SHALL post a system message in the main stream of the conversation, sent by the caller, with
`linked_entity_type: "call"` and `linked_entity_id` = the call's id; it cannot be posted by hand (400). Its text — which
previews and search read — SHALL be "📹 Cuộc gọi video" / "📞 Cuộc gọi thoại" while the call rings or runs, then:
ended "… · 32 phút" (under a minute "· dưới 1 phút", an hour or more "· 1 giờ 15 phút"), missed "📹 Cuộc gọi nhỡ",
declined "… Cuộc gọi bị từ chối", cancelled "… Cuộc gọi đã huỷ". Rewriting it is the call's state, not an edit: the
card does not show as edited. The card SHALL NOT be pushed as a message notification.

### Requirement: Live changes
Each committed change SHALL be announced as a `call_event` (`ServerEnvelope` field 23): `type` (`ring`, `cancel`,
`answered`, `declined`, `missed`, `state`), the call's id, conversation, kind, status, caller (name, avatar),
`participant_count`, `participant_ids`, `started_at`, `ended_at`. `ring`, `cancel`, `answered`, `declined` and `missed`
go to every session of both people of a direct call, whatever workspace they are in (so another device stops ringing
when one answers); every change goes as `state` to the conversation's subscribers; nobody else receives it. When the
card's text changed, the card goes to the subscribers as an edited message.

### Requirement: Configuration
Calls SHALL be on only when `LIVEKIT_URL`, `LIVEKIT_PUBLIC_URL`, `LIVEKIT_API_KEY` and `LIVEKIT_API_SECRET` are all
set; otherwise starting a call or asking for a ticket answers 503 `calls_unavailable`, and the webhook 503.

## Known limits
- One person, one place: identity is the user id, so a second device joining takes the first one's place in the room.
- A group call does not ring anybody.
- iOS home-screen apps cannot ring while closed (a notification only) and cannot present the screen.

## Status
Added 2026-10-13 with the server part (lifecycle, tickets, webhook, card, live events). Ringing notifications, the
call screens and the entry points follow.
