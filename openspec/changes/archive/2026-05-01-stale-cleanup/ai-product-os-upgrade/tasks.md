# Tasks: AI Product OS Upgrade

## Step 1: Foundation

- [x] Create `openspec/qa-memory/global-regression.json` with empty init schema
- [x] Rewrite `autopilot/SKILL.md` — add Input section with 3 invocation modes (new, resume, override)
- [x] Rewrite `autopilot/SKILL.md` — add Phase Detection section with auto-detect logic
- [x] Rewrite `autopilot/SKILL.md` — add Pipeline State Schema (full `.openspec.yaml` pipeline format)
- [x] Rewrite `autopilot/SKILL.md` — update all Phase definitions to read/write `.openspec.yaml`
- [x] Rewrite `autopilot/SKILL.md` — add Multi-Conversation Messaging templates at end of each phase
- [x] Update `autopilot.md` workflow — add multi-conversation explanation

## Step 2: Lock Enforcement

- [x] Add Spec Lock Protocol section to `autopilot/SKILL.md`
- [x] Add Step 6 (Lock Spec) to `agent-ba/SKILL.md` — hash computation + write to yaml

## Step 3: QA Memory Integration

- [x] Add QA Memory Protocol section to `agent-qc/SKILL.md` — read on start
- [x] Add QA Memory Protocol section to `agent-qc/SKILL.md` — write on complete
- [x] Add qa-memory.json schema reference to QC output contract
- [x] Add global regression read/write to QC regression phase

## Step 4: Checkpoint System

- [x] Add Checkpoint Protocol to `agent-dev/SKILL.md` — write after each task
- [x] Add Resume from Checkpoint logic to `autopilot/SKILL.md` dev phase

## Step 5: Review Routing

- [x] Add Review Routing section to `autopilot/SKILL.md` — reviews.yaml schema + routing logic
- [x] Add phase naming: `qc-fix-dev`, `qc-fix-ux` with iteration counter
- [x] Add loop history tracking format to pipeline state

## Step 6: Polish

- [x] Add enterprise simulation guidance to `agent-qc/SKILL.md` (role-based test scenarios)
- [x] Add OpsX Rules enforcement section to `autopilot/SKILL.md`
- [x] Verify all 6 files are internally consistent (cross-reference check)
