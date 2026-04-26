## ADDED Requirements

### Requirement: Document upload
The system SHALL allow authenticated users to upload documents. Each uploaded document SHALL create an Object node in the NGAC graph and assign it to the DraftDocs OA under the user's company Policy Class. The file SHALL be stored on the local filesystem.

#### Scenario: Successful upload
- **WHEN** user "alice" (Acme_Finance) uploads "invoice.pdf"
- **THEN** a document record is created, an NGAC Object node is created, the object is assigned to OA "DraftDocs" under PC_Acme, and the file is written to `/data/documents/{doc-id}/invoice.pdf`

### Requirement: Document retrieval with access check
The system SHALL check NGAC access before returning any document. The access check SHALL evaluate (user, document, read) through the NGAC engine.

#### Scenario: Authorized retrieval
- **WHEN** user "alice" requests a document she has read access to via the NGAC graph
- **THEN** the system SHALL return the document metadata and download URL

#### Scenario: Unauthorized retrieval denied
- **WHEN** user "charlie" (Beta_Engineering) requests "invoice.pdf" (Acme-only document with no sharing)
- **THEN** the system SHALL return 403 Forbidden with an explanation

### Requirement: Document listing
The system SHALL return only documents the requesting user has access to. The listing SHALL be filtered by evaluating NGAC access for each document.

#### Scenario: User sees only accessible documents
- **WHEN** user "alice" requests her document list
- **THEN** she sees documents in OAs accessible through her UA assignments, but NOT documents in other companies without sharing associations

### Requirement: Document deletion with access check
The system SHALL require write access to delete a document. Deletion SHALL remove the NGAC Object node and all its assignments.

#### Scenario: Owner deletes document
- **WHEN** user "alice" deletes her document with write access
- **THEN** the document, file, NGAC node, and all assignments are removed
