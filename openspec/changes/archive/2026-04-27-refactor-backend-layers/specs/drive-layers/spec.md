## ADDED Requirements

### Requirement: Domain layer for Drive business logic
The Drive service SHALL have a `domain` package (`internal/domain/`) that contains business logic extracted from the gRPC handler. The handler MUST NOT contain direct `s.db` calls.

#### Scenario: Confirming file upload
- **WHEN** `ConfirmFile` is called
- **THEN** size update MUST go through `store.UpdateFileSize()` not direct `s.db.Exec`

#### Scenario: Ensuring root folder exists
- **WHEN** `ensureRoot` logic executes
- **THEN** workspace DB lookup MUST use `store.GetWorkspacePCID()` not direct `s.db.QueryRow`

### Requirement: Handler delegates to domain
The Drive gRPC handler methods MUST delegate business orchestration to domain service methods. Handler is responsible only for request parsing, input validation, and response formatting.

#### Scenario: Creating a folder
- **WHEN** `CreateFolder` gRPC is called
- **THEN** handler calls `domain.DriveService.CreateFolder()` which orchestrates NGAC node creation, assignment, and store insert

#### Scenario: No direct DB access in handler
- **WHEN** the handler file is inspected
- **THEN** there SHALL be zero references to `s.db.QueryRow` or `s.db.Exec` in `server.go`
