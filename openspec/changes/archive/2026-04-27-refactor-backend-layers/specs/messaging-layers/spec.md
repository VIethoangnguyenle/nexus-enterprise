## ADDED Requirements

### Requirement: Store layer owns all SQL operations
The Messaging service SHALL have a `store` package (`internal/store/`) that owns all database queries. The gRPC handler MUST NOT contain SQL strings or direct `db.Query`/`db.Exec` calls.

#### Scenario: Inserting a channel
- **WHEN** a new channel is created
- **THEN** the SQL INSERT MUST execute via `store.InsertChannel()`, not inline in the handler

#### Scenario: Listing messages
- **WHEN** messages are requested for a channel
- **THEN** the paginated SQL query MUST execute via `store.ListMessages()` with proper parameters

### Requirement: Domain layer orchestrates business logic
The Messaging service SHALL have a `domain` package (`internal/domain/`) that orchestrates between store, NGAC policy, and external services. The gRPC handler MUST only parse requests, validate input, delegate to domain, and return responses.

#### Scenario: Creating a channel
- **WHEN** `CreateChannel` gRPC is called
- **THEN** the handler MUST delegate to `domain.Service.CreateChannel()` which orchestrates NGAC node creation, DB insert, and drive creation

#### Scenario: Handler line count
- **WHEN** the handler file is measured
- **THEN** each handler method MUST be under 20 lines (parse → validate → delegate → return)

### Requirement: DM lookup uses database query
The system SHALL provide `store.FindDMByMembers(userNodeID, targetNodeID)` that finds existing DM channels using a DB-level join or intersection query. The system MUST NOT scan all DM channels and make per-channel gRPC calls.

#### Scenario: Finding existing DM between two users
- **WHEN** a DM is requested between user A and user B who already have a DM
- **THEN** the system SHALL find it with at most 1 SQL query (not N gRPC calls)

#### Scenario: DM membership tracking
- **WHEN** a user is added to a channel
- **THEN** the system SHALL insert a row into `channel_members` table for DM discovery optimization

### Requirement: Store models separate from protobuf
The store layer SHALL define its own Go structs (`store.Channel`, `store.Message`) for database row scanning. These MUST NOT be protobuf-generated types.

#### Scenario: Channel model conversion
- **WHEN** a channel is loaded from the database
- **THEN** it SHALL be scanned into `store.Channel` struct and converted to protobuf via explicit `channelToProto()` function
