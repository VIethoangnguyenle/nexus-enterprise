## ADDED Requirements

### Requirement: Knowledge directory structure
The system SHALL maintain a knowledge directory at `.agent/knowledge/` with subdirectories: `patterns/`, `anti-patterns/`, `bugs/`, and an `index.yaml` file.

#### Scenario: Knowledge structure exists
- **WHEN** the knowledge layer is initialized
- **THEN** `.agent/knowledge/patterns/`, `.agent/knowledge/anti-patterns/`, `.agent/knowledge/bugs/` directories SHALL exist
- **AND** `.agent/knowledge/index.yaml` SHALL exist with schema header and bootstrap items

### Requirement: Knowledge item schema
Each knowledge item in `index.yaml` SHALL have: id, type (pattern/anti-pattern/bug), name, tags, rule, example, applicability (must_have/must_not_have), lifecycle (new/validated/deprecated), usage_mode (strict/advisory), tier (standard/protected), frequency, impact, confidence, last_seen, source_reviews.

#### Scenario: Valid knowledge item
- **WHEN** a knowledge item is created
- **THEN** it SHALL contain all required fields
- **AND** `lifecycle` SHALL default to `new`
- **AND** `confidence` SHALL default to 0.5

### Requirement: Hypothesis lifecycle
Knowledge items SHALL follow lifecycle: NEW → VALIDATED → DEPRECATED. An item moves to VALIDATED when observed ≥3 times OR manually marked. An item moves to DEPRECATED when contradicting NGAC or context no longer exists.

#### Scenario: Lifecycle progression
- **WHEN** a NEW knowledge item reaches frequency ≥3
- **THEN** its lifecycle SHALL change to `validated`

#### Scenario: NGAC contradiction
- **WHEN** a knowledge item contradicts NGAC domain invariants
- **THEN** it SHALL be immediately DEPRECATED

### Requirement: Usage mode enforcement
Items with `usage_mode: strict` SHALL be mandatory (deviation = blocking issue). Items with `usage_mode: advisory` SHALL be suggestive (may deviate with documented reason).

#### Scenario: Strict knowledge violation
- **WHEN** DEV deviates from strict knowledge without documented reason
- **THEN** QA SHALL create a blocking issue in reviews.yaml

### Requirement: Protected tier
Items with `tier: protected` SHALL NEVER be auto-pruned. Only manual review can remove protected items.

#### Scenario: Protected item survives pruning
- **WHEN** pruning runs and a protected item has low confidence
- **THEN** the protected item SHALL NOT be removed

### Requirement: Applicability structure
Each knowledge item SHALL have `applicability.must_have` (ALL conditions must be true) and `applicability.must_not_have` (ANY true = do NOT apply).

#### Scenario: Applicability check
- **WHEN** an agent loads knowledge items
- **THEN** it SHALL check must_have and must_not_have before applying
- **AND** items that don't match SHALL NOT be applied

### Requirement: Bootstrap seeds
The system SHALL include 3 seed knowledge items at initialization: K-BOOT-001 (permission-before-mutation, strict), K-BOOT-002 (no-unused-imports, advisory), K-BOOT-003 (explicit-error-handling, strict). All seeds SHALL be `lifecycle: validated, tier: protected`.

#### Scenario: Fresh initialization
- **WHEN** knowledge layer is first created
- **THEN** index.yaml SHALL contain 3 bootstrap items with validated lifecycle and protected tier

### Requirement: Thinking priority
Knowledge SHALL sit below Specs+Architecture and NGAC in thinking priority. Validated strict at level 4, validated advisory at level 5, new at level 6.

#### Scenario: Knowledge contradicts specs
- **WHEN** knowledge contradicts BA specs or SA architecture
- **THEN** specs SHALL win
- **AND** knowledge SHALL be flagged for review
