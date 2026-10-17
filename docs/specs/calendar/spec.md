# calendar

## Purpose
A Google-Calendar-style module: each person has a calendar, the workspace has a shared one, and every group
conversation has its own. Events repeat, have guests who answer, remind, and can carry a video call. Every decision
about who may see or change what is an NGAC check; nothing in the calendar service decides access by role or by
table. Events are not graph nodes: like a message, an event is authorized on the OA of the calendar that holds it, or
on its own OA when it has guests or a call.

## Requirements

### Requirement: There are three kinds of calendar, each created when first needed
A calendar SHALL be personal (one per person per workspace), the workspace calendar (one per workspace) or a group
calendar (one per group conversation; a direct message has none). Each SHALL be created the first time it is needed
and never twice, including under concurrent first use. A calendar's graph shape SHALL be:

- **personal**: OA `Cal_<calendarId>` (`ngac.CalendarOAName`) under the workspace's policy class; the owner's personal
  UA (`ngac.PersonalUAName`) is associated with `read write share manage` (`ngac.CalendarOwnerOps`);
- **workspace**: OA under the policy class; `<ws>_Members` is associated with `read`, `<ws>_Owners` with
  `read write share manage`;
- **group**: OA assigned beneath the conversation's `Ch_<id>_Content` OA, so that members inherit
  `read write invite` (`ngac.ChannelMemberOps`) and nobody else has anything. Nothing the workspace's own UAs hold on
  the Channels OA reaches it: `<ws>_Members` has `create_channel` only and `<ws>_Owners` `manage create_channel`
  there (`ngac.MemberChannelOps`, `ngac.OwnerChannelOps`), so a workspace member or owner who is not in the group
  has no right on its calendar (vectors in `pdp_calendar_scenarios_test.go`). `manage` is inherited by the group's
  calendar from the Content OA the owners hold it on, so the service SHALL NOT read `manage` (or `write`) as access
  to a calendar the person cannot also `read`: every access level includes reading, and a calendar one cannot read
  is not held at all.

No operation is added to the vocabulary of eight. All names come from `backend/ngac` and are keyed by ID, never by a
name a tenant chooses.

#### Scenario: First use
- **WHEN** a person opens the calendar for the first time
- **THEN** their personal calendar exists, and opening it again creates no second one

#### Scenario: A direct message
- **WHEN** a calendar is requested for a direct-message conversation
- **THEN** the answer is a refusal and no calendar or node is created

#### Scenario: Workspace calendar, ordinary member
- **WHEN** a workspace member who is not an owner reads the workspace calendar
- **THEN** they see it, and an attempt to add or change an event is refused (403)

