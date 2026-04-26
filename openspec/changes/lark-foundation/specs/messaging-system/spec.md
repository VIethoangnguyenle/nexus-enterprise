## MODIFIED Requirements

### Requirement: Send message
A user with `write` permission on the channel Content OA SHALL be able to send a message. Messages are stored in the PostgreSQL `messages` table only (not as NGAC nodes). Messages SHALL support optional `parent_message_id` for threading and optional `linked_entity_type`/`linked_entity_id` for entity linking.

#### Scenario: Send message to channel
- **WHEN** a channel member calls `POST /api/channels/{id}/messages` with content "Hello team"
- **THEN** the system SHALL call `CheckAccess(user, channel_oa, "write")`, and if ALLOW, insert the message into the `messages` table

#### Scenario: Send threaded reply
- **WHEN** a channel member calls `POST /api/channels/{id}/messages` with content and `parent_message_id`
- **THEN** the system SHALL create the reply linked to the parent message and broadcast `thread_reply` event

#### Scenario: Non-member cannot send message
- **WHEN** a non-member attempts to send a message
- **THEN** the access check SHALL return DENY and the message SHALL NOT be stored

## ADDED Requirements

### Requirement: System message types
The Messaging Service SHALL support system-generated messages with type `system` for automated notifications from Kafka events.

#### Scenario: System message from asset event
- **WHEN** the Messaging Service receives an `asset.lifecycle` Kafka event for a channel with notification enabled
- **THEN** it SHALL create a message with type "system", formatted content describing the event, and the service account as sender

#### Scenario: System messages are read-only
- **WHEN** a user attempts to edit or delete a system message
- **THEN** the system SHALL reject with 403 "System messages cannot be modified"

### Requirement: Notification message delivery
The Messaging Service SHALL consume Kafka events from Asset Service and create targeted notifications for relevant users.

#### Scenario: Kafka consumer processes asset event
- **WHEN** an `asset.lifecycle` event is received from Kafka
- **THEN** the Messaging Service SHALL determine affected users via NGAC permission queries, create notification records, and push to connected WebSocket clients
