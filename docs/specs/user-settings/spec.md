# user-settings

## Purpose
Keeps each person's options (notifications, how Chat, Lịch and Tài liệu behave, the phone's bottom bar) on the server so
they follow the person across devices, lets an organization lock an option for everyone, and gathers a workspace's own
settings on one page.

## Status
Specified ahead of code (2026-10-19, plan `plans/261018-1742-mobile-ux-overhaul`, phases 04–05).

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

#### Scenario: Not a member
- **WHEN** a user asks for or writes workspace-scope settings of a workspace they are not in
- **THEN** the answer is 403 and nothing is written

### Requirement: Switches that turn something off stop it
A notification switch or quiet hours SHALL be read by the code that sends the notification, so turning it off stops it.

Quiet hours block push, except when the person is mentioned by name, `@Tất cả` included (`chat-mention-all`).

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
present or not); no separate lock row exists. Personal workspaces have no locks. Locked rows on screen show the value and
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

### Requirement: A person lists only their own sessions
`GET /api/auth/sessions` (auth service) SHALL list the signed-in devices and sessions of the caller only, and ending one
SHALL use the existing session revocation (`RevokeSession`, which takes a bare session id), so the route SHALL first check
that the session belongs to the caller.

#### Scenario: Another user's sessions
- **WHEN** a user lists their sessions while another user also has sessions
- **THEN** only the caller's own are returned, and revoking a session id that belongs to someone else is refused
