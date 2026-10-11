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

### Requirement: API
`GET|POST /api/workspaces/{id}/chat-sections`, `PATCH|DELETE /api/workspaces/{id}/chat-sections/{sectionId}`,
`PUT /api/workspaces/{id}/chat-sections/{sectionId}/channels/{channelId}` and
`DELETE /api/workspaces/{id}/chat-sections/channels/{channelId}`, routed to messaging in the dev proxy, Docker
(Traefik) and production (nginx).
