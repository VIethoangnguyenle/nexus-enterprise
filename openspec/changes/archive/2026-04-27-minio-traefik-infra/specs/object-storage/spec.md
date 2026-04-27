## ADDED Requirements

### Requirement: MinIO service in Docker Compose
The system SHALL include a MinIO container in `docker-compose.yml` configured with health checks, persistent volume, and environment-based credentials.

#### Scenario: MinIO starts with compose up
- **WHEN** `docker-compose up` is executed
- **THEN** MinIO container starts on port 9000 (S3 API) and 9001 (console)
- **THEN** MinIO is healthy before dependent services start

#### Scenario: MinIO data persists across restarts
- **WHEN** MinIO container is stopped and restarted
- **THEN** all previously stored objects are still accessible

### Requirement: Workspace bucket auto-creation
The system SHALL create a MinIO bucket named `ws-{workspace_id}` when a new workspace is provisioned.

#### Scenario: New workspace creates bucket
- **WHEN** a user creates a new workspace via `POST /api/workspaces`
- **THEN** a MinIO bucket `ws-{workspace_id}` is created
- **THEN** the workspace record stores the bucket name

#### Scenario: Bucket creation failure is non-fatal
- **WHEN** MinIO is temporarily unavailable during workspace creation
- **THEN** workspace creation still succeeds (NGAC graph + DB created)
- **THEN** bucket creation is retried on next document upload

### Requirement: Document storage in MinIO
The system SHALL store document files in MinIO at path `ws-{workspace_id}/documents/{doc_id}/{filename}` instead of local disk.

#### Scenario: Document stored in correct bucket path
- **WHEN** a document upload is confirmed
- **THEN** the object exists in MinIO at `ws-{workspace_id}/documents/{doc_id}/{filename}`

#### Scenario: Local disk volume no longer used for documents
- **WHEN** the system is deployed
- **THEN** no document files are written to the `docdata` Docker volume

### Requirement: MinIO credentials via environment variables
The system SHALL configure MinIO access via `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, and `MINIO_USE_SSL` environment variables in consuming services.

#### Scenario: Services connect to MinIO
- **WHEN** Document Service starts
- **THEN** it connects to MinIO using environment variables
- **THEN** connection failure is logged but does not crash the service
