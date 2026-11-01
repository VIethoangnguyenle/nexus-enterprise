# drive-permission-engine

## Purpose
Keep the drive UI's permission answers fast and consistent, so actions appear only when genuinely allowed and never flicker between allowed and denied.

## Status

Verified against code 2026-08-02. Requirements below match `frontend/src/hooks/usePermissions.ts`
and `frontend/src/stores/permission.store.ts` — batching, coalescing, the 60s TTL, and the
permission shape are all as specified.

The previously open divergence on cache keying is resolved: the store now keys by
`{tenantId}:{objectId}`, with the tenant read from the access token's `tenant_id` claim so it
cannot drift from the tenant the request actually runs as. Tenant isolation no longer depends on
`clear()` firing at the right moment.

## Requirements

### Requirement: Permission Cache
The frontend SHALL maintain an in-memory permission cache keyed by `{tenantId}:{objectId}` with a configurable TTL (default 60 seconds). The cache SHALL store resolved permissions (read, write, share) as booleans.

The tenant component SHALL be derived from the current access token rather than tracked separately, so that a tenant switch or a token refresh cannot leave the key scoped to a stale tenant.

#### Scenario: Same object under two tenants
- **WHEN** the same object ID is resolved under tenant A and later under tenant B
- **THEN** the two results occupy separate cache entries
- **AND** a failure to clear the cache on switch yields a stale-within-tenant read, never one tenant reading another tenant's answer

#### Scenario: Cache hit within TTL
- **WHEN** a component requests permissions for an object that was batch-checked less than 60 seconds ago
- **THEN** the cached permissions are returned synchronously without a network call

#### Scenario: Cache miss triggers batch fetch
- **WHEN** a component requests permissions for an object not in cache or with expired TTL
- **THEN** the permission engine queues the object for the next batch fetch cycle

#### Scenario: Tenant switch clears cache
- **WHEN** the user switches tenant context
- **THEN** the entire permission cache is cleared immediately

### Requirement: Batch Permission Hook
The frontend SHALL provide a `usePermissions(objectIds)` hook that batches permission checks for visible items. The hook SHALL coalesce multiple concurrent calls into a single batch request.

#### Scenario: File list renders with permissions
- **WHEN** a folder is loaded containing 50 items
- **THEN** `usePermissions` sends a single batch request for all 50 object IDs and returns a map of permissions

#### Scenario: Incremental items (scroll)
- **WHEN** user scrolls to reveal 20 new items in a virtualized list
- **THEN** `usePermissions` batch-checks only the newly visible items (not already-cached ones)

### Requirement: Permission-Aware Action Rendering
The UI SHALL render file/folder actions (edit, share, delete) only when the corresponding permission is `true`. The UI SHALL NOT render any action before permissions are resolved.

The UI SHALL only ask the policy service about operations the backend actually evaluates. There is no `delete` right: trashing, restoring and permanent deletion are all enforced as `write` on the item's object attribute, so the delete action is gated on `write`. Asking for a `delete` operation returns DENY for every user — including those who can genuinely delete — and hides the action from them.

#### Scenario: User with write permission sees edit action
- **WHEN** user has write=true for a file
- **THEN** the edit/rename action is visible on hover

#### Scenario: User without write permission
- **WHEN** user has write=false for a file
- **THEN** the trash/delete action is not rendered (no disabled state, completely hidden)

#### Scenario: Permissions loading state
- **WHEN** permissions are being fetched for newly visible items
- **THEN** action buttons are not rendered until permissions resolve (skeleton or hidden)

### Requirement: Confirming an upload is idempotent
`POST /api/drive/files/{id}/confirm` SHALL publish a pending file and charge its size to the workspace quota as one change. A second confirmation of the same file finds nothing pending: it answers 409 and charges nothing, so a client that retries cannot be billed twice. Removing a file that was never confirmed releases no quota, because none was taken. If the object store has no such object the answer is 409 with the fixed message `file not uploaded`; if the store or the document service fails for any other reason the answer is a generic 500, and the failure's text (hosts, buckets) goes to the log only.

