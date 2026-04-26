## ADDED Requirements

### Requirement: Channel creation
A user with `create_channel` permission on the workspace Channels OA SHALL be able to create a new channel. Channel creation SHALL generate:
- A `Channel_{name}_Content` OA assigned to the workspace Channels OA
- A channel Object (O) assigned to the Content OA
- A `Channel_{name}_Members` UA assigned to the workspace PC
- An Association: `Channel_{name}_Members` → `Channel_{name}_Content` with [read, write]
- A `channels` DB row linking the channel to its NGAC nodes
- Creator auto-assigned to `Channel_{name}_Members`

#### Scenario: Create workspace channel
- **WHEN** a workspace member with `create_channel` permission calls `POST /api/workspaces/{id}/channels` with name "engineering"
- **THEN** a channel SHALL be created with the correct NGAC scaffolding and the creator SHALL be auto-joined

#### Scenario: Cannot create channel without permission
- **WHEN** a user without `create_channel` permission attempts to create a channel
- **THEN** the request SHALL be denied with 403 Forbidden

### Requirement: Channel membership management
Channel access SHALL be controlled by assignment to the channel's Members UA. Only users with `manage` permission or the channel creator SHALL be able to add/remove members.

#### Scenario: Add member to channel
- **WHEN** an admin calls `POST /api/channels/{id}/members` with user_id
- **THEN** the target user's NGAC node SHALL be assigned to `Channel_{name}_Members` UA

#### Scenario: Remove member from channel
- **WHEN** an admin calls `DELETE /api/channels/{id}/members/{userId}`
- **THEN** the target user's assignment to `Channel_{name}_Members` SHALL be removed and they SHALL lose channel access

#### Scenario: Non-member cannot access channel
- **WHEN** a user who is NOT assigned to `Channel_{name}_Members` attempts to read messages
- **THEN** `CheckAccess(user, channel_oa, "read")` SHALL return DENY

### Requirement: Send message
A user with `write` permission on the channel Content OA SHALL be able to send a message. Messages are stored in the PostgreSQL `messages` table only (not as NGAC nodes).

#### Scenario: Send message to channel
- **WHEN** a channel member calls `POST /api/channels/{id}/messages` with content "Hello team"
- **THEN** the system SHALL call `CheckAccess(user, channel_oa, "write")`, and if ALLOW, insert the message into the `messages` table

#### Scenario: Non-member cannot send message
- **WHEN** a non-member attempts to send a message
- **THEN** the access check SHALL return DENY and the message SHALL NOT be stored

### Requirement: Read messages
A user with `read` permission on the channel Content OA SHALL be able to retrieve messages from that channel.

#### Scenario: Read channel messages
- **WHEN** a channel member calls `GET /api/channels/{id}/messages`
- **THEN** the system SHALL verify `CheckAccess(user, channel_oa, "read")` and return paginated messages ordered by creation time

#### Scenario: Pagination support
- **WHEN** a user requests messages with `?before={timestamp}&limit=50`
- **THEN** the system SHALL return up to 50 messages created before the given timestamp

### Requirement: Direct Messages
A user SHALL be able to create a DM channel with another registered user. DM channels SHALL use `PC_Global` as their Policy Class to support cross-workspace messaging.

#### Scenario: Create DM
- **WHEN** a user calls `POST /api/dm` with target_user_id
- **THEN** the system SHALL create a DM channel with Content OA and Members UA under PC_Global, assigning both users to the Members UA

#### Scenario: DM deduplication
- **WHEN** a user creates a DM with someone they already have a DM channel with
- **THEN** the system SHALL return the existing DM channel instead of creating a duplicate

#### Scenario: DM only visible to participants
- **WHEN** a third user attempts to access a DM channel between two other users
- **THEN** `CheckAccess` SHALL return DENY because the third user is not in the DM's Members UA

### Requirement: Channel listing
A user SHALL be able to list channels they have access to within a workspace.

#### Scenario: List my channels
- **WHEN** a user calls `GET /api/workspaces/{id}/channels`
- **THEN** the response SHALL contain only channels where the user is assigned to the channel's Members UA

#### Scenario: List DMs
- **WHEN** a user calls `GET /api/dm`
- **THEN** the response SHALL contain all DM channels where the user is a member

### Requirement: Workspace boundary enforcement
Channel access SHALL respect workspace boundaries — a channel created in Workspace A SHALL NOT be accessible to users who only have roles in Workspace B.

#### Scenario: Cross-workspace isolation
- **WHEN** a user with roles only in Workspace B attempts to access a channel in Workspace A
- **THEN** the access check SHALL return DENY because the user has no UA reaching Workspace A's PC
