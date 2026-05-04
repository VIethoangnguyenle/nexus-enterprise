## ADDED Requirements

### Requirement: Drive Object Events
The WebSocket server SHALL emit `DriveObjectEvent` messages when drive items are created, updated, deleted, or moved. Events SHALL include item_id, parent_id, workspace_id, and event_type.

#### Scenario: File uploaded by another user
- **WHEN** another user uploads a file to a shared folder
- **THEN** all connected users in the same workspace receive a DriveObjectEvent with event_type="created"

#### Scenario: File deleted
- **WHEN** a file is trashed
- **THEN** connected users receive DriveObjectEvent with event_type="deleted" and the item disappears from their UI

### Requirement: Drive Permission Events
The WebSocket server SHALL emit `DrivePermEvent` messages when NGAC permissions change for a drive item (share created, share revoked). Events SHALL include item_id and workspace_id.

#### Scenario: Permission changed
- **WHEN** a share is created or revoked for a drive item
- **THEN** connected users in the workspace receive DrivePermEvent
- **AND** their frontend permission cache for that item_id is invalidated

### Requirement: Frontend Event Handling
The frontend WebSocket handler SHALL process drive events by invalidating the relevant TanStack Query cache entries and permission cache entries.

#### Scenario: DriveObjectEvent triggers list refresh
- **WHEN** frontend receives DriveObjectEvent for the currently viewed folder
- **THEN** the drive folder query is invalidated and data refetches

#### Scenario: DrivePermEvent triggers permission refresh
- **WHEN** frontend receives DrivePermEvent for a visible item
- **THEN** the permission cache entry for that item is invalidated
- **AND** the item's action buttons update without full page refresh

### Requirement: Reconnect Resync
When the WebSocket reconnects after a disconnect, the frontend SHALL invalidate all drive-related queries and clear the permission cache.

#### Scenario: Reconnect after network drop
- **WHEN** WebSocket reconnects after being disconnected for >5 seconds
- **THEN** all drive queries are invalidated and permission cache is cleared
