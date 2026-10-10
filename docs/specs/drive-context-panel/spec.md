# drive-context-panel

## Purpose
Give the user file details, sharing, and history without leaving the file list, so inspecting an item never costs a navigation. People are always shown by name and avatar, never by identifier.

## Requirements

### Requirement: Context Panel Layout
The Drive UI SHALL include a right-side detail panel that slides in when an item is selected. The panel SHALL NOT cause a full-page navigation. Below 768px the panel is a full-screen sheet.

#### Scenario: Select file in list
- **WHEN** user activates a file row
- **THEN** the detail panel slides in from the right showing the file's name, kind and size

#### Scenario: Deselect
- **WHEN** user presses Escape or the panel's close button, or leaves the folder
- **THEN** the detail panel slides out

### Requirement: Context Panel Content
The detail panel SHALL show, in one scrolling column: the item's name with its kind and size, the creation and modification times, an image preview when the file is an image, "Người có quyền" and "Hoạt động".

#### Scenario: Metadata
- **WHEN** the panel is open for a file
- **THEN** it shows type, size, created and modified times, formatted in the Vietnamese display formats

#### Scenario: Owner by name
- **WHEN** the panel shows the owner of an item
- **THEN** the owner is a person chip (avatar and display name) labelled "chủ sở hữu"; no user id, node id or other identifier appears. An owner outside the workspace is named by the display name the drive REST returns (`owner_name`)

#### Scenario: Activity names the actor
- **WHEN** the panel lists activity
- **THEN** each entry shows who (avatar and display name), what they did (a verb) and when (relative time, full time in the tooltip). The drive records who created an item and when; later edits are not yet recorded and are not listed

### Requirement: Permission-Gated Sharing
The list of people with access beyond the owner, and the Chia sẻ button, SHALL only be available if the user has `share` permission on the selected item.

#### Scenario: No share permission
- **WHEN** user selects a file where share=false
- **THEN** no share list is requested and no Chia sẻ button is rendered; the owner is still shown

### Requirement: Share Dialog
Sharing SHALL be managed in one dialog, opened from the panel's Chia sẻ button or the row menu. The dialog SHALL list who has access now (by name, with the permission in words and a way to remove it) and let the user pick people by name, with "Có thể xem" or "Có thể sửa". In the panel, a person's permission on an item the user may share SHALL be a dropdown that changes it in place. There SHALL be no field that takes an identifier.

#### Scenario: Share a file
- **WHEN** user picks another person, chooses "Có thể sửa" and presses Chia sẻ
- **THEN** the system creates an NGAC association, emits a permission_changed event, and the person appears in the list

#### Scenario: Change a person's permission
- **WHEN** user picks another permission in a person's dropdown in the panel
- **THEN** the old share is replaced by one with the new permission; if the new one cannot be granted, the old permission is restored and the user is told

#### Scenario: Share to a person is a per-user attribute
- **WHEN** a share names a person (a user node)
- **THEN** the drive resolves that person's personal user attribute (created with the properties `type = personal_ua` and `user_node_id`, containing only that user, on first use) and the association starts from it, never from the user node: an association can only start from a UA
- **AND** the attribute is recognised by its properties and the user being inside it, never by its name, so a role named like it is not reused

#### Scenario: Permission words map to operations on the server
- **WHEN** a share is created with permission `read` or `write`
- **THEN** the association carries `read` for "Chỉ xem" and `read` + `write` for "Có thể sửa" (`ngac.ShareOps`), and the stored share lists those operations
- **AND** any other permission, several permissions, or an operation name such as `manage` or `share` is rejected with InvalidArgument before anything is written

#### Scenario: Sharing requires the share operation
- **WHEN** a caller holding `write` but not `share` on the item creates a share
- **THEN** the request is denied (PermissionDenied) and nothing is written. Workspace Owners and members hold `share` on Documents, and channel members on their channel's drive; the grantee of a share does not

#### Scenario: Trashed item
- **WHEN** a share is created for an item in the trash
- **THEN** the request fails with NotFound and nothing is written

#### Scenario: A refused share leaves nothing behind
- **WHEN** a share target is unknown, is not a person or group, or the association is refused
- **THEN** the request fails and no share OA, assignment or association remains (a target problem is InvalidArgument before any write; a later failure deletes the share OA)

#### Scenario: Revoke a share
- **WHEN** user removes an existing share and confirms
- **THEN** the system removes the NGAC association, emits a permission_changed event, and the person disappears from the list

#### Scenario: Share target without a known person
- **WHEN** a share targets a role or group, or a person missing from the directory
- **THEN** the name shown is the readable part of the server's label with any identifier removed, or a neutral word ("Thành viên", "Nhóm người dùng") if nothing readable is left

### Requirement: Delete Confirmation
Deleting an item SHALL ask for confirmation in the shared confirmation dialog and, once done, SHALL offer "Hoàn tác" in a toast that restores the item.

#### Scenario: Delete and undo
- **WHEN** user confirms deleting an item and then presses Hoàn tác in the toast
- **THEN** the item is removed from the list at once and returns to it after the restore

### Requirement: Permanent Delete Refuses Folders That Hold Text Documents
`DELETE /api/drive/items/:itemId/permanent` SHALL refuse a folder when it, or any folder beneath it, still holds a text document, whatever the document's state. The refusal comes before anything is touched: `409` with `{"error", "reason": "folder_has_documents"}` (gRPC `FailedPrecondition`), the folder row, its files, its stored objects, its quota and its object attribute all stay. For a folder that can be deleted, the row is deleted first and only then are the stored objects removed, the quota released and the folder's object attribute deleted from the graph, so a refusal by the database (a document saved after the check) also leaves the folder whole. The caller's `write` right on the folder is checked before the documents are counted, so a refusal never tells a denied caller what a folder holds. The UI SHALL say that the folder still holds documents and what to do ("Chuyển hoặc xoá các văn bản trước, rồi xoá thư mục"), never the raw reason.

#### Scenario: Folder with a document beneath it
- **WHEN** a caller with `write` permanently deletes a folder whose subfolder holds a text document
- **THEN** the answer is 409 `folder_has_documents` and the folder, the subfolder, the document, every stored object and the folder's graph node are unchanged

#### Scenario: Folder with only files
- **WHEN** a caller with `write` permanently deletes a folder holding only files
- **THEN** the row is gone, the stored objects are removed, the quota is released and the folder's graph node is deleted

#### Scenario: Caller without write
- **WHEN** a caller without `write` deletes a folder that holds documents
- **THEN** the answer is 403, not 409
