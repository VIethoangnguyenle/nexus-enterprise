# text-documents

## Purpose
Let people write a document in the app (Văn bản) and keep it safe while they write: one author at a time, saved on its own, refused rather than overwritten when someone else saved first. A document is authorized like a drive file: on the OA of the folder it sits in, never on a node of its own.

## Requirements

### Requirement: A document is authorized on its folder's OA
A text document SHALL NOT be a node of the graph. Reading it SHALL take `read`, and creating, changing or deleting it SHALL take `write`, on the OA of the folder it sits in (the workspace's Documents OA at the top). The OA SHALL be worked out when the request is made, from where the document sits then, so a folder that is removed takes the document's permissions with it. Every check SHALL fail closed: no caller, an unresolvable OA or a policy-service error denies. A document that does not exist and one the caller may not touch SHALL answer alike, so an id's existence is not learned; so SHALL a document deleted while a save of it is in flight.

| Endpoint | Op | Object |
|---|---|---|
| `GET /workspaces/:id/documents/texts[?scope=&cursor=&limit=]` | `read`, per document | each document's folder OA (one batch call per page) |
| `GET /workspaces/:id/documents/texts/count[?scope=]` | `read`, per OA | the OAs the counted documents sit under (one batch call) |
| `POST /workspaces/:id/documents/texts` | `write` | the OA of `folder_id`, or the workspace Documents OA |
| `GET /documents/texts/:docId` | `read` (and `write`, reported as `can_write`) | the document's folder OA |
| `PATCH /documents/texts/:docId` | `write`; what comes back needs `read` too | the document's folder OA |
| `DELETE /documents/texts/:docId` | `write` | the document's folder OA |

#### Scenario: Reader tries to write
- **WHEN** a caller holding `read` but not `write` on the folder saves, creates or deletes
- **THEN** the answer is 403 and nothing is written

#### Scenario: A grant one step off
- **WHEN** the caller holds `write` on another folder, `manage` on this one, or `write` only on the Documents OA while the folder is below it
- **THEN** the request is denied

#### Scenario: Folder trashed
- **WHEN** the folder a document sits in is no longer active
- **THEN** the document is unreachable for everyone, whatever they hold on that OA

#### Scenario: Policy outage
- **WHEN** the policy service errors during a listing
- **THEN** the call fails and lists nothing

### Requirement: Writing does not carry the right to read
A save SHALL return the saved title and text only to a caller who also holds `read` on the OA. A caller who may write but not read SHALL receive a 200 without content (and the version), and, on a stale save, a 409 carrying the current version alone: no title, text or owner. Reading the document the ordinary way stays a 403 for them.

#### Scenario: Write-only caller saves
- **WHEN** a caller holding `write` but not `read` saves
- **THEN** the save is applied and the response has no `content`

#### Scenario: Write-only caller is stale
- **WHEN** that caller's save is based on an old version
- **THEN** the answer is 409 with `current: {version}` and nothing else about the document

### Requirement: Listings show only what the caller may read, a page at a time
A listing SHALL carry no content, SHALL hold only documents of the named workspace that the caller may read, and SHALL say per document whether the caller may also write. It SHALL be ordered by last edit, newest first, and served in pages: `limit` (default 50, at most 200; an oversized value is clamped, a non-positive or non-numeric one is a 400) and an opaque `cursor`; the answer carries `next_cursor` when more may follow. One call SHALL read a bounded number of rows however few of them the caller may read. A bad cursor is a 400. The count route SHALL answer how many documents of a scope the caller may read without loading any of them. `scope` SHALL be one of `mine` (written by the caller), `drafts` (the caller's drafts) or `shared` (written by someone else, or by nobody any more); another value is a 400. Owners SHALL be named only while they belong to the workspace.

#### Scenario: Interleaved folders
- **WHEN** documents of a readable and an unreadable folder alternate in time
- **THEN** every page holds only readable ones, and the pages together hold each readable one once

### Requirement: A save names the version it is based on
A save SHALL carry `base_version` and be applied only if the document is still at that version, in one statement, so two saves from the same version cannot both succeed. Otherwise the answer SHALL be 409 with `reason: "version_conflict"` and the document as it now stands, and nothing SHALL be written. The conflict body SHALL be sent only to a caller who holds `write`. The document returned by a successful save SHALL be the version that save produced.

#### Scenario: Stale save
- **WHEN** a save is based on version 1 and the document is at version 2
- **THEN** the answer is 409 carrying version 2, and the stored text is unchanged

#### Scenario: Racing saves
- **WHEN** several saves from the same version arrive together
- **THEN** exactly one succeeds and the rest are conflicts

### Requirement: What a document holds is bounded
A title SHALL be 1 to 200 characters after trimming. Content SHALL be valid UTF-8 without NUL and at most 1 MiB; a larger request body SHALL be refused before it is read. A status SHALL be `draft`, `active` or `archived`. A save SHALL change at least one field. A failure that is not the caller's SHALL answer with a generic body, never database text. Content SHALL be sanitized on the server when it is saved, to what the editor can produce (paragraphs, headings, the inline marks, lists, quotes, code, rules, and links to http, https or mailto, with no attribute but a link's address), and again by readers wherever it is shown or loaded into the editor.

#### Scenario: Script in a save
- **WHEN** a save carries a script, an event handler, a `javascript:` or `data:` link, a frame, a form, a style or a class
- **THEN** what is stored and returned has none of them

### Requirement: The editor saves for the person and never overwrites silently
Writing SHALL be automatic after a pause in typing, with no Save button, and the state SHALL be in words next to the title ("Đang lưu…", "Đã lưu 09:41", "Chưa lưu, đang chờ mạng", "Chưa lưu: có bản mới hơn"). One save SHALL be in flight at a time; text typed during it is saved right after. A network failure SHALL keep the text, say it is waiting and retry. After a conflict the editor SHALL stop sending, whatever is typed, and the person SHALL choose: take the other version (their unsaved changes are dropped) or keep theirs (their text is written over the other version, because they asked), after optionally comparing the two side by side. Closing the choice decides nothing. Leaving with unsaved text SHALL ask the browser to hold the page and save on the way out, except over a conflict.

#### Scenario: Someone saved first
- **WHEN** a save is refused with a conflict
- **THEN** a dialog offers "Tải lại bản mới nhất", "Giữ bản của tôi" and "So sánh", and the server's text is unchanged until one is chosen

#### Scenario: No live co-editing
- **WHEN** two people open the same document
- **THEN** neither sees the other's cursor or typing; the second to save is told of the conflict (live co-editing is a later phase, after realtime)

### Requirement: A document outlives its author and is not lost with its folder
Deleting a user SHALL leave their documents without an owner (shown as an unknown person, and counted as shared). Hard-deleting a folder that still holds documents SHALL be refused by the database rather than destroy them, and SHALL NOT move them to the top of the workspace (which would widen who can read them): until someone deals with them on purpose they are unreachable.

## Status
- A document can be created only at the top of the workspace's Documents from the UI; `folder_id` is supported by the service and not yet offered by a screen.
- There is no version history or restore: only the version counter exists.
- Drive's hard delete of a folder ignores a refused row delete after it has already removed the folder's OA; it should refuse earlier when documents sit in the folder (drive service, not changed here).
- Sharing a single document is not offered: it is shared by sharing its folder.