The upload URL SHALL take exactly the size declared when the upload was started: the size is signed into it as the request's `Content-Length`, which the object store checks (any other length is refused with 403), so the stored object can neither outgrow the quota checked at creation nor be replaced by a bigger one through the same URL. A negative size is refused (400). Browsers send a file's own length, so a client that declares `file.size` needs nothing else. Uploads the server makes itself (previews, thumbnails) name no size.

The quota SHALL be held to the size the object store reports at confirmation, not the size declared when the upload was started: an upload that would take the workspace past its bytes or files limit is refused with 413 `quota_exceeded`, its row and its stored object are removed, and nothing is charged. A copy is held to the same check when it is published.

#### Scenario: Declared small, uploaded large
- **WHEN** a client declares 0 bytes, uploads more than the room left and confirms
- **THEN** the answer is 413 `quota_exceeded`, the file and its object are gone and used bytes did not change

#### Scenario: Confirmed twice
- **WHEN** a client confirms the same upload twice
- **THEN** the first answer is 200 and the second 409, and the workspace's used bytes and used files rose once

#### Scenario: Object store unreachable while confirming
- **WHEN** the document service cannot reach the object store
- **THEN** the answer is 500 `internal error`, the file stays pending, and no host name appears in the body

### Requirement: Downloads are of live files and never run in the browser
`GET /api/drive/files/{id}/download` SHALL answer 404 for a trashed file, a pending upload and a folder. The signed
URL SHALL keep the object inline only when its stored type, compared by its essence (the part before any `;`, trimmed
and lower-cased, as a browser reads it), is one the file viewer shows inline and that cannot run script: PNG, JPEG, GIF,
WebP, AVIF and BMP images, PDF, video, audio and plain text, and its name is not an HTML, SVG, XML or script name.
Everything else, malformed, empty and unreadable types included, SHALL be signed with
`response-content-disposition=attachment` and `response-content-type=application/octet-stream`. The viewer fetches
other text kinds and shows them as text, which an attachment does not affect.

#### Scenario: Uploaded HTML page
- **WHEN** a member uploads `x.html` (or an SVG with script) and opens its download link
- **THEN** the storage host answers it as an `application/octet-stream` attachment, and nothing in it runs

### Requirement: A full quota and a lost race are refusals with a reason
Whether the caller may write comes first: a caller without write on the destination is refused (403, gRPC `PermissionDenied`) whatever room is left, and so learns nothing about the workspace's usage. When creating a file would take the workspace past its storage quota (bytes or number of files), `POST /api/workspaces/{id}/drive/files` SHALL answer `413 {"message":"storage quota exceeded","reason":"quota_exceeded"}`; over gRPC the code is `ResourceExhausted`. When another request changed the item first (a move that lost the race for the same item), the answer SHALL be `409 {"message":"item was changed by another request; retry","reason":"item_changed"}` (gRPC `Aborted`). A lock that could not be had in time stays a generic 500. Only the class of the error decides the answer: a database error whose text mentions the quota is still a 500.

#### Scenario: Upload over the quota
- **WHEN** a member asks to create a file larger than the room the workspace has left
- **THEN** the answer is 413 with reason `quota_exceeded`, no item is created and nothing is charged

#### Scenario: Two moves of one item
- **WHEN** two requests move the same item at once
- **THEN** the loser is told 409 `item_changed` and may retry

