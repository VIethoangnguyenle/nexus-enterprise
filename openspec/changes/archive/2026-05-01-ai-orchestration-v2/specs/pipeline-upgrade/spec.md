## ADDED Requirements

### Requirement: 9-phase pipeline
Autopilot SHALL orchestrate a 9-phase sequential pipeline: CEO → BA → [User Checkpoint] → SA → UX → DEV → [SA verify] → QA → POLISH → DONE → LEARN.

#### Scenario: Full pipeline execution
- **WHEN** autopilot runs a feature
- **THEN** it SHALL execute all 9 phases in order
- **AND** each phase SHALL update execution state in .openspec.yaml

### Requirement: User checkpoint after BA
After BA phase completes specs, system SHALL pause for user intent validation. Pipeline SHALL NOT proceed without user confirmation.

#### Scenario: User rejects intent
- **WHEN** user rejects at intent checkpoint
- **THEN** pipeline SHALL stop and return to CEO phase

### Requirement: SA verify after DEV
After DEV phase, SA SHALL verify code against architecture using 4-point checklist before QA proceeds.

#### Scenario: SA verify passes
- **WHEN** SA verify completes with no blocking issues
- **THEN** pipeline SHALL advance to QA phase

### Requirement: POLISH phase
POLISH SHALL fix non-blocking issues from QA. Execution order: UX → BA → DEV implements all → QA re-verifies once. Max 2 polish rounds.

#### Scenario: Max polish rounds
- **WHEN** 2 polish rounds are completed
- **THEN** pipeline SHALL move to DONE with notes about remaining items

### Requirement: Feedback routing
Feedback SHALL route directly to the owner role, not sequentially backwards. Stop conditions: 2 rounds per pair, 5 total pipeline loops.

#### Scenario: Max retries exceeded
- **WHEN** retry_count reaches max_retries (5)
- **THEN** pipeline SHALL stop and escalate to user

### Requirement: Cascade invalidation
When an upstream artifact is updated due to feedback, all downstream artifacts SHALL be re-verified.

#### Scenario: Specs updated
- **WHEN** specs/ artifact is updated
- **THEN** architecture.md and design.md SHALL be re-verified

### Requirement: Execution state tracking
Pipeline SHALL track state in .openspec.yaml: current_phase, status, retry_count, phase_history, artifact_versions.

#### Scenario: Artifact version tracking
- **WHEN** an artifact is updated due to feedback
- **THEN** its version SHALL increment in artifact_versions
- **AND** downstream agents SHALL detect stale artifacts
