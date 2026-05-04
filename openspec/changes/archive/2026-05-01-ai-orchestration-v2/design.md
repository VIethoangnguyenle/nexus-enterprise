## Context

Hệ thống AI orchestration hiện tại sử dụng multi-agent pipeline (autopilot) để tự động hóa feature development. Pipeline gồm 5 phases (CEO → BA → UX → DEV → QC) nhưng thiếu: SA verification, knowledge retention, quality gates, và unified thinking priority.

Implement plan đã được review và approve qua 6 vòng architectural analysis. Plan nằm tại `implementation_plan.md` trong conversation artifacts.

Tất cả thay đổi nằm trong `.agent/` — không có code changes.

## Goals / Non-Goals

**Goals:**
- Rewrite 7 core agent skills với unified 7-level thinking priority
- Add mandatory pre-flight checklists cho mọi agent
- Add DEV quality gate (8-step pre-flight + 7-point code quality check)
- Add QA knowledge violation checking
- Upgrade pipeline 5 → 9 phases (thêm SA, SA verify, POLISH, LEARN)
- Create Knowledge Layer với hypothesis-based lifecycle
- Bootstrap 3 seed knowledge items
- Deprecate 3 redundant skills
- Update 3 supporting files

**Non-Goals:**
- Không thay đổi Go backend code
- Không thay đổi frontend code
- Không thêm tools/dependencies mới
- Không implement Knowledge Phase 2/3 (positive signal, CHALLENGED state) — chỉ Phase 1

## Decisions

### 1. Knowledge as Hypothesis (không phải Rule)
**Decision:** Knowledge items là hypotheses với varying confidence, không phải rules.
**Why:** Rules tạo lock-in. Hypotheses cho phép system self-correct qua thời gian.
**Alternative rejected:** Static rules — dẫn tới false pattern lock-in sau 20+ runs.

### 2. Simplified Lifecycle (3 states, không phải 5)
**Decision:** NEW → VALIDATED → DEPRECATED (Phase 1 only).
**Why:** 5-state lifecycle (draft/hypothesis/validated/challenged/deprecated) quá complex cho Phase 1. Validate qua runs thực tế trước khi evolve.
**Alternative rejected:** Full 5-state — thêm complexity nhưng chưa có data để justify.

### 3. Merged DEV Checklist (1 checklist, không phải 2)
**Decision:** Pre-flight + knowledge enforcement = 1 checklist 8 bước duy nhất.
**Why:** 2 checklists tách rời → DEV skip 1 trong 2. Merge đảm bảo complete coverage.

### 4. Outcome-based Signal (không phải self-report)
**Decision:** LEARN phase infer knowledge effectiveness từ QA outcomes, không từ agent tự claim.
**Why:** Agent self-report ("tôi đã dùng K-001") là unverifiable. QA outcomes là ground truth.

### 5. Bootstrap Seeds (3 items)
**Decision:** Tạo sẵn 3 knowledge items (permission-before-mutation, no-unused-imports, explicit-error-handling).
**Why:** Tránh "knowledge empty" period trong 10 runs đầu. Items này là universal best practices.

### 6. Execution Order: store → domain → handler → infrastructure
**Decision:** Implement core skills trước → supporting files → deprecation → knowledge init.
**Why:** Core skills là foundation. Supporting files reference core skills. Knowledge cần skills để consume.

## Risks / Trade-offs

| Risk | Impact | Mitigation |
|------|--------|------------|
| Agent ignores pre-flight checklist | Quality regression | QA explicitly checks for quality report in DEV output |
| Knowledge items become stale | False guidance | Confidence decay + standard tier auto-prune |
| Bootstrap seeds too generic | Low value early | Seeds are protected tier, universal patterns — low risk |
| 9-phase pipeline too long | Slower iteration | Each phase is lean. LEARN is conditional (may skip). |
| Thinking priority conflicts between old/new skills | Inconsistent behavior | All 7 skills rewritten atomically in Phase 1 |