#### Scenario: Group calendar, outsider
- **WHEN** a person who is not in the group conversation asks for its calendar or its events
- **THEN** the answer is 403 and nothing is returned
- **AND** this holds for a member of the workspace and for a workspace owner alike (the owner's `manage` on the group does not make its calendar readable, and the calendar is not listed for them); the group's own member reads and writes it
- **AND** the same owner still manages the workspace calendar

### Requirement: Calendar membership is being under the workspace's Members UA
Every calendar request SHALL be admitted only for a caller who is a user under `<ws>_Members` of the workspace their
token is scoped to (`policyclient.IsWorkspaceMember`), and every person a request names (a guest, a grantee, someone to
subscribe to) SHALL be one too, by the graph and by the people table. Reaching the workspace's policy class is not
membership: an event's guest UA (`CalEvGuests_<eventId>`) and a group's UA sit under the class so that the intersection
rule holds, and a person who is only a guest of an event or only in a group gets nothing of the workspace's calendars.
A policy service that cannot answer denies.

#### Scenario: Guest of an event or member of a group only
- **WHEN** a user whose only path to the workspace's policy class is an event's guest UA or a group's Members UA lists calendars or is named as a guest
- **THEN** the request is refused (403)
- **AND** an owner and an ordinary member of the same workspace are admitted

### Requirement: A personal calendar is shared at one of three levels
Sharing a personal calendar SHALL require `share` on its OA (403 otherwise) and name a person of the same workspace
and one of three levels, chosen from `ngac.CalendarShareOps` and never from operation names in the request:

| Level | Operations | Meaning |
|---|---|---|
| `view` | `read` | sees every detail |
| `edit` | `read write` | and changes events |
| `manage` | `read write share manage` | and shares or deletes the calendar |

A share SHALL be an association from the grantee's personal UA to the calendar's OA. That UA has no parent of its own
(the drive creates it the same way); the grantee reaches the workspace's policy class through the workspace UAs they
already belong to, so the association holds exactly while both sides reach the same policy class. A person of another
workspace therefore gets nothing even if an association was written. Changing the level SHALL replace the
association; revoking SHALL remove it, and the next decision is DENY. Sharing needs EPP invalidation like every graph
write. A person never shares with themselves, and there is no "free/busy only" level (see the next requirement).

#### Scenario: View share
- **WHEN** a calendar is shared with a person at `view`
- **THEN** they read its calendar and its events, and an attempt to write is refused

#### Scenario: Edit share
- **WHEN** a calendar is shared at `edit`
- **THEN** the grantee changes events, and cannot share or delete the calendar

#### Scenario: A stranger
- **WHEN** a workspace member with no share reads another person's personal calendar or one of its events
- **THEN** the answer is 403

#### Scenario: Share revoked
- **WHEN** a share is removed
- **THEN** the former grantee is refused on their next request

#### Scenario: Another workspace
- **WHEN** a person of another workspace holds an association to the calendar's OA
- **THEN** every operation is denied, because the two sides reach different policy classes

#### Scenario: Unknown level
- **WHEN** a share names a level other than the three
- **THEN** the answer is 400 and no association is written

### Requirement: Colleagues see when someone is busy, never why
For any person of the same workspace, the service SHALL answer free/busy for their personal calendar as time
ranges only (no title, place, guests or description). The check SHALL be that both people are users under the
workspace's Members UA, the same test the workspace uses for membership (a person who only reaches the policy class
through an event's guest UA or a group's UA is not a member); it does not use a calendar share. A person
of another workspace SHALL be refused. A private event SHALL count as busy like any other and show no detail to
anyone who is not allowed to read the calendar.

#### Scenario: A colleague with no share
- **WHEN** a member asks when another member is busy
- **THEN** they get busy ranges and nothing else

#### Scenario: A stranger to the workspace
- **WHEN** a person outside the workspace asks
- **THEN** the answer is 403

### Requirement: Events are authorized on the calendar, not on themselves
Listing, reading, creating, changing and deleting an event SHALL be decided by the calendar's OA: `read` to see,
`write` to create, change or delete. An event that has no guests and no call SHALL have no node. Seeing a calendar
the caller may not read SHALL answer 403; an unknown event 404. A request SHALL name its workspace from the verified
caller, never the body.

#### Scenario: Read-only
- **WHEN** a person with `read` alone creates, moves or deletes an event
- **THEN** the answer is 403 and the event is unchanged

#### Scenario: Plain event
- **WHEN** an event with no guests and no call is created
- **THEN** it has no graph node

### Requirement: An event can repeat, and an edit says how far it reaches
An event SHALL carry its start with its time zone and MAY repeat by an RRULE. Only `FREQ` DAILY, WEEKLY, MONTHLY and
YEARLY are accepted (with INTERVAL, BYDAY, BYMONTHDAY, BYMONTH, BYSETPOS); `COUNT` is at most 730; `COUNT` and `UNTIL`
are not both given; a rule with neither is expanded no further than five years after its start; sub-day parts are
refused (422). Occurrences SHALL be expanded in the event's own time zone, so "09:00 every Monday" stays at 09:00
across a daylight-saving change. The server SHALL return a Vietnamese sentence describing the rule
(`recurrenceText`); clients do not translate the rule. Changing or deleting an occurrence SHALL ask for a scope
(`scope=this|following|all` with `occurrence` = the `occurrenceStart` of the instance): **this one**, **this and
following**, or **all**. A scope applied to a one-off event is the same as all; a repeating event without one is
refused (422).

