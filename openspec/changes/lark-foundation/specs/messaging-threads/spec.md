## ADDED Requirements

### Requirement: Create message thread
A user SHALL be able to reply to a message, creating a thread. Thread replies SHALL be scoped to the parent message and not appear in the main channel feed.

#### Scenario: Reply to message
- **WHEN** a user calls `POST /api/channels/{id}/messages` with `parent_message_id` set to an existing message ID
- **THEN** the system SHALL create the reply linked to the parent, increment the parent's reply_count, and broadcast a `thread_reply` event via WebSocket to thread participants

#### Scenario: Thread replies not in main feed
- **WHEN** a user fetches channel messages via `GET /api/channels/{id}/messages`
- **THEN** thread replies (messages with non-null `parent_message_id`) SHALL NOT appear in the main message list

#### Scenario: Get thread messages
- **WHEN** a user calls `GET /api/messages/{id}/thread`
- **THEN** the system SHALL return all replies to that message ordered by creation time, with the parent message included

### Requirement: Link thread to entity
A message thread SHALL support linking to an external entity (asset, document, etc.) for contextual discussion.

#### Scenario: Create asset-linked thread
- **WHEN** a user calls `POST /api/channels/{id}/messages` with `linked_entity_type: "asset"` and `linked_entity_id: "{asset_id}"`
- **THEN** the system SHALL create the message with entity link and the thread becomes the discussion context for that asset

#### Scenario: Auto-post on asset state change
- **WHEN** an asset linked to a thread undergoes a state transition
- **THEN** the system SHALL post a system message in the linked thread describing the state change (e.g., "MacBook Pro #042 was approved by @admin")

#### Scenario: Find threads by entity
- **WHEN** a user calls `GET /api/threads?entity_type=asset&entity_id={id}`
- **THEN** the system SHALL return all message threads linked to that asset, across all channels

### Requirement: Thread participant tracking
The system SHALL track thread participants (users who replied or were mentioned) for notification purposes.

#### Scenario: Auto-subscribe on reply
- **WHEN** a user replies to a thread
- **THEN** the user SHALL be added to the thread's participant list and receive notifications for future replies

#### Scenario: Thread notification
- **WHEN** a new reply is posted to a thread
- **THEN** all thread participants who are not the author SHALL receive a `thread_reply` notification
