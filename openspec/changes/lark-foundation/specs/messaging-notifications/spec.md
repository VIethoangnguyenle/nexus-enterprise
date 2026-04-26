## ADDED Requirements

### Requirement: Asset event notifications
The Messaging Service SHALL consume Kafka asset events and create notifications for relevant users based on their NGAC permissions and roles.

#### Scenario: Asset request notification
- **WHEN** an employee submits an asset request (Kafka: `asset.request`)
- **THEN** the system SHALL create notifications for all users with `approve` permission on that asset type within the workspace

#### Scenario: Asset approved notification
- **WHEN** an admin approves an asset request (Kafka: `asset.request` status=approved)
- **THEN** the system SHALL create a notification for the requester with the approval details

#### Scenario: Asset assigned notification
- **WHEN** an asset is assigned to a user (Kafka: `asset.assignment`)
- **THEN** the system SHALL create a notification for the assigned user with asset details

### Requirement: Notification storage and retrieval
Notifications SHALL be stored in a dedicated table and retrievable via API with unread tracking.

#### Scenario: List notifications
- **WHEN** a user calls `GET /api/notifications`
- **THEN** the system SHALL return the user's notifications ordered by timestamp descending, with unread count in response header

#### Scenario: Mark notification as read
- **WHEN** a user calls `POST /api/notifications/{id}/read`
- **THEN** the system SHALL mark the notification as read and decrement the unread count

#### Scenario: Mark all as read
- **WHEN** a user calls `POST /api/notifications/read-all`
- **THEN** the system SHALL mark all unread notifications as read

### Requirement: Real-time notification delivery
Notifications SHALL be delivered in real-time to connected users via WebSocket.

#### Scenario: Online user receives instant notification
- **WHEN** a notification is created for a user who is connected via WebSocket
- **THEN** the system SHALL push the notification to the user's WebSocket connection within 500ms

#### Scenario: Offline user sees on next load
- **WHEN** a user logs in after missing notifications
- **THEN** the unread notification count SHALL be displayed and the user can view all missed notifications

### Requirement: Notification channel posting
Asset events SHALL also be posted as system messages in relevant workspace channels when configured.

#### Scenario: Asset event posted to channel
- **WHEN** an asset lifecycle event occurs and the workspace has a configured notification channel (e.g., #asset-updates)
- **THEN** the system SHALL post a formatted system message to that channel with event details
