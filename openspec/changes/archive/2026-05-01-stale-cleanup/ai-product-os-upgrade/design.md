# Design: AI Product OS Upgrade

## Design Decisions

1. **`.openspec.yaml` as state machine** — Pipeline state lives in the same file that already tracks change metadata. No new file format needed for orchestration.
2. **`reviews.yaml` for rejections** — Separate artifact prevents spec mutation. Each rejection is append-only with timestamp and suggested_changes.
3. **File-level + global hash** — Global hash for fast guard check, file hashes for debugging which spec was tampered.
4. **5-run sliding window** — Balances LLM context budget vs regression coverage. Issues array is never pruned.
5. **Hybrid phase control** — Auto-detect removes friction for normal flow. Manual override enables debugging and re-running specific phases.

## File Inventory

| # | File | Action | Purpose |
|---|---|---|---|
| 1 | `.agent/skills/autopilot/SKILL.md` | MODIFY | Multi-conv detection, lock enforcement, checkpoint, review routing, phase naming |
| 2 | `.agent/workflows/autopilot.md` | MODIFY | Multi-conv user instructions |
| 3 | `.agent/skills/agent-qc/SKILL.md` | MODIFY | QA memory read/write protocol |
| 4 | `.agent/skills/agent-dev/SKILL.md` | MODIFY | Checkpoint write after each task |
| 5 | `.agent/skills/agent-ba/SKILL.md` | MODIFY | Spec hash computation after lock |
| 6 | `openspec/qa-memory/global-regression.json` | NEW | Empty init for global regression tracking |

## Detailed Changes

### 1. `autopilot/SKILL.md` — Major rewrite

**Current**: 220 lines, single-conversation pipeline with 7 phases.

**New structure**:

```
# Autopilot — Multi-Agent Orchestrator (v2)

## Identity (unchanged)

## Input
- /autopilot "feature description"   → new change, run CEO
- /autopilot <change-name>           → detect phase, resume
- /autopilot <change-name> --phase X → manual override

## Phase Detection (NEW)
- Read .openspec.yaml → pipeline.current_phase
- If no change exists → Phase 0 + 1
- If change exists → run current_phase
- If --phase provided → validate and override

## Pipeline State Schema (NEW)
- Full .openspec.yaml pipeline schema
- Phase statuses: pending | in-progress | done | skipped
- Loop tracking: total, max, history array

## Spec Lock Protocol (NEW)
- After BA: compute hashes, set status=locked
- Before UX/Dev/QC: verify hash, halt if tampered
- Unlock: max 1, requires reason, routes to BA

## Review Routing (NEW)
- Agent writes reviews.yaml
- Orchestrator reads → routes to target agent
- Max 2 reject cycles per handoff

## Phase Definitions
- Phase 0-7 (same as current, with additions):
  - Each phase reads .openspec.yaml first
  - Each phase writes completion status + conversation_id
  - Dev phase writes checkpoint after each task
  - QC phase reads/writes qa-memory.json
  - Loop phases use qc-fix-dev / qc-fix-ux naming

## Checkpoint Protocol (NEW)
- Dev writes: tasks_completed, tasks_total, last_file, completed_tasks[]
- Resume: skip completed tasks, continue from next

## Multi-Conversation Messaging (NEW)
- End of each phase: clear instruction for user
- "Phase X done. Open new conversation, run: /autopilot <name>"

## Orchestration Rules (enhanced)
- Same guardrails + new lock enforcement + review routing
```

### 2. `autopilot.md` workflow — Minor update

Add multi-conversation explanation:
```
- Each phase runs in its own conversation for context efficiency
- Just re-run /autopilot <change-name> in a new conversation
- Orchestrator auto-detects which phase to run next
- Use --phase <name> to override
```

### 3. `agent-qc/SKILL.md` — Add QA Memory protocol

Add new section after Phase 6 (Regression):

```
## QA Memory Protocol (NEW)

### On Start:
1. Read qa-memory.json → previous issues, scenarios
2. Read global-regression.json → cross-change patterns
3. If change affects module in known_issues → test those first

### On Complete:
1. Append new run to qa-memory.json (cap at 5, remove oldest)
2. Update issue statuses (open → fixed, or increment fix_count)
3. If new cross-change pattern → add to global-regression.json
4. Issues array is NEVER pruned (only status updates)
```

### 4. `agent-dev/SKILL.md` — Add checkpoint write

Add to Step 3 (Implement):

```
### Checkpoint Protocol
After completing each task:
1. Update .openspec.yaml phases.dev.checkpoint:
   - tasks_completed: N
   - tasks_total: total
   - last_file: "path/to/last/file"
   - completed_tasks: ["task1", "task2", ...]
2. If resuming from checkpoint:
   - Read checkpoint → skip completed tasks
   - Continue from tasks_completed + 1
```

### 5. `agent-ba/SKILL.md` — Add spec hash

Add to end of process (after Step 5):

```
### Step 6: Lock Spec (NEW)
After producing all spec files:
1. Compute SHA256 for each file in specs/
2. Compute global hash (hash of all file hashes sorted)
3. Write to .openspec.yaml:
   - pipeline.status = "locked"
   - pipeline.spec_lock.locked_at = timestamp
   - pipeline.spec_lock.locked_by = "BA"
   - pipeline.spec_lock.spec_hash = global_hash
   - pipeline.spec_lock.file_hashes = {path: hash, ...}
```

### 6. `global-regression.json` — New file

```json
{
  "version": 1,
  "last_updated": null,
  "known_issues": [],
  "regression_patterns": []
}
```

## OpsX Rules Integration

The following rules are embedded into the orchestrator:

- **Role isolation**: Each agent skill file defines exactly ONE role. Orchestrator enforces sequence.
- **No role overlap**: UX cannot modify specs (reviews.yaml only). Dev cannot design UI. QA cannot change logic.
- **Spec lock**: After BA → LOCKED. No modifications allowed.
- **Superpowers = thinking**: Agent skills guide thinking. MCP tools execute.
- **QA completeness**: QC must test UI/flow/data/permission, click all buttons, update qa-memory.
- **Wrong role = reject**: If any agent attempts work outside its role, orchestrator rejects and re-routes.
