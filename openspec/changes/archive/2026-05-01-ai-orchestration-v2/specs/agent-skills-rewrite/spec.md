## ADDED Requirements

### Requirement: Unified thinking priority in all skills
All 7 agent skills SHALL use the same 7-level thinking priority: (1) Specs+Architecture, (2) NGAC invariants, (3) Role thinking, (4) Knowledge validated/strict, (5) Knowledge validated/advisory, (6) Knowledge new, (7) gstack thinking.

#### Scenario: Consistent priority across agents
- **WHEN** any agent (CEO/BA/SA/UX/DEV/QC/Autopilot) starts execution
- **THEN** it SHALL follow the unified 7-level thinking priority
- **AND** no simplified or alternative priority hierarchy SHALL be used

### Requirement: Pre-flight checklist in every SKILL.md
Every agent SKILL.md SHALL have a mandatory pre-flight checklist at the TOP of the file. The checklist SHALL include: (1) read upstream artifacts + version, (2) verify artifact version = latest, (3) check blocking reviews, (4) define scope boundary.

#### Scenario: Agent starts without pre-flight
- **WHEN** an agent attempts to start work without completing pre-flight
- **THEN** the SKILL.md instructions SHALL prevent proceeding

### Requirement: Role boundary enforcement
Each agent SHALL have explicit KHÔNG (NOT) boundaries defining what it SHALL NOT do. CEO SHALL NOT write specs. BA SHALL NOT design UI. SA SHALL NOT write code. UX SHALL NOT write logic. DEV SHALL NOT redesign UX. QA SHALL NOT self-fix.

#### Scenario: DEV attempts spec change
- **WHEN** DEV finds spec issues during implementation
- **THEN** DEV SHALL create feedback in reviews.yaml
- **AND** DEV SHALL NOT modify specs directly

### Requirement: SA verification step
After DEV phase, SA SHALL verify code against architecture using 4-point checklist: (1) data flow, (2) NGAC permission model, (3) service integration, (4) consistency with architecture.md.

#### Scenario: SA finds architecture violation
- **WHEN** SA verify finds code violates architecture.md
- **THEN** SA SHALL create blocking feedback in reviews.yaml for DEV

### Requirement: QA knowledge violation checking
QA SHALL load all `usage_mode: strict` knowledge items relevant to the feature and verify code does not violate them. Violations SHALL be blocking issues.

#### Scenario: QA finds strict knowledge violation
- **WHEN** QA identifies code that violates a strict knowledge item
- **THEN** QA SHALL create a blocking issue in reviews.yaml routed to DEV

### Requirement: QA regression from knowledge bugs
QA SHALL check regression from `knowledge/bugs/` — known bug patterns SHALL NOT reappear.

#### Scenario: Known bug pattern detected
- **WHEN** QA finds a bug matching a pattern in knowledge/bugs/
- **THEN** QA SHALL report it as a regression with reference to the knowledge item

### Requirement: Skill deprecation headers
Deprecated skills SHALL have a DEPRECATED header at the top indicating replacement.

#### Scenario: Deprecated skill read
- **WHEN** an agent reads a deprecated skill
- **THEN** the DEPRECATED header SHALL redirect to the replacement skill