- **this**: writes (or rewrites) an exception row for that occurrence; deleting it writes a cancelled exception.
- **following**: ends the series with `UNTIL` one second before the occurrence and starts a new event there with the
  change applied (the guests and reminders are copied; a `COUNT` becomes the number that remained); exceptions from
  that occurrence on are dropped. From the first occurrence it is the same as **all**. The answer is the new event.
- **all**: changes the series row. Exceptions edited on their own are kept (their number is answered as
  `preservedOverrides`). When `start`/`end` are sent they are the new position of the named occurrence and the whole
  series shifts by the same wall-clock distance in its zone (a weekly `BYDAY` follows a change of weekday). A
  monthly or yearly rule that names its day (`BYMONTHDAY`, `BYMONTH`, or an ordinal weekday such as `2WE`) does not
  follow: moving such a series (all, or following) to another day of the month, month, or week of the month is
  refused (422, "Không dời được chuỗi lặp theo thứ trong tháng sang ngày khác. Hãy sửa quy tắc lặp.") and the rule is
  edited instead; moving it within the same day, to another time, is allowed.

A change to one occurrence (**this**) MAY restate the series' `allDay` and `tzid` (screens send them with every edit);
only a value that differs from the series' is refused (422). When changing the guests, the call or the invite flag
fails after the event row was saved, the row, its reminders and (for a shifted series) its exceptions are written
back, a split's new tail is removed, and the version moves on; answers to single occurrences of a shifted series and
exceptions cut off by a split cannot be restored.

An edit SHALL carry the `version` it started from. A series, a single event or an exception row each has its own
version; the one an occurrence shows is its exception's when it has one, else the series'. A stale version answers
409 with the message "Sự kiện vừa được người khác sửa. Tải lại rồi thử lại.", and a lost race to create the same
exception is the same 409. An occurrence is listed only when it overlaps the requested range (at most 400 days).

#### Scenario: This and following
- **WHEN** a weekly event is changed from one occurrence "this and following"
- **THEN** earlier occurrences keep their old time and the series splits there

#### Scenario: Daylight saving
- **WHEN** a weekly 09:00 event in New York spans a clock change
- **THEN** every occurrence is at 09:00 local time

#### Scenario: Just this one
- **WHEN** one occurrence is deleted with scope "this one"
- **THEN** only that day disappears; the series continues

#### Scenario: Stale edit
- **WHEN** an edit arrives with a version that is no longer current
- **THEN** the answer is 409 and nothing is changed

#### Scenario: Rule out of bounds
- **WHEN** a rule is hourly, or asks for more than 730 occurrences
- **THEN** the answer is 422 and nothing is stored

### Requirement: What a person sees of an occurrence depends on what they hold
For each occurrence the service SHALL answer whether the caller may edit it (`canEdit`: `write` on the calendar and
the right to read it). A caller who holds only free/busy of a calendar (a colleague of a personal calendar), or who
may not read a `private` event, SHALL get `{start, end, busyOnly: true}` with the title "Bận" and no place, guests,
organizer or call; an event marked free (`transparency: free`) is left out of such a list. A `private` event is read
in full only by its organizer, its guests, the owner of a personal calendar and whoever holds `manage` on the
calendar; a person who cannot read an event SHALL NOT change or delete it either. Reading one event (`GET
/events/:id`) needs `read` on the calendar, or `read` on the event's own OA for a guest (403 otherwise).

The event's OA sits beneath its calendar's, so everyone who reads the calendar also holds `read` on the event's OA.
"Guest" therefore means both an attendee row *and* `read` on the event's OA; `read` on the OA alone makes nobody a
guest. A private event is read by its guests, its organizer, the owner of a personal calendar and whoever holds
`manage` — never by a reader of the calendar who is none of those, with or without guests or a call. A busy-only
occurrence SHALL NOT carry the event's id: it carries an opaque key (stable per occurrence, `busy-…`) the screen uses
to tell blocks apart, which no route answers to (404).

