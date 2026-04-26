## MODIFIED Requirements

### Requirement: Workspace-scoped documents
Documents SHALL be scoped to workspaces. Each document SHALL belong to a specific workspace and be assigned to OAs under that workspace's Policy Class.

#### Scenario: Upload document to workspace
- **WHEN** a user calls `POST /api/workspaces/{id}/documents` with a file
- **THEN** the document's NGAC Object node SHALL be assigned to the workspace's Documents OA and DraftDocs OA

#### Scenario: Document workspace isolation
- **WHEN** a user with access only to Workspace A attempts to read a document in Workspace B
- **THEN** `CheckAccess` SHALL return DENY because the user has no UA reaching Workspace B's PC

### Requirement: Document Service as microservice
Document operations SHALL be handled by a dedicated Document Service that calls the Policy Service for all access checks and graph mutations.

#### Scenario: Upload flow through services
- **WHEN** a user uploads a document
- **THEN** the Gateway forwards to Document Service, which calls Policy Service for access check, stores the file, creates DB record, and calls Policy Service to create NGAC nodes

#### Scenario: Share flow through services
- **WHEN** a user shares a document with a UA
- **THEN** the Document Service SHALL call the Policy Service to create the share OA, assignments, and association — same NGAC pattern as the current monolith

### Requirement: Preserved document features
All existing document features SHALL be preserved: upload, download, approve (draft→approved), share (cross-workspace via scoped Share OAs), publish (public access), unpublish, and delete.

#### Scenario: Approve document
- **WHEN** a user with approve permission calls `POST /api/documents/{id}/approve`
- **THEN** the document's NGAC node SHALL be reassigned from DraftDocs to ApprovedDocs (same workflow as current system)

#### Scenario: Share document cross-workspace
- **WHEN** a user shares a document with a UA from another workspace
- **THEN** a Share OA SHALL be created under SharedDocs (PC_Global), enabling cross-workspace access via the existing mechanism
