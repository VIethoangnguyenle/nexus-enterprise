## Context

Autopilot pipeline chạy 9 agents sequentially trong 1 conversation. Orchestrator switch personas và tự quyết định conflicts. Hiện tại chỉ có `reviews.yaml` (machine routing) và `qa-memory.json` (test data) — không có audit trail cho reasoning quality.

Tất cả thay đổi nằm trong `.agent/skills/autopilot/SKILL.md` — không có code changes.

## Goals / Non-Goals

**Goals:**
- Thêm evidence-based session-dialogue protocol vào autopilot SKILL.md
- Define template structure cho `session-dialogue.md` per-change artifact
- Define write triggers (khi nào Orchestrator phải viết)
- Define evidence requirements (file + location + observation)
- Define red flag detection rules
- Enforce validator verification (không chỉ "agree")

**Non-Goals:**
- Không thêm agents mới
- Không thay đổi pipeline flow (vẫn 9 phases)
- Không merge với reviews.yaml
- Không cho agents đọc session-dialogue.md

## Decisions

### 1. Evidence-based, không phải narrative
**Decision:** Mỗi entry phải có evidence trỏ tới file/location/observation cụ thể.
**Why:** Self-reported reasoning không tin được. Evidence cho phép human verify bằng cách đọc.
**Alternative rejected:** Free-form dialogue — dẫn tới roleplay không kiểm chứng được.

### 2. Write-only cho Orchestrator, read-only cho human
**Decision:** Agents KHÔNG đọc file này. Chỉ Orchestrator viết, chỉ human đọc.
**Why:** Nếu agents đọc → feedback loop → agents optimize cho "looking good" thay vì "being accurate".

### 3. Conflict-only logging, không phải full trace
**Decision:** Chỉ log decisions, conflicts, deviations — không log routine operations.
**Why:** Signal-to-noise ratio. Full trace quá verbose, human sẽ không đọc.

### 4. Status VERIFIED/UNVERIFIED thay vì chỉ confidence
**Decision:** Thêm status field ngoài confidence. VERIFIED = evidence + validator. UNVERIFIED = chưa đủ.
**Why:** Confidence alone là subjective. Status forces binary accountability.

### 5. Red flags detection bắt buộc
**Decision:** Orchestrator phải tự flag khi evidence yếu hoặc thiếu.
**Why:** Human reviewer cần biết đâu cần kiểm tra kỹ hơn. Tốt hơn là self-report weakness sớm.

## File Structure

### Template: `session-dialogue.md`

```markdown
# Session Dialogue: <feature-name>

> Evidence-based audit trail. Write-only: Orchestrator. Read-only: Human.
> KHÔNG narrative. KHÔNG roleplay. CHỈ structured reasoning.

---

## Decision Log

### D-001: <decision title>

| Field | Value |
|-------|-------|
| Phase | <phase number + name> |
| Proposer | <agent> |
| Proposal | <what they proposed> |
| Challenger | <agent> |
| Challenge | <why they disagreed> |
| Response | <proposer's counter> |
| Decision | <final choice> |
| Evidence | file: <path>, location: <line/section>, observation: <what was found> |
| Validator | <agent>, checked: <what they verified>, conclusion: <confirm/reject> |
| Status | VERIFIED / UNVERIFIED |
| Confidence | HIGH / MEDIUM / LOW |

---

## Handoff Log

### H-001: Phase <X> → Phase <Y>

| Field | Value |
|-------|-------|
| From | <agent> |
| To | <agent> |
| Output | <what was delivered> |
| Evaluation | ACCEPT / REJECT |
| Issues | <list or "none"> |
| Action | <what changed, or "proceed"> |

---

## Feedback Loops

### L-001: <agents involved>

| Field | Value |
|-------|-------|
| Trigger | <what started the loop> |
| Agents | <A ↔ B> |
| Rounds | <N> |
| Resolution | <what was decided> |
| Final status | RESOLVED / ESCALATED / MAX_LOOPS |

---

## Deviations

### V-001: <what was violated>

| Field | Value |
|-------|-------|
| Agent | <who deviated> |
| Source | <skill path / knowledge K-ID / architecture rule> |
| Expected | <what should have happened> |
| Actual | <what agent did instead> |
| Reason | <specific justification> |
| Evidence | file: <path>, location: <line/section>, observation: <why deviation was necessary> |
| Validator | <agent>, checked: <what they verified>, conclusion: <justified/unjustified> |
| Verdict | JUSTIFIED / UNJUSTIFIED |

---

## Red Flags

| # | Type | Description | Location |
|---|------|-------------|----------|
| 1 | <flag type> | <what's suspicious> | <D-/V- reference> |
```

### Write Triggers

| Trigger | Section | Condition |
|---------|---------|-----------|
| Decision Log | D-xxx | Agent makes non-trivial choice OR rejects upstream output |
| Handoff Log | H-xxx | Phase transition with REJECT or issues (skip clean ACCEPTs) |
| Feedback Loop | L-xxx | reviews.yaml creates a loop (QA→DEV→QA, SA→BA→SA) |
| Deviation | V-xxx | Agent breaks from skill/knowledge/architecture |
| Red Flag | table | Evidence missing, confidence mismatch, validator skip |

### Evidence Rules

| Rule | Enforcement |
|------|-------------|
| Every decision MUST have evidence | file + location + observation |
| Evidence MUST be specific | "common pattern" = REJECTED, must cite file:line |
| Validator MUST verify evidence | "agree" alone = REJECTED, must state what was checked |
| Confidence MUST match evidence | HIGH without multiple evidence sources = RED FLAG |
| Deviation MUST have validator | Unvalidated deviation = UNJUSTIFIED by default |

### Red Flag Types

| Type | Trigger |
|------|---------|
| MISSING_EVIDENCE | Decision has no file/location/observation |
| VAGUE_EVIDENCE | Evidence says "common pattern" or "best practice" without specifics |
| UNVALIDATED | Validator field empty or validator didn't check evidence |
| CONFIDENCE_MISMATCH | HIGH confidence with single weak evidence source |
| PHANTOM_CHALLENGE | Challenge exists but no actual reasoning conflict visible |

## Risks / Trade-offs

| Risk | Impact | Mitigation |
|------|--------|------------|
| Orchestrator fabricates evidence | False trust | Human cross-checks evidence paths — fabricated files are instantly detectable |
| Context overhead from writing | Slower pipeline | Conflict-only logging minimizes writes; most phases will write 0 entries |
| File grows too large | Human won't read | Red flags section at bottom surfaces what needs attention first |
| Self-reported red flags are unreliable | Misses real issues | True — but explicit red flag structure makes absence suspicious |