#### Scenario: Free/busy only
- **WHEN** a colleague with no share lists another person's calendar
- **THEN** each occurrence is a "Bận" block and fetching one event is refused (403)

#### Scenario: Private event and a view share
- **WHEN** a calendar is shared at `view` and holds a private event of its owner
- **THEN** the grantee sees a "Bận" block, and reading the event is refused (403)

#### Scenario: Private event with guests and a call
- **WHEN** a private event has guests and a call, and a view-share reader who is not one of them reads it or asks for its call
- **THEN** both are refused (403), the listing shows a "Bận" block with an opaque key, and a guest still reads and joins

#### Scenario: Every query stays in its workspace
- **WHEN** a person who is a guest of an event in workspace A lists, searches or asks free/busy in workspace B
- **THEN** the answer is 200 without that event and without a busy block for it; in A the event is there

### Requirement: Guests are people of the workspace and see only their event
Adding a guest SHALL require `write` on the calendar, or `invite` on the event's OA for a guest who was allowed to
invite. Only a person of the same workspace can be a guest; an email address is refused (no outside guests). The first
guest SHALL create the event's OA `CalEv_<eventId>` (`ngac.CalendarEventOAName`) beneath the calendar's OA and a UA
`CalEvGuests_<eventId>` (`ngac.CalendarEventGuestsUAName`) under the workspace's policy class, associated with
`ngac.CalendarGuestOps`: `read`, plus `invite` only when the host switched on "guests can invite others". Guests
SHALL never hold `write`: they cannot edit, move or delete the event, nor read the host's calendar beyond it.
Removing a guest SHALL remove the assignment, and the next decision is DENY. Deleting the event SHALL delete its OA
and UA. An event that has a call keeps its OA and UA with no guest at all; one that has neither SHALL have no node, so
removing the last guest (with no call) deletes them. Guests belong to the whole series: they are named when the event
is made, in `attendees` of an edit (the whole list of guests wanted; the organizer is always one) or by
`POST /events/:id/guests`, never for one occurrence (422). A group conversation MAY be invited (`inviteChannelId`): the
caller needs `read` on its content OA, its members are expanded when the event is saved, and each is checked against
the graph (the member table is only a cache). The guest rule is enforced twice: the event's `guests_can_invite` and the
`invite` operation on its OA must both allow a guest to invite. A guest who has an out-of-office event over the time is
added as having declined, with the note "Đang vắng mặt". A "this and following" edit gives the new event its own OA and
UA with the same guests. An event appears in a guest's listing wherever its calendar is, when the guest holds `read`
on the event's OA; a guest who may not see the guest list (`guestsCanSeeList` off) is shown themselves and the
organizer.

#### Scenario: A guest reads
- **WHEN** a person is invited to an event
- **THEN** they read that event and see it on their own calendar, and cannot read the host's other events

#### Scenario: A guest edits
- **WHEN** a guest tries to change, move or delete the event
- **THEN** the answer is 403

#### Scenario: A guest without the invite flag
- **WHEN** a guest invites another person and the host did not allow it
- **THEN** the answer is 403

#### Scenario: A guest with the invite flag
- **WHEN** the host allows guests to invite and a guest invites someone
- **THEN** the new guest is added and may read, not edit

#### Scenario: Removed guest
- **WHEN** a guest is removed
- **THEN** their next read of the event is 403

#### Scenario: An outside address
- **WHEN** an email address, or a person who is not a member of the workspace, is given as a guest
- **THEN** the answer is 422 for an address and 403 for a person, and nobody is added

