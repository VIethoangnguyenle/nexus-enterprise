# drive-tree-navigation

## Purpose
Let the user see and traverse the folder hierarchy without loading the whole tree, while never showing a folder they cannot read. Where the user is lives in the URL, so reload, Back/Forward and a pasted link all land in the same folder.

## Requirements

### Requirement: Folder Tree Sidebar
The Drive UI SHALL render a folder tree in the list panel (column 2, desktop widths) showing the workspace's folder hierarchy under the workspace name. The tree SHALL lazy-load children on expand. Below 1024px the list panel is not shown; the breadcrumb and the folder rows are the way to move.

#### Scenario: Initial tree load
- **WHEN** user navigates to Drive
- **THEN** the tree shows root-level folders only (children not loaded)

#### Scenario: Expand folder
- **WHEN** user clicks the expand arrow on a folder node
- **THEN** children folders are fetched and rendered under the parent, indented 16px per level

#### Scenario: Collapse folder
- **WHEN** user clicks the collapse arrow on an expanded folder
- **THEN** children are hidden (but cached, not refetched on re-expand)

#### Scenario: Keyboard
- **WHEN** a tree item has focus and the user presses ↑/↓, →, ← or Enter
- **THEN** focus moves between visible items, → opens a closed node or steps into an open one, ← closes an open node or steps out to the parent, and Enter opens the folder

### Requirement: Folder Path in the URL
The open folder SHALL be part of the URL (`/drive?folder=<folder id>`; no `folder` means the workspace root), together with the workspace (`ws`) and the shared view (`view=shared`). The Drive UI SHALL NOT keep the open folder in a client store. The breadcrumb SHALL be built from the path the server returns for the folder and every ancestor in it SHALL be a link into the URL. A folder id SHALL NOT be shown on screen.

#### Scenario: Reload keeps the folder
- **WHEN** user reloads the page on `/drive?folder=<id>`
- **THEN** the same folder is listed, its breadcrumb is shown, and the tree has its ancestors open

#### Scenario: Back and Forward
- **WHEN** user opens a subfolder and then presses Back
- **THEN** the previous folder is listed again; Forward returns to the subfolder

#### Scenario: Switching workspace
- **WHEN** user switches workspace
- **THEN** the new workspace opens at its root (the `folder` and `view` parameters are dropped)

#### Scenario: Folder that cannot be opened
- **WHEN** the URL names a folder that is gone or that the user may not read
- **THEN** the list says so and offers a link back to the root

#### Scenario: Shared with me
- **WHEN** user opens "Được chia sẻ với tôi"
- **THEN** the URL carries `view=shared` and the table lists the items shared with the user, with each owner shown by name

### Requirement: Folders That Cannot Be Opened Or Filled
The drive SHALL answer NotFound for a folder that is trashed, for a parent folder that belongs to another workspace than the one in the route, and for a parent that is not a folder, in `ListFolder`, `CreateFolder` and `CreateFile`. `GetItem` answers NotFound for a trashed item. Restoring an item makes it open again.

#### Scenario: Trashed folder
- **WHEN** a folder is trashed and its id is listed, opened, or used as the parent of a new folder or file
- **THEN** the call fails with NotFound and nothing is written

#### Scenario: Parent from another workspace
- **WHEN** `CreateFolder` or `CreateFile` (routes under `/api/workspaces/:id/drive/`), or `ListFolder` when the caller names the workspace (`GET /api/drive/folders/:folderId?ws=<workspace id>`), is called for workspace A with a folder id from workspace B
- **THEN** the call fails with NotFound and nothing is written
- **AND** `GET /api/drive/folders/:folderId` without `ws` names no workspace to compare, so it is authorized by the `read` check on the folder itself

### Requirement: Moving Folders Keeps The Graph Connected
`MoveItem` SHALL refuse, before any policy write, a destination that is the item itself or inside it (InvalidArgument), is not an active folder, or lies in another workspace or drive context. It SHALL treat an empty destination as the top level of the item's drive (under the drive root OA). A moved folder SHALL be re-assigned from its current parent OA (its parent folder's, or the root's for a top-level folder) to the destination OA by making the new assignment first and removing the old one after, so the folder never reaches fewer policy classes than it should (a shared folder detached from its workspace reaches only PC_Global); if the old assignment cannot be removed the new one is withdrawn, and a failure that cannot be repaired is returned as an error. Moves of one item SHALL be serialised (advisory lock per item), the item SHALL be read again under the lock, and the row SHALL change only if it is still under the parent the move started from, in a single statement together with the item's authorizing OA; a move that loses that race undoes its assignments and fails with Aborted. A trashed item cannot be moved (NotFound).

