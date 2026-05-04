---
description: "Run autonomous 9-phase multi-agent pipeline: CEO → BA → User → SA → UX → DEV → SA verify → QA → POLISH → DONE → LEARN. Single command, fully automatic."
---

Chạy toàn bộ pipeline tự động. User trigger 1 lần, AI làm hết.

**Input**:
- `/autopilot "feature description"` — Chạy mới từ đầu
- `/autopilot <change-name>` — Resume nếu bị gián đoạn

**Steps**

1. **Read the orchestrator skill**
   Read `.agent/skills/autopilot/SKILL.md` — operating manual cho toàn bộ pipeline.

2. **Run ALL phases sequentially** in this conversation:
   - Phase 0: Setup
   - Phase 1: CEO → proposal.md
   - Phase 2: BA → specs/ + lock
   - Phase 3: User Checkpoint → intent validation (PAUSE)
   - Phase 4: SA → architecture.md
   - Phase 5: UX → design.md (Stitch)
   - Phase 6: DEV → implement + quality gate
   - Phase 7: SA verify → architecture compliance
   - Phase 8: QA → test + knowledge check
   - Phase 9: POLISH → fix non-blocking
   - Phase 10: DONE → summary
   - Phase 11: LEARN → conditional knowledge extraction

3. **Only pause for**: user checkpoint (Phase 3), context overflow, scope escalation, or repeated failures.

**Guardrails**
- This is a FULL implementation workflow — code WILL be written
- All 9 phases run in sequence, no skipping
- Spec lock enforced after BA
- User checkpoint REQUIRED after BA (Phase 3)
- DEV quality gate REQUIRED (8-step pre-flight + 7-point quality check)
- QA knowledge violation check REQUIRED
- POLISH max 2 rounds
- LEARN is conditional (triggers on bug repeats, patterns, P0/P1)
- User sees progress (`[N/9] ✅`), not internals