### Requirement: A guest answers the invitation
A guest SHALL answer `yes`, `no` or `maybe` with an optional short note, for themselves only (403 on anyone else's
answer). The answer needs `read` on the event's OA. The host and other guests see each guest's answer; an answer to a
repeating event applies to the occurrence, series or "this and following" the guest chooses. A removed guest's answer
is removed with them. An answer to one occurrence is kept beside the series' answer (`calendar_attendee_occurrences`,
by the occurrence's original start) and wins for that occurrence; an answer for all replaces them. Moving the whole
series or changing its rule drops the single-occurrence answers; a "this and following" edit keeps those of the tail
when its times are unchanged. An occurrence a guest declined stays on their listing, marked declined.

#### Scenario: Answering
- **WHEN** a guest answers "maybe" with a note
- **THEN** the host sees "maybe" and the note beside that guest's name

#### Scenario: Answering for someone else
- **WHEN** a person answers on another guest's behalf
- **THEN** the answer is 403

### Requirement: Finding a time uses busy ranges
Free/busy for up to 20 people over at most 62 days (`GET /freebusy`) SHALL return, per person, busy ranges and working
hours (`workStart`/`workEnd`, HH:mm in the person's own zone; 08:00 to 17:30 in Asia/Ho_Chi_Minh until they save
settings with `PUT /settings`). Busy is an event on the person's personal calendar, or one they are invited to, that is
marked busy and not declined (a tentative or unanswered invitation counts), repeating events expanded. A title is
shown only when the caller reads the calendar the event is on or is a guest of it (a private event only to those who
may read it in full); everyone else shows ranges only. A person who is not a member of the caller's workspace, by the
database and by the graph, SHALL be refused (403) and nobody is answered about. `GET /freebusy/suggest` SHALL return
at most three of the earliest half-hour-grid times on a day (a date in the caller's zone) of the asked duration (5 to
720 minutes) inside everyone's working hours (each person's, on that date in their zone) when nobody is busy.

#### Scenario: Another workspace
- **WHEN** a person of another workspace is among the people asked about
- **THEN** the answer is 403

### Requirement: Events can be found by their text
`GET /events/search?q=` SHALL find, among the events of the calendars the caller reads and those they are invited to,
those whose title, place, description or a guest's name contains the text (case-insensitively; the characters of a
pattern are literal), at most 50, each as its next occurrence (its last when the series is over), upcoming first. An
event the caller may not read in full never matches, and a guest's name matches only where the caller sees the guest
list.

### Requirement: Reminders arrive as notifications
An event MAY carry reminders (minutes before; a person's own reminders replace the event's for them). Every 30 seconds
the calendar service SHALL look for occurrences whose reminder falls due: now is at or past the occurrence's start
minus the minutes, and not more than 10 minutes past it. For each such occurrence and each person it concerns it SHALL
claim the reminder in `calendar_reminder_deliveries` (one row per event, person, occurrence and minutes; a second claim
by any instance does nothing) and then publish it to the Redpanda topic `calendar.events` (`event.reminder`) and wait
for the broker's answer. A publish that fails gives the claim back, so the next pass retries it while the 10 minutes
of grace last; with no broker nothing is claimed. Delivery is therefore at least once (a broker timeout may repeat a
reminder), never silently lost. The
people it concerns are the guests who have not declined that occurrence (an answer to the occurrence outranks the
answer to the series), plus the owner of a personal calendar or the host of any other event unless they declined. The
messaging service SHALL store one notification of type `calendar_reminder` for each, and send a web push
(`docs/specs/notifications/spec.md`). A person who is not a guest, or lost access before it fell due, gets none. The
service loads, in the database, only the events that can be due: a single event whose start is within its own longest
reminder of now (two weeks for a two-week reminder) and not over, and a repeating series that has begun and not
finished, whose near occurrences are then expanded in memory. Inviting someone SHALL notify them
as `calendar_invitation`, a guest's answer the host as `calendar_rsvp`, a move of time or place the guests as
`calendar_updated`, a cancellation the guests as `calendar_cancelled` and sharing a calendar the grantee as
`calendar_shared`.

#### Scenario: Reminder
- **WHEN** an event with a 10-minute reminder is 10 minutes away
- **THEN** each person who can read it and has not declined has one new notification, with the event's title and start time

