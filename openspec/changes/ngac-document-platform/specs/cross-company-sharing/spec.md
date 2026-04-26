## ADDED Requirements

### Requirement: Share document via NGAC graph
The system SHALL model cross-company sharing by creating a scoped OA under SharedDocs, assigning the document to that OA, and creating an association from the target UA to the scoped OA with specified operations. No ad-hoc share tables SHALL be used.

#### Scenario: Share document with another company's department
- **WHEN** user "alice" (Acme) shares approved document "invoice.pdf" with Beta_Engineering for [read] access
- **THEN** a scoped OA "Share_{docId}_to_Beta_Engineering" is created under SharedDocs, "invoice.pdf" is assigned to it, and association Beta_Engineering --[read]--> Share_{docId}_to_Beta_Engineering is created

#### Scenario: Target company can now access shared document
- **WHEN** "charlie" (Beta_Engineering) requests "invoice.pdf" after it has been shared
- **THEN** the NGAC access check finds a path: charlie → Beta_Engineering → association[read] → Share OA ← invoice.pdf, and returns ALLOW

### Requirement: Cross-company access denied by default
The system SHALL deny access to documents across company boundaries unless an explicit sharing association exists in the NGAC graph.

#### Scenario: No sharing means no access
- **WHEN** "charlie" (Beta_Engineering) requests "invoice.pdf" (Acme only, not shared)
- **THEN** the access check SHALL return DENY because no association path exists

### Requirement: Revoke sharing
The system SHALL allow revoking a share by removing the scoped OA assignment and association from the NGAC graph.

#### Scenario: Revoke removes access
- **WHEN** "alice" revokes the share of "invoice.pdf" with Beta_Engineering
- **THEN** the scoped share OA, its assignments, and associations are removed, and "charlie" can no longer access the document
