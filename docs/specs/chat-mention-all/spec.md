# chat-mention-all

## Purpose
Lets someone in a group reach everyone in it with one mention, `@Tất cả`, under the group's rule for who may do so.

## Status
Specified ahead of code (2026-10-19, plan `plans/261018-1742-mobile-ux-overhaul`, phase 07). No `@Tất cả` exists today;
mentions are lists of user ids.

## Requirements

### Requirement: @Tất cả notifies every member of the group
A message in a group MAY carry the mention `@Tất cả`. It SHALL notify every current member of the group (except the sender)
as if each were mentioned by name, following each member's notification level for the group the way a name mention does.
It SHALL NOT be available in direct messages.
In the workspace-wide channel only workspace administrators (the workspace admin check) MAY use it, because everyone in the
workspace is a member there and has no group owner or managers.

#### Scenario: Mention everyone
- **WHEN** a member sends a message with `@Tất cả` in a group of 7
- **THEN** the 6 other members are notified as mentioned

### Requirement: The group decides who may mention everyone
When the group's setting "Ai được nhắc @Tất cả" is "Chỉ chủ nhóm và quản trị nhóm" (see `chat-group-management`), the
messaging service SHALL refuse `@Tất cả` from a non-manager on **send, reply and edit**, so an edit cannot add it to a
message that was sent without it. Listing every member by name is a normal mention and is not affected.

#### Scenario: Edit adds @Tất cả
- **WHEN** the setting is restricted and a member edits their message to add `@Tất cả`
- **THEN** the edit is refused and nobody is notified

#### Scenario: Manager
- **WHEN** the setting is restricted and a manager sends `@Tất cả`
- **THEN** it is delivered

### Requirement: Direct messages and the workspace-wide channel restrict it
`@Tất cả` SHALL be refused in a direct message, and in the workspace-wide channel from anyone who is not a workspace
administrator, on send, reply and edit alike.

#### Scenario: Direct message
- **WHEN** a person sends `@Tất cả` in a direct message
- **THEN** it is refused and nobody is notified

#### Scenario: Workspace-wide channel, member
- **WHEN** a member who is not an administrator sends, replies or edits in `@Tất cả` in the workspace-wide channel
- **THEN** each is refused

#### Scenario: Workspace-wide channel, administrator
- **WHEN** a workspace administrator sends `@Tất cả` in the workspace-wide channel
- **THEN** every other member of the workspace is notified as mentioned