#### Scenario: Once only
- **WHEN** the reminder falls due and two calendar services (or two passes) look for it
- **THEN** each person is reminded once

#### Scenario: Declined
- **WHEN** a guest declined that occurrence of a repeating event
- **THEN** they get no reminder for it, and still get one for the next occurrence

#### Scenario: No longer allowed
- **WHEN** a guest was removed after the event was created and its reminder falls due
- **THEN** they get no notification

### Requirement: Changes reach open screens
A committed change to a calendar or event SHALL be announced on `calendar.events` and delivered as a `calendar`
domain event under `docs/specs/realtime-event-delivery/spec.md`: workspace-level for the workspace calendar,
user-addressed to the owner, grantees and guests for personal calendars and events, channel-level for a group
calendar. A frame carries ids only, never titles. Receiving one only marks the affected calendar views stale; each
client refetches under its own permission.

#### Scenario: A move seen in another tab
- **WHEN** an event is moved in one tab
- **THEN** a second tab of an authorized person shows the new time without a manual reload

#### Scenario: Not allowed to see
- **WHEN** a change concerns a personal calendar the person cannot read
- **THEN** they receive no frame for it

### Requirement: An event can carry a call, authorized by read on the event
An event MAY have a video call (`docs/specs/chat-calls/spec.md`). The ticket SHALL be granted only after `read` on the
event's OA (for a group-calendar event, on the group's content OA as for any call in that group), checked at the
moment of asking. A person who is not a guest, not the owner and not shared with is refused (403). The call of a
private event is for its guest list alone (an attendee row and `read` on the OA): a reader of the calendar is refused.
The call is also refused to a caller whose workspace is not the event's.

#### Scenario: A guest joins
- **WHEN** a guest asks for the call ticket
- **THEN** they get it, and a stranger asking gets 403

