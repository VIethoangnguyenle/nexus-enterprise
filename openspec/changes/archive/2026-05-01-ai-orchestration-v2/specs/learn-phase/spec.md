## ADDED Requirements

### Requirement: Conditional LEARN triggers
LEARN phase SHALL only execute when at least one condition is met: (1) bug repeats from qa-memory, (2) pattern appears ≥2 times cross-features, (3) P0/P1 failure, (4) bootstrap rule — in first 10 runs, P0/P1 single occurrence triggers LEARN.

#### Scenario: No trigger conditions met
- **WHEN** none of the trigger conditions are satisfied
- **THEN** LEARN phase SHALL be skipped

#### Scenario: Bootstrap rule applies
- **WHEN** system is within first 10 pipeline runs
- **AND** a P0/P1 failure occurs (single occurrence)
- **THEN** LEARN phase SHALL trigger

### Requirement: LEARN process steps
LEARN SHALL execute 8 steps in order: (1) extract from QA report + reviews, (2) validate frequency/impact, (3) distill into pattern/anti-pattern/bug, (4) score, (5) deduplicate semantically, (6) write to knowledge/, (7) prune standard-tier items below threshold, (8) compact resolved reviews.

#### Scenario: Deduplication
- **WHEN** a new knowledge item has the same semantic meaning as an existing item (same rule, different wording)
- **THEN** LEARN SHALL merge them into one item with combined frequency

#### Scenario: Pruning respects tiers
- **WHEN** pruning runs
- **THEN** only `tier: standard` items with confidence < 0.3 SHALL be deprecated
- **AND** `tier: protected` items SHALL NOT be affected

### Requirement: Scoring rules
Confidence SHALL increase by 0.1 (cap 0.95) when observed again. Confidence SHALL decrease by 0.15 when 20 runs pass without match (standard tier only). Items below 0.3 confidence SHALL be DEPRECATED.

#### Scenario: Success paradox prevention
- **WHEN** a feature matches knowledge tags but the related bug does NOT appear
- **THEN** the knowledge item's `last_seen` SHALL be refreshed

### Requirement: Reviews compaction
LEARN SHALL move `status: resolved` reviews to a `resolved_reviews` section at end of reviews.yaml file.

#### Scenario: Compaction preserves history
- **WHEN** LEARN compacts reviews
- **THEN** resolved reviews SHALL be moved, not deleted
