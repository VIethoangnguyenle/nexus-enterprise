## ADDED Requirements

### Requirement: Unified DEV pre-flight checklist
DEV agent SHALL complete an 8-step pre-flight checklist before implementing any code. Steps: (1) read upstream artifacts, (2) verify artifact version = latest, (3) check blocking reviews, (4) verify specs+architecture consistency, (5) load relevant knowledge, (6) list strict items + check applicability, (7) confirm applicable items, (8) define scope boundaries.

#### Scenario: Pre-flight blocks on inconsistency
- **WHEN** DEV finds specs and architecture are inconsistent (step 4 fails)
- **THEN** DEV SHALL NOT implement
- **AND** DEV SHALL create feedback in reviews.yaml for BA or SA
- **AND** pipeline SHALL stop until resolved

#### Scenario: Pre-flight knowledge loading
- **WHEN** DEV loads knowledge items (step 5-7)
- **THEN** DEV SHALL list all `usage_mode: strict` items
- **AND** DEV SHALL check applicability (must_have/must_not_have) for each
- **AND** DEV SHALL confirm which items will be applied

### Requirement: Code quality gate
DEV SHALL self-check 7 quality points before outputting code: (1) clean code, (2) reusability, (3) dependencies, (4) architecture alignment, (5) error handling, (6) performance, (7) security/NGAC.

#### Scenario: Quality gate blocks on failure
- **WHEN** any quality check item FAILS
- **THEN** DEV SHALL NOT output code
- **AND** DEV SHALL fix the issue first

### Requirement: Mandatory output format
DEV output SHALL include: code quality checklist results (PASS all 7 or FAIL with reason), applied knowledge list with mode, and deviation explanations if any.

#### Scenario: Complete DEV output
- **WHEN** DEV completes implementation
- **THEN** output SHALL contain quality checklist, applied knowledge, and deviation report

### Requirement: Mandatory skill usage
DEV SHALL use golang-* skills (backend), frontend-best-practices, and component-reuse-checklist skills.

#### Scenario: Backend implementation
- **WHEN** DEV implements Go backend code
- **THEN** relevant golang-* skills SHALL be consulted