### Requirement: A group's calendar speaks in the group's chat
An event created on a group's calendar SHALL post a message of its creator into the group's conversation: a system message
carrying an event card (`linked_entity_type` `calendar_event`, the event's id and nothing else). The card is read live from
the calendar under each viewer's own access, so a move shows at once and a viewer who may no longer read the event sees no
details. A move in time of such an event (the whole event, or one occurrence) SHALL post a short line of the person who
moved it ("<Tên> đã dời “<tiêu đề>” sang <giờ>", worded in the event's own zone), and a cancellation a line of the person
who cancelled it; a cancelled event's card is dimmed and says "Đã huỷ". A rename, a change of place or of guests posts
nothing. A private event is never announced to the chat. Messaging reads these as notices with `announce` on
`calendar.events` (`event.created`, `event.updated`, `event.cancelled`, with `channel_id`, `actor_user_id`,
`actor_node_id` and `tzid`), which have no recipients and make no notification. The topic delivers at least once and the
announcement SHALL make one message: the message id is derived from what is announced. Nothing is posted into a direct
message or a conversation of another workspace, nor in the name of someone who can no longer read the group.

#### Scenario: A new event
- **WHEN** a member creates an event on the group's calendar
- **THEN** the group's stream shows a card of it from the creator, and the same notice delivered twice stores one message

#### Scenario: A move
- **WHEN** the event is moved to another time
- **THEN** the card shows the new time and a line "<Tên> đã dời … sang <giờ>" follows it

#### Scenario: Someone who left the group
- **WHEN** the creator no longer reads the group when the notice arrives
- **THEN** nothing is posted

#### Scenario: A card answered by a non-guest
- **WHEN** a member who was not invited answers the card
- **THEN** the answer is 403; the card offers no answer to someone who is not a guest

### Requirement: A group has a Lịch tab
A group conversation (not a direct message, not a personal workspace) SHALL have a **Lịch** tab beside Trò chuyện, Tệp and
Công việc: what is coming up on the group's calendar, a small month, "Tạo sự kiện" and "Mở trong Lịch". "Tạo sự kiện" (also
"Sự kiện" in the composer's "Tạo trong chat" menu, except in a thread's reply box) opens the editor on the group's
calendar, making the calendar first if it does not exist, and an event made there invites the whole group. Someone who is
not in the group is refused by the calendar's API (403) and the tab shows a sentence, never events. The video call of an
event on a group's calendar is the group's own call.

#### Scenario: Outsider
- **WHEN** a person who is not in the group asks for the group calendar's events
- **THEN** the answer is 403

### Requirement: An occurrence can be read
`GET /events/{id}?occurrence=<occurrenceStart>` SHALL answer for that instance of a repeating event: its own start and end,
and the data of its exception when it has one. An instant the series never produces, or a cancelled occurrence, is 404.
Changing the guests of a repeating event belongs to the whole series: the editor does not offer "only this one" for it.

### Requirement: Deleting removes what the event created
Deleting an event SHALL be allowed with `write` on the calendar and SHALL remove its guests, reminders, its OA and
UA, and end any call it opened. Deleting a calendar SHALL need `manage` and delete all events and the calendar's
nodes. The workspace calendar and personal calendars can be emptied but a person's last personal calendar is not
deletable. Deleting a graph node SHALL go through the policy write service so the EPP invalidates every replica.

#### Scenario: Delete
- **WHEN** an event with guests is deleted
- **THEN** its guests are refused on the next read and its OA and UA no longer exist

#### Scenario: Delete without manage
- **WHEN** a person with `write` but not `manage` deletes a whole calendar
- **THEN** the answer is 403

### Requirement: The screens show people and titles, never identifiers
Every guest, owner, sharer and organizer SHALL show as display name (with avatar), calendars as their name, and a
person is picked from a list or search, never typed as an ID or email.

## Known limits
- The organizer stays in the event's guest UA (an attendee row, `accepted`) on purpose: they keep access to an event
  they created even after losing access to its calendar.
- Deleting a group conversation does not delete its calendar's nodes or events (messaging has no hook for it yet).
  The orphaned calendar is unreachable (nobody holds anything on it) but its OA, rows and events remain.
- A share is two records, the association in the graph and `calendar_shares`; both change under one lock per
  (calendar, grantee), the graph first.

## Status
Added 2026-10-17. Guests, answers, free/busy, suggestions, search and settings are implemented
(`POST /events/:id/rsvp`, `POST /events/:id/guests`, `GET /freebusy`, `GET /freebusy/suggest`, `GET /events/search`,
`GET/PUT /settings`; `data/migrations/058_calendar_occurrence_answers.sql`). Vocabulary, graph shape and PDP test vectors (`pdp_calendar_scenarios_test.go`) exist. The service
(`backend/services/calendar`, REST 8187, schema `data/migrations/057_calendar.sql`) implements the three kinds of
calendar, lazy creation, subscriptions and sharing (`GET /api/workspaces/:id/calendar/calendars`, shares, subscriptions,
group calendar). Events (`GET/POST /events`, `GET/PATCH/DELETE /events/:id`, expansion of repeating events in their
zone, exceptions, edit and delete scopes, versions, Vietnamese `recurrenceText`, busy-only occurrences, `calendar`
realtime events for created/updated/deleted) are implemented in the same service. Reminders (a 30 second scheduler,
claimed in `calendar_reminder_deliveries`), the notices on `calendar.events` (`event.invited`, `event.updated`,
`event.cancelled`, `event.rsvp`, `event.reminder`, `calendar.shared`, and `event.created` for an event on a group's
calendar, which carries the group and notifies nobody) and the notifications and push they make in the messaging
service are implemented. The call of an event (`POST /api/calendar-events/:eventId/call`) is served by the messaging
service and needs the event to carry a call (`hasCall`). A person's own reminders are honoured when present but no
route writes them yet. A group calendar's announcements (`announce` notices) are posted by the messaging service into the
group's chat (`backend/services/messaging/internal/domain/calendar_chat.go`), and the group's Lịch tab, the event card and
"Sự kiện" in the composer are in the frontend (`components/spaces/SpaceCalendar.tsx`, `components/calendar/EventChatCard.tsx`).
