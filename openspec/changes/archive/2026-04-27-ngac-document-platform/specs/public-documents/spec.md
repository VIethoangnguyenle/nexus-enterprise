## ADDED Requirements

### Requirement: Publish document as public via NGAC
The system SHALL model public document visibility by assigning the document to the PublicDocs OA. The PublicUsers UA (which all registered users belong to) SHALL have a [read] association to PublicDocs. No boolean "public" flag SHALL be used.

#### Scenario: Publishing a document
- **WHEN** a user publishes approved document "report.pdf"
- **THEN** the document is assigned to OA "PublicDocs" under PC_Global

#### Scenario: All users can read public documents
- **WHEN** any authenticated user requests "report.pdf" after it has been published
- **THEN** the access check finds a path: user → PublicUsers → association[read] → PublicDocs ← report.pdf, and returns ALLOW

### Requirement: Unpublish document
The system SHALL allow removing public visibility by removing the document's assignment to PublicDocs OA.

#### Scenario: Unpublish removes public access
- **WHEN** a document is unpublished
- **THEN** the document's assignment to PublicDocs is removed, and users without other access paths can no longer read it

### Requirement: All users assigned to PublicUsers
The system SHALL assign every registered user to the PublicUsers UA at registration time. This ensures public document access flows through the NGAC graph like any other access.

#### Scenario: New user gets PublicUsers membership
- **WHEN** a new user registers
- **THEN** the user's NGAC node is assigned to UA "PublicUsers"
