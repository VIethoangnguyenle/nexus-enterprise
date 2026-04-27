## ADDED Requirements

### Requirement: Draft to Approved workflow
The system SHALL support a document lifecycle where documents start in DraftDocs OA and can be moved to ApprovedDocs OA through an approval action. Approval SHALL be modeled as an NGAC graph mutation: remove assignment from DraftDocs, add assignment to ApprovedDocs.

#### Scenario: Document starts as draft
- **WHEN** a document is uploaded
- **THEN** it SHALL be assigned to OA "DraftDocs" and NOT to OA "ApprovedDocs"

#### Scenario: Reviewer approves document
- **WHEN** a user with "approve" operation access to DraftDocs approves a document
- **THEN** the document's assignment moves from DraftDocs to ApprovedDocs

#### Scenario: Non-reviewer cannot approve
- **WHEN** a user without "approve" operation access attempts to approve a document
- **THEN** the system SHALL return 403 Forbidden

### Requirement: Only approved documents can be shared
The system SHALL enforce that only documents assigned to ApprovedDocs OA can be shared or published. The share and publish APIs SHALL check this precondition via the NGAC graph.

#### Scenario: Sharing a draft document is rejected
- **WHEN** a user attempts to share a document still assigned to DraftDocs
- **THEN** the system SHALL return an error: "Document must be approved before sharing"

#### Scenario: Sharing an approved document succeeds
- **WHEN** a user shares a document assigned to ApprovedDocs
- **THEN** the sharing operation proceeds normally