#### Scenario: Folder into itself or a descendant
- **WHEN** a folder is moved into itself or into one of its subfolders
- **THEN** the call fails with InvalidArgument, no assignment is removed or added, and the subtree stays attached

#### Scenario: New assignment refused
- **WHEN** the policy service refuses the new assignment (for example it would create a cycle)
- **THEN** the old assignment was never removed and the item's row is unchanged

#### Scenario: Old assignment cannot be removed
- **WHEN** the new assignment is made but the old one cannot be removed
- **THEN** the new assignment is withdrawn, the row is unchanged, and an error is returned

#### Scenario: Concurrent moves of one item
- **WHEN** two moves of the same item to different destinations run at once
- **THEN** the item ends under exactly one parent in the graph, the one its row records; a move that loses may only fail with Aborted

#### Scenario: Move to the top level
- **WHEN** a folder is moved with an empty destination
- **THEN** it has no parent row and hangs under the drive root OA

### Requirement: Folder Walks Are Bounded
The breadcrumb and the ancestor check used by moves SHALL stop after 64 levels, so a `parent_id` cycle ends the query instead of running until the statement times out.

### Requirement: Tree State Persistence
The folder tree SHALL persist expand/collapse state in the Zustand store, surviving route transitions within the workspace session. Opening a folder through the URL SHALL also open its ancestors.

#### Scenario: Navigate away and back
- **WHEN** user navigates from Drive to Messaging and back to Drive
- **THEN** previously expanded folders remain expanded

### Requirement: Active Path Highlighting
The tree SHALL mark the currently open folder (`aria-current`) and give all its ancestors a stronger weight than their siblings.

#### Scenario: Deep folder selected
- **WHEN** user navigates to /Projects/Q4/Reports
- **THEN** "Reports" is marked current, and "Projects" and "Q4" are emphasised

### Requirement: Permission-Aware Tree
The folder tree SHALL NOT display folders the user cannot read. For workspace members, all workspace folders are readable by default.

#### Scenario: Shared folder not in workspace
- **WHEN** a user has been shared a specific subfolder but not its parent
- **THEN** only the shared folder appears in the tree (not the inaccessible parent hierarchy)

### Requirement: A workspace's drive root is its recorded Documents OA
The drive root of a workspace SHALL hang on the Documents OA recorded on the workspace (`workspaces.documents_oa_id`, set when the workspace is provisioned). The drive SHALL NOT work the root out from node names — not by looking for "Documents" or "Docs" among the policy class's children, and not by taking the first OA found. A workspace with no Documents OA recorded SHALL get a root OA of its own, `ngac.DriveRootName(workspace id)`, which is found again on later calls instead of created twice. A drive root is marked (`drive_items.is_root`), because top-level user folders are parent-less too; at most one active root exists per workspace, drive context and context id (unique index), and a request that loses the race to create it SHALL read the winner's root again and use it. A channel's drive SHALL be built only for a channel of the requesting workspace (or for the workspace's own id, its root-drive context); a request pairing one workspace with another's channel is refused (PermissionDenied) before anything is written, and an OA that is already another workspace's drive root is never adopted. Folders and shares created in the drive SHALL be named by their own IDs (`ngac.FolderNodeName`, `ngac.ShareOAName`), and a channel's drive by the channel's ID (`ngac.ChannelDriveName`), with what the screen shows kept in the item name and the node's `display_name` property.

#### Scenario: Recorded Documents OA
- **WHEN** the first top-level folder is created in a workspace that has a Documents OA recorded
- **THEN** the root and the folder hang under that OA, whatever other OAs the policy class has and whatever they are called

#### Scenario: Nothing recorded
- **WHEN** the workspace has no Documents OA recorded
- **THEN** a `DriveRoot_{workspace id}` OA is used (found, or created once), and no OA is chosen by name or by order

#### Scenario: Two channels with one name
- **WHEN** two channels, in one workspace or two, are both called "general" and each gets a drive
- **THEN** the drives are two OAs, each named by its channel's ID, and each drive shows "general"

#### Scenario: Drive provisioning fails part way
- **WHEN** a step after the drive's OA is created fails
- **THEN** the OA this call created is deleted and no drive row exists; an OA that was already there is not deleted

#### Scenario: Many first requests at once
- **WHEN** several requests that each need a workspace's drive root arrive together for a workspace that has none
- **THEN** exactly one root row exists afterwards and every request is served

#### Scenario: Top-level folders are not roots
- **WHEN** a workspace holds any number of parent-less user folders
- **THEN** they are accepted, and only the one marked root is unique per context

#### Scenario: Another workspace's channel
- **WHEN** workspace B asks for a drive for a channel that belongs to workspace A, or for a channel that does not exist
- **THEN** the request is refused with PermissionDenied and no node, association or row is written

