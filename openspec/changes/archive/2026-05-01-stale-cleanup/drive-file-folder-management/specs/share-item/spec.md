# S2: Share File/Folder with Workspace Members

## User Story
As a workspace member, I want to share files and folders with other workspace members so they can access my content.

## Acceptance Criteria

### AC-1: Share Modal Opens
- GIVEN a file or folder in Drive
- WHEN user clicks "Share" in the context menu
- THEN a Share modal opens with the item name in the title
- AND it shows a search input for workspace members
- AND it shows the current shares for this item

### AC-2: Search and Add Share
- GIVEN the Share modal is open
- WHEN user types in the search input
- THEN workspace contacts are filtered by name/email
- AND each contact shows display name, email, and avatar initial
- AND contacts already shared with show "Shared" badge
- AND non-shared contacts show "Share" button

### AC-3: Create Share
- GIVEN a non-shared workspace member in the Share modal
- WHEN user clicks "Share" button on that member
- THEN `POST /drive/items/:itemId/share` is called with:
  - `target_node_id` = member's `ngac_node_id`
  - `share_type` = "user"
  - `permission` = "read" (default)
- AND the shares list updates optimistically
- AND the member shows "Shared" badge

### AC-4: Revoke Share
- GIVEN a shared member in the current shares list
- WHEN user clicks the revoke (X) button
- THEN a confirmation is shown
- AND on confirm, `DELETE /drive/shares/:shareId` is called
- AND the share is removed optimistically

### AC-5: Permission Level
- GIVEN the Share modal
- WHEN user selects permission level (read or write)
- THEN the share is created with the selected permission

### AC-6: Shared With Me
- GIVEN files have been shared with the current user
- WHEN user clicks "Shared with me" in the Drive sidebar
- THEN `GET /drive/shared-with-me` returns the shared items
- AND they are displayed in the file list

## UI Flow
1. User clicks "Share" on a Drive item
2. `ShareDialog` opens:
   - Header: "Share [item name]"
   - Current shares section (with revoke buttons)
   - Search + workspace member picker
   - Permission selector (read/write dropdown)
3. User searches, selects member → clicks "Share"
4. Share appears in current shares list

## Existing Components/Hooks Used
- `useCreateShare(itemId)` — exists
- `useRevokeShare(itemId)` — exists
- `useDriveShares(itemId)` — exists
- `useContacts(wsId)` — workspace member data source
- `Modal` — compound component for dialog shell
- `ConfirmDialog` — for revoke confirmation
- `Button`, `Spinner`, `Input` — primitives

## New Component
- `ShareDialog` — composite using Modal + useContacts + useDriveShares
