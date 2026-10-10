# drive-realtime-sync

## Purpose
Keep every connected client's view of drive contents and permissions correct as other users change them, without requiring a refresh.

## Status
Until 2026-10-10 this spec said "Matches code" and did not: `DriveObjectEvent` and `DrivePermEvent` existed in `ws.proto` and the frontend handled them, but no service produced them and messaging consumed no drive topic, so drive never updated live. The divergence is closed: the drive service now emits `drive.events` after each committed change, and the hub delivers them to workspace subscribers as `DomainEvent` frames (`domain = "drive"`). The two old message types were removed from the wire (fields 13 and 14 are reserved). Delivery, scoping, numbering and resynchronisation are specified in `realtime-event-delivery`.

## Requirements

### Requirement: Drive Object Events
The drive service SHALL emit a drive event after a drive item is created (folder, confirmed upload, copy), renamed, restored, moved, trashed, or deleted. Events SHALL include the item id, the containing folder (`parent_id`, empty for the top level), the workspace, the kind (`created`, `updated`, `moved`, `deleted`) and, for a move, the folder the item left (`old_parent_id`). Events SHALL NOT be emitted for a refused or failed change, nor for an upload that is still pending.

#### Scenario: File uploaded by another user
- **WHEN** another user confirms an upload into a shared folder
- **THEN** every session subscribed to the workspace receives a drive event with kind `created` and the folder's listing refetches

#### Scenario: File deleted
- **WHEN** a file is trashed
- **THEN** subscribed sessions receive a drive event with kind `deleted` and the item disappears from their listing

#### Scenario: Item moved
- **WHEN** an item is moved from folder A to folder B
- **THEN** the event names both folders and both listings refetch

### Requirement: Drive Permission Events
The drive service SHALL emit a drive event of kind `share_created` or `share_revoked` when a share is created or revoked. The event SHALL include the item id and workspace. The permission service SHALL additionally emit a `permission` event to every user whose effective permissions an NGAC write can change, so their permission cache is dropped.

#### Scenario: Permission changed
- **WHEN** a share is created or revoked for a drive item
- **THEN** subscribed sessions receive the event
- **AND** their frontend permission cache is invalidated and the item's shares and detail refetch

### Requirement: Frontend Event Handling
The frontend SHALL map each drive event to the query keys it makes stale (through the key factories) and refetch them; it SHALL NOT patch the cache from the event.

#### Scenario: Drive event triggers list refresh
- **WHEN** the frontend receives a drive event for the currently viewed folder
- **THEN** the drive folder query is invalidated and data refetches

#### Scenario: Share event triggers permission refresh
- **WHEN** the frontend receives a share event for a visible item
- **THEN** the permission cache is invalidated
- **AND** the item's action buttons update without full page refresh

### Requirement: Reconnect Resync
When the WebSocket reconnects after a disconnect, the frontend SHALL, once the workspace subscription is acknowledged, invalidate all drive-related queries and clear the permission cache.

#### Scenario: Reconnect after network drop
- **WHEN** WebSocket reconnects after being disconnected
- **THEN** all drive queries are invalidated and the permission cache is cleared
