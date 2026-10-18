# user-settings

## Purpose
Keeps each person's options (notifications, how Chat, Lịch and Tài liệu behave, the phone's bottom bar) on the server so
they follow the person across devices, lets an organization lock an option for everyone, and gathers a workspace's own
settings on one page.

## Status
Built (2026-10-19, plan `plans/261018-1742-mobile-ux-overhaul`, phases 04 and 05):
- Workspace service: the workspace-scope store (`user_settings`, migration `068_user_settings.sql`) with `mobile.bar` at
  `GET`/`PUT /api/workspaces/:id/settings`, and the group-creation lock at `GET`/`PUT /api/workspaces/:id/locks`
  (`workspace-admin-authorization`, `tenant-ngac-init`).
- Messaging: `pause_until` on `GET`/`PUT /api/me/push-preferences`, quiet hours letting a mention through for message
  pushes, and Chat's options at `GET`/`PUT /api/workspaces/:id/chat-settings` (`default_level` seeds a person's level
  when they create or are added to a private group, never a DM or #general).
- Calendar: `calendar_user_settings` extended by migration 068 with default view, week start, visible calendars, default
  duration and the three notification switches, at `GET`/`PUT /api/workspaces/:id/calendar/settings`; the invite, change
  and reminder notices read the switches.