### Requirement: A copy takes what an upload to its destination takes
`POST /api/drive/items/{itemId}/copy` (gRPC `CopyItem`) SHALL copy an active file only when the caller has `read`
on the source and `write` on the destination, in that order, then room in the quota; the copy is charged to the
quota when it is published, exactly like a confirmed upload. The destination is a folder of the **same workspace**
as the source (any of its drives, e.g. a channel drive); an empty `target_folder_id` or a drive root means the top
level of that drive (no parent, the drive's own context). Where the copy lands — workspace, drive, parent, OA — is
taken from the destination and never from the request. A name already used in the destination becomes
"Bản sao của <name>", then "Bản sao của <name> (2)" and on; a name taken by a concurrent request at publish time is
409. A copy that fails after its object was stored removes the object and the row, even when the client has hung up.

#### Scenario: No write on the destination
- **WHEN** a member who may read the file but not write the destination folder asks to copy it there
- **THEN** the answer is 403 and nothing is stored or charged

#### Scenario: Destination in another workspace
- **WHEN** the destination folder belongs to another workspace, whatever rights the caller holds
- **THEN** the answer is 400 and nothing lands in that folder

#### Scenario: Request names another workspace
- **WHEN** a gRPC caller sets `dest_workspace_id` or `dest_drive_context` to something else
- **THEN** they are ignored and the copy belongs to the destination folder's workspace and drive

#### Scenario: Copy over the quota
- **WHEN** the copy would take the workspace past its quota
- **THEN** the answer is 413 `quota_exceeded`, after the rights checks, and nothing is stored

#### Scenario: Copy into the same folder
- **WHEN** a file is copied into the folder that already holds it
- **THEN** the copy is called "Bản sao của <name>", and a second copy "Bản sao của <name> (2)"

#### Scenario: Gone or wrong destination and source
- **WHEN** the destination is trashed or missing, or the source is trashed or still pending
- **THEN** the answer is 404; a destination that is a file is 400

### Requirement: A star is personal and grants nothing
`drive_stars` (migration `070_drive_stars.sql`) holds one row per person per starred item. `POST
/api/drive/items/:itemId/star` stars an active item for the caller only when the caller is an active member of **the
item's own workspace** and may read the item now; nothing in the request names the workspace. `DELETE` on the same
route removes the caller's own star and needs only a signed-in caller, so a star on an item that can no longer be
read can still be taken off. A star never changes who may open the item.

#### Scenario: Star an unreadable item
- **WHEN** the policy denies read on the item's folder
- **THEN** the answer is 403 and nothing is written

#### Scenario: Star an item of another workspace
- **WHEN** the caller is not a member of the workspace the item belongs to, even if the policy would allow read
- **THEN** the answer is 403 and nothing is written

#### Scenario: Trashed, pending or missing
- **WHEN** the item is trashed, still uploading or does not exist
- **THEN** the answer is 404

#### Scenario: Someone else's star
- **WHEN** a person unstars an item another person also starred
- **THEN** only their own star goes

### Requirement: Lists built from stars, shares and recency re-check read
`GET /api/drive/starred?workspace_id=` and `GET /api/drive/quick-access?workspace_id=` answer 403 to a caller who is
not an active member of `workspace_id`, return only items of that workspace, and re-check read on every item with one
batch check each time they are read. A failed check fails the request (fail closed, no partial list).

Truy cập nhanh returns up to six active files, each with one reason: **shared** (someone else shared it with the
caller or a group of theirs; the sharer is named, never sent as an id), **starred**, or **yours** (the caller put it
there), in that order of strength, newest reason first. Mentions in conversations are not a reason: the drive holds
no record of them.

#### Scenario: Read taken away
- **WHEN** an item was starred or shared while readable and read on its folder is later removed
- **THEN** it is no longer listed, and neither is its reason or who shared it

#### Scenario: Another workspace
- **WHEN** a starred, shared or owned item belongs to a workspace other than `workspace_id`
- **THEN** it is never listed, even if readable

#### Scenario: Unrelated file
- **WHEN** a readable file was neither shared with, starred by nor put there by the caller
- **THEN** it is not in Truy cập nhanh

#### Scenario: A share I made
- **WHEN** the caller is the one who created the share
- **THEN** the file is not offered as "shared"

### Requirement: Multi-select acts only where every item allows it
On a phone the selection bar enables an action only when every chosen item allows it, from the same batch answers as
the rows: Chia sẻ needs exactly one item with share; Chuyển and Xoá need write on every item; Tải xuống needs only
files; Gắn sao is always offered. A refused action stays dimmed and says why when tapped; each item's change is still
checked by the server.

### Requirement: Stale pending uploads are removed
An upload row left `pending` more than 24 hours after it was started (the client never confirmed it) is removed by the
drive service at start and then hourly, at most 500 a pass, together with its stored object. The row is deleted only
while still pending, so an upload confirmed meanwhile is kept. The object is deleted on behalf of the person who
started the upload; a pending row was never charged to the quota.

#### Scenario: Stale and fresh
- **WHEN** one upload has been pending for 25 hours and another for a minute
- **THEN** the first row and its object are removed, the second is left
