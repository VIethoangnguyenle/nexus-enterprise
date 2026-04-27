## ADDED Requirements

### Requirement: Get presigned upload URL
The system SHALL provide an API endpoint that returns a presigned PUT URL for direct-to-MinIO upload.

#### Scenario: Client requests upload URL
- **WHEN** client sends `POST /api/workspaces/{id}/documents/upload-url` with `{"filename": "report.pdf", "mime_type": "application/pdf", "title": "Q4 Report"}`
- **THEN** response contains `{"upload_url": "https://...", "doc_id": "...", "object_key": "..."}`
- **THEN** the presigned URL is valid for 5 minutes

#### Scenario: Upload URL requires authentication
- **WHEN** client sends the request without a valid JWT
- **THEN** response is 401 Unauthorized

#### Scenario: NGAC access check before URL generation
- **WHEN** client requests upload URL for a workspace
- **THEN** the system checks NGAC write access for the user on the workspace's document OA
- **THEN** returns 403 if access denied

### Requirement: Direct upload to MinIO
The system SHALL allow clients to upload file content directly to MinIO using the presigned PUT URL, bypassing the Gateway and gRPC stack.

#### Scenario: Client uploads file via presigned URL
- **WHEN** client sends `PUT {upload_url}` with binary file body and correct `Content-Type`
- **THEN** MinIO stores the object at the path encoded in the URL
- **THEN** MinIO returns 200 OK

#### Scenario: Expired presigned URL rejected
- **WHEN** client tries to upload after the 5-minute expiry
- **THEN** MinIO returns 403 Forbidden
- **THEN** client must request a new upload URL

#### Scenario: File size up to 5GB supported
- **WHEN** client uploads a file up to 5GB via presigned URL
- **THEN** upload succeeds (no 4MB gRPC limit)

### Requirement: Confirm upload
The system SHALL provide an API endpoint to confirm that an upload completed successfully, creating the document record in the database.

#### Scenario: Client confirms successful upload
- **WHEN** client sends `POST /api/documents/{doc_id}/confirm`
- **THEN** Document Service verifies the object exists in MinIO (HeadObject)
- **THEN** document record is created in the database with status "draft"
- **THEN** NGAC node is assigned to workspace's DraftDocs OA
- **THEN** response contains the full Document object

#### Scenario: Confirm without upload fails
- **WHEN** client sends confirm but the object does not exist in MinIO
- **THEN** response is 400 Bad Request with "file not uploaded"

### Requirement: Get presigned download URL
The system SHALL provide an API endpoint that returns a presigned GET URL for downloading a document, after verifying NGAC read access.

#### Scenario: Client requests download URL
- **WHEN** client sends `GET /api/documents/{doc_id}/download-url`
- **THEN** system checks NGAC read access for the user on the document's NGAC node
- **THEN** response contains `{"download_url": "https://..."}` valid for 15 minutes

#### Scenario: Download URL denied for unauthorized user
- **WHEN** user without NGAC read access requests download URL
- **THEN** response is 403 Forbidden

### Requirement: Proto changes for presigned flow
The system SHALL add new gRPC RPCs to the DocumentService proto: `GetUploadURL`, `ConfirmUpload`, and `GetDownloadURL`.

#### Scenario: Proto compiles successfully
- **WHEN** `make proto` is executed
- **THEN** new RPCs generate valid Go code
- **THEN** existing RPCs remain backward compatible

#### Scenario: Legacy Upload RPC still functional
- **WHEN** existing `Upload` RPC is called with `bytes content`
- **THEN** it still works for backward compatibility but stores to MinIO instead of disk
