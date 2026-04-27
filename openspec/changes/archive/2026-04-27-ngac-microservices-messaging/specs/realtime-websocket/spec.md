## ADDED Requirements

### Requirement: WebSocket connection
The system SHALL provide a WebSocket endpoint at `/api/ws` for real-time communication. The connection SHALL be authenticated via JWT passed as a query parameter.

#### Scenario: Authenticated WebSocket connection
- **WHEN** a client connects to `ws://host/api/ws?token={jwt}`
- **THEN** the Gateway SHALL validate the JWT and proxy the WebSocket to the Messaging Service

#### Scenario: Unauthenticated WebSocket rejected
- **WHEN** a client connects without a valid JWT
- **THEN** the connection SHALL be rejected with appropriate error

### Requirement: Real-time message delivery
When a message is sent to a channel, all connected members of that channel SHALL receive the message in real-time via their WebSocket connection.

#### Scenario: Message broadcast
- **WHEN** user A sends a message to #engineering
- **THEN** all users connected via WebSocket who are members of #engineering SHALL receive the message within 100ms

#### Scenario: Non-member does not receive
- **WHEN** a message is sent to a channel
- **THEN** users who are NOT members of that channel SHALL NOT receive the message via WebSocket

### Requirement: Channel subscription management
The Messaging Service SHALL automatically subscribe connected users to channels they have access to.

#### Scenario: Auto-subscribe on connect
- **WHEN** a user establishes a WebSocket connection
- **THEN** the service SHALL determine which channels the user can access (via NGAC) and subscribe them to those channels

#### Scenario: Dynamic subscription update
- **WHEN** a user is added to a new channel while connected
- **THEN** the user SHALL receive a `channel_joined` event and begin receiving messages from that channel

### Requirement: Typing indicators
The system SHALL support typing indicator events between channel members.

#### Scenario: Typing broadcast
- **WHEN** a user sends a `typing` event for a channel
- **THEN** all other connected members of that channel SHALL receive a `user_typing` event with the user's ID

### Requirement: Connection resilience
The frontend SHALL implement automatic WebSocket reconnection with exponential backoff.

#### Scenario: Reconnect after disconnect
- **WHEN** the WebSocket connection drops
- **THEN** the client SHALL attempt to reconnect with exponential backoff (1s, 2s, 4s, 8s, max 30s)

#### Scenario: State recovery after reconnect
- **WHEN** the client successfully reconnects
- **THEN** it SHALL re-subscribe to all channels and fetch missed messages via REST API
