## ADDED Requirements

### Requirement: Notification WebSocket events
The WebSocket connection SHALL support notification events in addition to chat messages.

#### Scenario: Notification event delivery
- **WHEN** a notification is created for a connected user
- **THEN** the system SHALL send a `notification` event via WebSocket with type, title, body, entity reference, and timestamp

#### Scenario: Notification count sync
- **WHEN** a user establishes a WebSocket connection
- **THEN** the system SHALL send the current unread notification count as an initial `notification_count` event

### Requirement: Asset event subscription
Connected WebSocket clients SHALL receive real-time updates for asset state changes relevant to their permissions.

#### Scenario: Asset state change broadcast
- **WHEN** an asset undergoes a state transition
- **THEN** all connected users with `read` permission on the asset SHALL receive an `asset_updated` WebSocket event with asset_id, new_state, and actor

#### Scenario: Scoped delivery
- **WHEN** an asset event occurs
- **THEN** only users with NGAC `read` permission on the asset's OA SHALL receive the WebSocket event (not all connected users)
