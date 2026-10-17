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
  `read write invite` (`ngac.ChannelMemberOps`) and nobody else has anything.

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
ranges only (no title, place, guests or description). The check SHALL be that the workspace's policy class is an
ancestor of both people, the same test the workspace uses for membership; it does not use a calendar share. A person
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
  series shifts by the same wall-clock distance in its zone (a weekly `BYDAY` follows a change of weekday).

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

#### Scenario: Free/busy only
- **WHEN** a colleague with no share lists another person's calendar
- **THEN** each occurrence is a "Bận" block and fetching one event is refused (403)

#### Scenario: Private event and a view share
- **WHEN** a calendar is shared at `view` and holds a private event of its owner
- **THEN** the grantee sees a "Bận" block, and reading the event is refused (403)

### Requirement: Guests are people of the workspace and see only their event
Adding a guest SHALL require `write` on the calendar, or `invite` on the event's OA for a guest who was allowed to
invite. Only a person of the same workspace can be a guest; an email address is refused (no outside guests). The first
guest SHALL create the event's OA `CalEv_<eventId>` (`ngac.CalendarEventOAName`) beneath the calendar's OA and a UA
`CalEvGuests_<eventId>` (`ngac.CalendarEventGuestsUAName`) under the workspace's policy class, associated with
`ngac.CalendarGuestOps`: `read`, plus `invite` only when the host switched on "guests can invite others". Guests
SHALL never hold `write`: they cannot edit, move or delete the event, nor read the host's calendar beyond it.
Removing a guest SHALL remove the assignment, and the next decision is DENY. Deleting the event SHALL delete its OA
and UA.

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
- **WHEN** an email address that is not a workspace member is given as a guest
- **THEN** the answer is 400 and nobody is added

### Requirement: A guest answers the invitation
A guest SHALL answer `yes`, `no` or `maybe` with an optional short note, for themselves only (403 on anyone else's
answer). The answer needs `read` on the event's OA. The host and other guests see each guest's answer; an answer to a
repeating event applies to the occurrence, series or "this and following" the guest chooses. A removed guest's answer
is removed with them.

#### Scenario: Answering
- **WHEN** a guest answers "maybe" with a note
- **THEN** the host sees "maybe" and the note beside that guest's name

#### Scenario: Answering for someone else
- **WHEN** a person answers on another guest's behalf
- **THEN** the answer is 403

### Requirement: Finding a time uses busy ranges
Finding a free time for several people SHALL return busy ranges per person under the free/busy rule above. A person
the caller may read in full may also show titles; everyone else shows ranges only.

### Requirement: Reminders arrive as notifications
An event MAY carry reminders (minutes before). When one falls due, the calendar service SHALL publish it to the
Redpanda topic `calendar.events`, and the messaging service SHALL store one notification of type `calendar_reminder`
for each person who may see the event, and send a web push to them. A reminder SHALL fire once per occurrence and
person. A person who lost access before it fell due gets none. Inviting someone SHALL notify them as
`calendar_invitation`, and a guest's answer SHALL notify the host as `calendar_rsvp`.

#### Scenario: Reminder
- **WHEN** an event with a 10-minute reminder is 10 minutes away
- **THEN** each person who can read it has one new notification, with the event's title and start time

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
moment of asking. A person who is not a guest, not the owner and not shared with is refused (403).

#### Scenario: A guest joins
- **WHEN** a guest asks for the call ticket
- **THEN** they get it, and a stranger asking gets 403

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

## Status
Added 2026-10-17. Vocabulary, graph shape and PDP test vectors (`pdp_calendar_scenarios_test.go`) exist. The service
(`backend/services/calendar`, REST 8187, schema `data/migrations/057_calendar.sql`) implements the three kinds of
calendar, lazy creation, subscriptions and sharing (`GET /api/workspaces/:id/calendar/calendars`, shares, subscriptions,
group calendar). Events (`GET/POST /events`, `GET/PATCH/DELETE /events/:id`, expansion of repeating events in their
zone, exceptions, edit and delete scopes, versions, Vietnamese `recurrenceText`, busy-only occurrences, `calendar`
realtime events for created/updated/deleted) are implemented in the same service. Guests, RSVP, free/busy lists, event
search, reminders, notifications and calls are specified here ahead of their code; until guests exist an event is
created with its organizer as the only attendee, and naming other attendees is refused (422).