- Drive (`drive_user_settings`, `GET`/`PUT /api/drive/settings?workspace_id=`), asset (`asset_user_settings`,
  `GET`/`PUT /api/workspaces/:id/asset-settings`) and approval (in the organization's tenant schema, `GET`/`PUT
  /api/approval/settings`, refused in a personal workspace); approval's pending and result notices read its switches.
  Drive's `notify_shared` and the asset switches are stored and shown; neither service sends a notification yet, so
  nothing reads them.
- Auth: language on `PATCH /api/me/profile`; the person's sessions at `GET /api/auth/sessions` and
  `DELETE /api/auth/sessions/:id`.
- Screens: Tôi, the module settings pages and Cài đặt workspace at `/settings/<section>` (below), opened from Tôi, from
  each module's ⋯, from the ⚙ of the workspace block in Thêm, from the ⋯ of the open workspace in the workspace picker and
  from the search action Cài đặt <phân hệ>. The phone's bottom bar is hidden on `/settings/*` and `/admin/*`; Quản trị
  left the Thêm grid. The desktop Cài đặt tabs read and write the same settings.

## Requirements

### Requirement: Every option has one owner service and one scope
Each option SHALL be stored once, by the service that enforces it, at one scope: **account** (the person, every
workspace) or **workspace** (the person in one workspace). Owners: messaging — push and Chat notification switches, quiet
hours, pause-until, Enter to send, link previews, Chat's opening view, Chat's list order ("Sắp xếp"), double-tap reaction,
default level for new groups; calendar — the existing calendar settings, extended in its own table with default view,
week start, visible calendars, default duration and its notification switches; drive — view, sort, folders first,
previews on mobile data, drive notification switches (shared documents only: no document comments exist); approval and
asset — their notification switches and view options, each in its own service; workspace — the phone's bottom bar
(workspace scope); auth — language (account scope). The read-receipt option is
not offered (no "seen by" screen exists). Theme and reduced motion stay device-only.

#### Scenario: Same option from two screens
- **WHEN** a person changes Chat's notification for direct messages from Tôi → Thông báo → Chat and then opens Cài đặt Chat
- **THEN** both screens show the same value, because both read the one stored option

### Requirement: One workspace settings page, rows by permission
Below 1024px a workspace's own settings — name, description, members and invitations, departments, roles and
permissions, contacts scope, the group-creation rule, approval templates, asset types, storage — SHALL live on one
"Cài đặt workspace" page, reached from the workspace block, the workspace picker and Tôi; it replaces the phone's
Cài đặt → Workspace and the Quản trị entry. Each row SHALL appear only to people who can act on it, using the existing
admin checks, and the server SHALL decide on every screen. A personal workspace shows only its name and storage. Changing
a workspace's kind is never offered.

#### Scenario: Member opens workspace settings
- **WHEN** a member who is not an administrator opens Cài đặt workspace in an organization
- **THEN** they see the name, description, administrators, the rules (read only), their profile in the organization and
  Rời; no member, role, department, template or storage management is offered, and crafted requests to them are refused

### Requirement: Only a member reads or writes their settings in a workspace
Every settings route that names a workspace SHALL require the caller to be a member of it; a person SHALL only ever read
or write their own settings.

The workspace service SHALL serve `GET /api/workspaces/:id/settings` (`{"settings": {key: value}}`, only keys that were
set) and `PUT /api/workspaces/:id/settings` (`{"settings": {key: value}}`; only the keys sent change, all are validated
before any is written). The person is always the user id of the verified claims, never a field of the request. The one key
this service owns is `mobile.bar`: a JSON array of exactly 4 distinct module ids from `chat`, `calendar`, `drive`,
`approval`, `assets`, `contacts`, `notifications`, in slot order. A personal workspace offers no `approval` or `assets`,
and `admin` is never allowed. Anything else (wrong length, a repeat, an unknown id, a non-array, an unknown key, an empty
body) answers 400 and writes nothing.

#### Scenario: Another member's settings
- **WHEN** a member reads the settings after a colleague of the same workspace saved a bar
- **THEN** they see none of the colleague's keys, and saving theirs leaves the colleague's alone

#### Scenario: Invalid bar
- **WHEN** a member saves a bar with three modules, a repeated module, `admin`, or `approval` in a personal workspace
- **THEN** the answer is 400 and nothing is written

#### Scenario: Not a member
- **WHEN** a user asks for or writes workspace-scope settings of a workspace they are not in
- **THEN** the answer is 403 and nothing is written

### Requirement: Switches that turn something off stop it
A notification switch or quiet hours SHALL be read by the code that sends the notification, so turning it off stops it.

Quiet hours block push, except when the person is mentioned by name, `@Tất cả` included (`chat-mention-all`). The
exception covers message pushes only, and only while the mention switch is on; call rings, call invites and calendar
pushes stay blocked in quiet hours. "Tạm dừng thông báo" (`pause_until`) blocks every push until that moment, mentions
included; a pause whose moment has passed reads as not paused.

#### Scenario: Paused
- **WHEN** a person has paused notifications until a later moment and is mentioned in a message
- **THEN** no push is sent; after the moment passes, pushes resume

#### Scenario: Direct-message push off
- **WHEN** a person turns off push for direct messages and receives one
- **THEN** no push is sent to them

#### Scenario: Quiet hours and a mention
- **WHEN** it is within a person's quiet hours and a message arrives, first without mentioning them and then mentioning
  them by name or with `@Tất cả`
- **THEN** no push is sent for the first and a push is sent for the second

### Requirement: An organization can lock who creates groups
In an organization workspace, an administrator (the workspace admin check) MAY lock "Ai được tạo nhóm" to administrators.
The lock SHALL be implemented by removing the right, not by a prohibition: while locked, `create_channel` is removed from
the Members → Channels association and from every custom role's Channels grant, and the Owners keep their own
association. The roles screen SHALL refuse to grant `create_channel` on Channels while the lock is on. Unlocking SHALL
restore the Members association; custom roles are **not** restored automatically and an administrator grants them again.
Writes SHALL go through the EPP path, and the lock state SHALL be read from the graph (the Members association is
present or not); no separate lock row exists. The routes are `GET`/`PUT /api/workspaces/:id/locks` with
`{"group_creation": "everyone" | "admins"}` (`workspace-admin-authorization`); the graph change is specified in
`tenant-ngac-init`. Personal workspaces have no locks. Locked rows on screen show the value and
that an administrator set it ("Quản trị viên <workspace> đã đặt", without a person's name, because the graph records no
actor), and cannot be changed.

#### Scenario: Member while locked
- **WHEN** group creation is locked and an ordinary member tries to create a group
- **THEN** the PDP denies `create_channel`

#### Scenario: Custom role while locked
- **WHEN** group creation is locked and a member who holds a custom role with the Channels area tries to create a group
- **THEN** the PDP denies `create_channel`

#### Scenario: Owner while locked
- **WHEN** group creation is locked and a workspace owner creates a group
- **THEN** the PDP allows `create_channel`

#### Scenario: Granting while locked
- **WHEN** group creation is locked and an administrator tries to grant `create_channel` on Channels to a custom role
- **THEN** the roles screen does not offer it and a crafted request is refused

#### Scenario: Unlocking
- **WHEN** an administrator unlocks group creation
- **THEN** ordinary members can create groups again, and custom roles that lost the right stay without it until granted

#### Scenario: Non-admin sets the lock
- **WHEN** a member who is not an administrator tries to lock group creation
- **THEN** the answer is 403 and the graph is unchanged

### Requirement: Every settings PUT is a merge
A settings `PUT` (or `PATCH`) SHALL change only the fields its body carries; a field the body leaves out keeps its stored
value (or the default, when nothing was stored). The merged result is validated as a whole before anything is written, so
a bad field fails the save and stores nothing; a body that carries nothing changes nothing and is still refused to
someone who may not write there. A field sent with an empty value (`""`, `0`, `false`) is a value, not an omission, and is validated as such. This
holds for `PUT /api/workspaces/:id/settings` (only the keys sent), `PUT /api/workspaces/:id/chat-settings`,
`PUT /api/workspaces/:id/calendar/settings`, `PUT /api/drive/settings`, `PUT /api/workspaces/:id/asset-settings`,
`PUT /api/approval/settings` and `PATCH /api/me/profile`. For Lịch an empty `visibleCalendars` list means every calendar,
and only a list the body carries is checked against the calendars the caller can read. The one exception is
`PUT /api/me/push-preferences`, which takes the push kinds and quiet hours whole and keeps `incoming_call` and
`pause_until` when they are omitted (`null` for `pause_until` resumes). Screens therefore send only the change, so a stale
copy of the other fields on one device never overwrites a change made on another.

#### Scenario: The desktop calendar dialog saves its four fields
- **WHEN** a person who turned off invitation, change and reminder notices and chose a week start, default view, default
  duration and visible calendars on the phone then saves the zone, working hours and reminder in the desktop dialog
- **THEN** the zone, hours and reminder change and the other seven settings keep their values

#### Scenario: One switch from a module page
- **WHEN** a person turns off "Được chia sẻ tài liệu" in Cài đặt Tài liệu
- **THEN** the body is `{"notify_shared": false}` and the view, sort, folders-first and previews settings are unchanged

#### Scenario: A bad merged value
- **WHEN** a save carries one invalid field (an unknown view, an end of the working day before its start)
- **THEN** the answer is 400 or 422 and nothing is written

### Requirement: Settings pages have their own address on a phone
The settings pages SHALL be addressed as `/settings/<section>` with the workspace in `?ws=`: `account`, `org-profile`,
`notifications`, `appearance`, `sessions`, `workspace` (Cài đặt workspace), and one per module — `chat`, `calendar`,
`drive`, `approval`, `assets`. Below 1024px `/settings` itself is Tôi; on a desktop it is the tabbed Cài đặt
(`?tab=workspace`, `giao-dien`, `thong-bao`). Each page is the same whichever entry point opened it, and going back
returns to that entry point (Tôi when there is no history). An address naming no page, or a page the workspace does not
have (`org-profile`, `approval` and `assets` in a personal workspace), lands on `/settings` instead of an error. Danh bạ
has no settings page. Neither the settings pages nor the administration screens (`/admin/*`) show the phone's bottom bar.

#### Scenario: A module's ⋯
- **WHEN** a person opens the ⋯ on the title of Chat, Lịch, Tài liệu, Phê duyệt or Tài sản on a phone
- **THEN** its first row is Cài đặt <phân hệ>, which opens `/settings/<section>` of that module in the same workspace

#### Scenario: Cài đặt workspace from Thêm or the workspace picker
- **WHEN** a person taps the ⚙ on the workspace block of Thêm, or the ⋯ of the open workspace in the workspace picker
- **THEN** `/settings/workspace` opens for that workspace; Thêm has no Quản trị tile on a phone

#### Scenario: A page a personal workspace does not have
- **WHEN** someone in a personal workspace opens `/settings/approval`
- **THEN** they land on `/settings`

### Requirement: A person lists only their own sessions
`GET /api/auth/sessions` (auth service) SHALL list the signed-in sessions of the caller only — the user comes from the
verified claims, never the request — as `{"sessions": [{"session_id", "device", "started_at", "current"}]}`: the session in
use first, then the others newest first. `device` is a coarse "browser · system" label made from the User-Agent at sign-in
("Safari · iPhone", "Chrome · Windows"; empty when the agent is not a known browser); the raw header is not stored, and
`session_id` is an opaque handle that is never shown on screen. `DELETE /api/auth/sessions/:id` ends one of the caller's
other sessions through the existing `RevokeSession`, which takes a bare session id, so the route SHALL first check that
the session is the caller's and still live. Another user's session answers 404, exactly like an unknown or ended one, so
the route cannot reveal which ids are live; the session in use answers 400 (signing out ends it).

#### Scenario: Another user's sessions
- **WHEN** a user lists their sessions while another user also has sessions
- **THEN** only the caller's own are returned, each with its device label, the current one first

#### Scenario: Revoking someone else's session
- **WHEN** a user asks to end a session id that belongs to someone else
- **THEN** the answer is 404 and that session stays signed in

#### Scenario: Revoking the session in use
- **WHEN** a user asks to end the session they are using
- **THEN** the answer is 400 and nothing is ended
