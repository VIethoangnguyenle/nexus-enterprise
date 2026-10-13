# chat-sections

## Purpose
Personal sections of the chat navigator, as in Google Chat: each user groups their own groups and direct messages
into named sections of their choosing. A section arranges one person's list only; it grants nothing.

## Requirements

### Requirement: Sections are personal and grant nothing
A section SHALL belong to one user in one workspace; no one else lists, renames, reorders, deletes or fills it — to
anyone else it does not exist (404). It is not an NGAC node and placing a conversation in it changes no access. Every
section call SHALL first require the caller to be in the workspace (`read` on its workspace-level attribute, inside
their own tenant); otherwise 403.

#### Scenario: Another member's section
- **WHEN** a member renames, deletes or places a conversation into a section id another member created
- **THEN** the answer is 404 and the section is unchanged

### Requirement: Names, order and membership
A section name SHALL be 1–40 characters after trimming and unique among the user's sections in the workspace, whatever
its case (409 on a duplicate). Sections keep the order the user gives them (`move` up/down; the first cannot move up nor
the last down). A conversation SHALL sit in at most one of the user's sections: placing it again moves it. A
conversation can be placed only when the user reads its channel (403 otherwise) and it is a channel of this workspace
or a direct message (400 otherwise). A channel that is deleted leaves every section.

#### Scenario: A channel of another workspace
- **WHEN** a member places a readable channel of another workspace into a section here
- **THEN** the answer is 400

### Requirement: Delete and Undo
Deleting a section SHALL return it as it was — name, position and conversations — and send its conversations back to
the default lists. Creating a section with that position and those conversations restores it in place (Undo), subject
to the same checks.

### Requirement: Sections on a phone
Below 1024px the sections are the tab Mục of the phone's Chat (`/channels?tab=sections`), in the navigator column's order: Đã gắn sao (when anything is starred), the user's
sections, Tin nhắn trực tiếp, Nhóm. Rows are 56px with a 36px avatar and show the name only (bold when unread) with a
count, "@" for a mention in a muted conversation, or a bell-off. A header folds and opens its section with the same
state as the desktop column (`ngac-nav-collapse`), totals the unread by notification level while folded, and has its
own pill that lists only the unread conversations of that section (remembered on the device for the person, section by
section: `ngac-section-unread-only:<user>:<workspace>:<section>`). Long-pressing a
row opens its sheet (Chuyển tới mục, Mục mới…, Bỏ khỏi mục, Gắn sao, Tắt thông báo, and Đánh dấu đã đọc when it has
unread messages); long-pressing the header of one of the user's own sections opens Đổi tên mục, Chuyển lên, Chuyển
xuống and Xoá mục. "Mục mới" ends the list and asks for a name. The two shortcut tiles and the followed-thread block
that used to head the list are gone from it (see `chat-shortcuts`, `chat-activity-feed`).

#### Scenario: Unread filter of one section
- **WHEN** a person turns on the unread pill of "Quyết toán quý 4"
- **THEN** that section lists only its conversations with unread messages and the other sections are unchanged

#### Scenario: The filter is remembered
- **WHEN** the same person returns to Mục on that device
- **THEN** the pill of "Quyết toán quý 4" is still on, the other sections are not affected, and another person on the device sees it off

#### Scenario: Menu of an own section
- **WHEN** a person long-presses the header of one of their sections
- **THEN** the sheet offers Đổi tên mục, Chuyển lên, Chuyển xuống, Xoá mục; the headers of Đã gắn sao, Tin nhắn trực tiếp
  and Nhóm open no sheet

### Requirement: API
`GET|POST /api/workspaces/{id}/chat-sections`, `PATCH|DELETE /api/workspaces/{id}/chat-sections/{sectionId}`,
`PUT /api/workspaces/{id}/chat-sections/{sectionId}/channels/{channelId}` and
`DELETE /api/workspaces/{id}/chat-sections/channels/{channelId}`, routed to messaging in the dev proxy, Docker
(Traefik) and production (nginx).
