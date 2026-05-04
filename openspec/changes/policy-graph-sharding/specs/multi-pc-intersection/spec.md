## ADDED Requirements

### Requirement: ALL-PC intersection replaces single-PC intersection
The system SHALL enforce that user MUST reach ALL Policy Classes that the target object reaches. The existing `findCommonPC` (single-PC match) SHALL be replaced with `allPCsSatisfied` (ALL-PC match).

#### Scenario: Object belongs to one PC (backward compatible)
- **WHEN** object reaches {PC_W1} and user reaches {PC_W1}
- **THEN** the system SHALL return ALLOW (same as current behavior)

#### Scenario: Object belongs to two PCs, user has both
- **WHEN** object reaches {PC_W1, PC_W1_Confidential} and user reaches {PC_W1, PC_W1_Confidential}
- **THEN** the system SHALL return ALLOW

#### Scenario: Object belongs to two PCs, user has only one
- **WHEN** object reaches {PC_W1, PC_W1_Confidential} and user reaches {PC_W1}
- **THEN** the system SHALL return DENY with reason indicating missing PC

### Requirement: AccessDecision explanation lists all matched PCs
The `AccessExplanation` SHALL include all matched Policy Classes (not just the first one found) when decision is ALLOW.

#### Scenario: Multi-PC ALLOW explanation
- **WHEN** access is ALLOWED with objectPCs={PC_W1, PC_Confidential} and user has both
- **THEN** the explanation SHALL list both PC_W1 and PC_Confidential as matched PCs
