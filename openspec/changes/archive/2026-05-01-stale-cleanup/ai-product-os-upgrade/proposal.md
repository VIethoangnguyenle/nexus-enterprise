# AI Product OS Upgrade

## Evidence Summary

- **Autopilot Orchestrator**: ✅ Exists — `autopilot/SKILL.md` (220 lines), runs CEO → BA → UX → Dev → QC in single conversation
- **Agent Skills**: ✅ All 5 exist — `agent-ceo`, `agent-ba`, `agent-ux`, `agent-dev`, `agent-qc`
- **OpenSpec System**: ✅ Exists — `openspec/` with changes/, specs/, config.yaml, archive/
- **Rejection Loops**: ✅ Exist — max 2 cycles per handoff
- **QC Loops**: ✅ Exist — max 3 per issue, max 5 total
- **QA Memory**: ❌ Missing — no persistence between QC runs
- **Spec Lock**: ❌ Missing — no enforcement after BA completes
- **Multi-conversation**: ❌ Missing — all agents share single context window
- **Checkpoint/Resume**: ❌ Missing — crash = lost progress
- **Review Artifacts**: ❌ Missing — rejections are inline, not structured

## Product Assessment

- **Size**: M — No application code changes. Modifies 4 skill files, 1 workflow file, creates 1 JSON init file, adds 1 new schema concept (reviews.yaml)
- **Risk**: Low — All changes are to AI instruction files (.md), not to running application code. No proto, DB, or service changes.
- **Target user**: The developer (you) running `/autopilot` to build features autonomously
- **Core action**: Run multi-agent pipeline across multiple conversations with state persistence, spec safety, and QA learning

## Scope

### In scope

1. **Multi-conversation orchestration** — `.openspec.yaml` pipeline state machine with phase detection. User runs `/autopilot <name>` in fresh conversations, orchestrator auto-advances to next phase
2. **Spec lock enforcement** — After BA completes: compute file-level + global hash, block all spec modifications. Max 1 unlock with reason logged
3. **QA Memory** — Per-change `qa-memory.json` (5-run sliding window, issues never deleted) + global `qa-memory/global-regression.json` for cross-change patterns
4. **Rejection flow** — `reviews.yaml` artifact for structured agent rejections (no direct spec mutation)
5. **Checkpoint system** — Dev agent writes task-level progress to `.openspec.yaml` for resume after crash
6. **Phase naming** — `qc-fix-dev` / `qc-fix-ux` for fix iterations (distinct from fresh phases)
7. **Hybrid phase control** — Auto-detect next phase by default, `--phase <name>` manual override

### Out of scope

- **Application code changes** — This change modifies only AI workflow files
- **OpenSpec CLI tooling** — No new CLI commands (manual file operations)
- **Multi-user enterprise simulation data** — QC will simulate roles conceptually, no seed data creation
- **Cross-project memory** — QA memory is per-project only

### Deferred

- **Automated multi-conversation launcher** — Currently user manually starts new conversations
- **QA Memory analytics dashboard** — Aggregate stats across changes
- **Parallel agent execution** — CEO+BA could theoretically overlap for independent sub-features

## Success Criteria

1. Running `/autopilot <name>` in a new conversation correctly detects and resumes the next phase
2. Specs cannot be modified after BA lock (hash verification passes/fails correctly)
3. QC agent reads previous run data and tests known regression patterns
4. Agent rejections are captured in `reviews.yaml` and routed to correct agent
5. Dev agent can resume from checkpoint after conversation restart
