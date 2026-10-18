# chat-mention-all

## Purpose
Lets someone in a group reach everyone in it with one mention, `@Tất cả`, under the group's rule for who may do so.

## Status
Implemented (phase 07 of `plans/261018-1742-mobile-ux-overhaul`). The enforcement lives in
`backend/services/messaging/internal/domain/group_management.go` (`checkGroupPostingRestrictions`,
`checkEditMentionAllRestriction`); the push expansion is in `push.go` (`mentionedPeople`).

## Requirements

### Requirement: @Tất cả notifies every member of the group
A message in a group MAY carry the mention `@Tất cả` (the canonical string `MentionAll`). The `mentionedPeople` function
expands it to every current member of the conversation (`store.ChannelPeople`), so each of them is notified as if
mentioned by name (the sender is excluded by the caller). The member count is not capped — only explicit name mentions
are capped at `MaxMentionPushes` (20). Quiet hours let mentions through: `silenced` returns false when the recipient is
mentioned (by name or through `@Tất cả`). The notification level `off` silences the conversation, but in a group a
mention (including `@Tất cả`) still gets through.
It SHALL NOT be available in direct messages.
In the workspace-wide channel only workspace administrators (the `isWorkspaceAdmin` check) MAY use it, because everyone in the
workspace is a member there and has no group owner or managers.

#### Scenario: Mention everyone
- **WHEN** a member sends a message with `@Tất cả` in a group of 7
- **THEN** the 6 other members are notified as mentioned

### Requirement: The marker survives the HTML filter
In formatted (HTML) content the mention is a span `<span data-mention="@Tất cả">@Tất cả</span>`. The server's HTML
allowlist (`domain/richtext.go`) SHALL keep `data-mention` when it is a user ID (`[\w.:-]{1,128}`) or exactly the
`MentionAll` string (the pattern is built from that constant with `regexp.QuoteMeta`), and drop it otherwise; every
other attribute on the span (`onclick`, `style`, …) is dropped as before. The stored HTML therefore keeps the marker, an
edit reads it back, and the message stays in everyone's "Được nhắc". Messages stored before this rule lost the marker
and stay as they are.

#### Scenario: Marker kept, companions dropped
- **WHEN** a message's HTML carries `<span data-mention="@Tất cả" onclick="…">@Tất cả</span>`
- **THEN** the stored HTML keeps `data-mention="@Tất cả"` and has no `onclick`; a `data-mention` that is the marker
  with anything added (`@Tất cả x`) is dropped

#### Scenario: Edit keeps the mention
- **WHEN** a member sends a message mentioning everyone and edits it
- **THEN** the stored HTML and the stored mentions still carry `@Tất cả`

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
