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
- **declined** — `POST /api/calls/{id}/decline` by the direct message's other person, with `write` (403 for the
  caller, an outsider, or anyone else who happens to hold write there): `declined`;
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
- `participant_left`: the stay ends. A stay is one connection (LiveKit participant sid): a person's second device can
  take the first one's place, and its arrival may be reported before the first one's departure — the person stays in.
- `room_finished`: every stay ends; an `active` call becomes `ended` (`ended_at` set), a ringing one `missed`.
Events for an unknown room are ignored.

When someone is removed from a conversation or a workspace, the open calls of that conversation (or of every
conversation of the workspace, and every direct call someone took a ticket into from that workspace) SHALL be swept
1 second and again 3 seconds later, and each person in them who no longer holds `write` removed from the room —
within 5 seconds of the change. Any other permission change (an association, a prohibition, a role) sweeps every
open call the same way, and every instance sweeps every open call each 30 seconds as a safety net.

#### Scenario: Ticket signed before removal
- **WHEN** a person removed from a group enters the room with a ticket signed before the removal
- **THEN** they are removed from the room and do not appear among the participants

### Requirement: A lost webhook does not leave a call open forever
An `active` call with nobody recorded in it, in which nobody arrived or left for longer than the room's empty timeout
plus a minute, and whose room LiveKit reports empty (or gone), SHALL be taken as ended when someone starts a call or
asks for a ticket, as if `room_finished` had arrived. The age of the call itself never ends it. LiveKit opens no room
by itself (`auto_create: false`): a stale ticket cannot bring a closed call's room back.

### Requirement: The card
Starting a call SHALL post a system message in the main stream of the conversation, sent by the caller, with
`linked_entity_type: "call"` and `linked_entity_id` = the call's id; it cannot be posted by hand (400). Its text — which
previews and search read — SHALL be "📹 Cuộc gọi video" / "📞 Cuộc gọi thoại" while the call rings or runs, then:
ended "… · 32 phút" (under a minute "· dưới 1 phút", an hour or more "· 1 giờ 15 phút"), missed "📹 Cuộc gọi nhỡ",
declined "… Cuộc gọi bị từ chối", cancelled "… Cuộc gọi đã huỷ". Rewriting it is the call's state, not an edit: the
card does not show as edited. The card SHALL NOT be pushed as a message notification.

### Requirement: Live changes
Each committed change SHALL be announced as a `call_event` (`ServerEnvelope` field 23): `type` (`ring`, `cancel`,
`answered`, `declined`, `missed`, `state`, `invited`), the call's id, conversation, kind, status, caller (name, avatar),
`participant_count`, `participant_ids`, `started_at`, `ended_at`. `ring`, `cancel`, `answered`, `declined` and `missed`
go to every session of both people of a direct call, whatever workspace they are in (so another device stops ringing
when one answers); every change goes as `state` to the conversation's subscribers; nobody else receives it. When the
card's text changed, the card goes to the subscribers as an edited message.

### Requirement: Inviting people to a call
`POST /api/calls/{id}/invite` with `{"user_ids": [...]}` SHALL require `write` on the conversation (403 otherwise) and
an open call (409 `call_ended`). Of the people named (at most 20), only those who hold `write` there — who could join —
are invited; the rest are skipped without a word. Each invitee's open app hears a `call_event` of type `invited`, and
their devices a push (see `web-push`). A person @mentioned in the main stream within two minutes of a group call
starting is invited the same way.

### Requirement: Ringing reaches devices
A direct call's ringing, and the missed call that replaces it, SHALL be pushed to the other person as specified in
`web-push`, besides the live `call_event`.

### Requirement: One-to-one calls end for both
When an answered direct call has fewer than two people left in it (one hung up) and still has after a 15-second
grace — a page reload or a reconnect also leaves before it joins again — its room SHALL be closed, so LiveKit's
`room_finished` ends it for both. A group call goes on while anyone is in it.

### Requirement: The screens
The screens render DESIGN.md §6 "Cuộc gọi video và thoại" and `design/mockups/video-call.html`:
- `GET /api/channels/{id}/call` (read) answers `{"call": <open call> | null}`; the conversation header shows Gọi thoại /
  Gọi video in a direct message, "Bắt đầu cuộc gọi" in a group, and "Tham gia" with "Đang gọi · mm:ss" while the
  group's call is open, kept fresh by `call_event`.
- The call lives at `/calls/{id}`; `?answer=video|audio` joins directly, a group call otherwise shows the pre-join
  check (camera preview, microphone level, device choice remembered per person on the device). The room survives
  moving to another screen (a floating tile; Picture-in-Picture when leaving the tab).
- A ringing direct call shows a floating card on a computer and a full-screen ring on a phone, with a WebAudio ring
  for up to 45 seconds (Esc silences it); answering on one device makes the others say "Đã trả lời trên thiết bị khác".
- The call's card shows it live (people, clock, "Tham gia" / "Mở lại cuộc gọi"), ended (people · length · times) or
  missed ("Gọi lại"). Settings › Thông báo has a "Cuộc gọi đến" switch (`incoming_call`); it governs push
  notifications only — an open app still rings (decided 2026-10-13).
- Someone removed from the conversation is cut off with "Bạn không còn ở trong nhóm này". No id is ever shown.

### Requirement: A calendar event can hold a call
A call SHALL belong to a conversation or to a calendar event, exactly one of the two (`calls.calendar_event_id`,
`channel_id` nullable, a CHECK for exactly one). A call of an event SHALL be started and joined only after `read` on
the event's OA (`CalEv_<id>`) — checked when the ticket is asked for, never trusted from an earlier check — and a
person without it SHALL get 403 and no ticket. An event in a group calendar uses that group's own call and its
`write` rule above. A call of an event posts no card (there is no conversation); everything else — one open call per
event, the 20-person limit, `call_ended`, `call_full` — applies unchanged. See `docs/specs/calendar/spec.md`.

#### Scenario: A guest joins
- **WHEN** a guest of an event asks for its call ticket
- **THEN** they get a ticket for that event's room

#### Scenario: Not a guest
- **WHEN** a workspace member who cannot read the event asks for the ticket
- **THEN** the answer is 403 and no token is signed

### Requirement: Configuration
Calls SHALL be on only when `LIVEKIT_URL`, `LIVEKIT_PUBLIC_URL`, `LIVEKIT_API_KEY` and `LIVEKIT_API_SECRET` are all
set; otherwise starting a call or asking for a ticket answers 503 `calls_unavailable`, and the webhook 503.

## Known limits
- One person, one place: identity is the user id, so a second device joining takes the first one's place in the room.
- A group call does not ring anybody.
- iOS home-screen apps cannot ring while closed (a notification only) and cannot present the screen.

## Status
Added 2026-10-13: server (lifecycle, tickets, webhook, card, live events, ringing notifications, invitations) and
screens (entry points, pre-join, stage, phone, incoming ring, cards, settings). Verified on the dev stack with real
LiveKit (`frontend/e2e/call-ui-live.mjs` 22/22, `frontend/e2e/call-ring-live.mjs` 12/12; removed member cut off in
1.3 s). Real devices (iPhone PWA, Android), a UDP-blocked network and a load test on the VPS are still to be done.
