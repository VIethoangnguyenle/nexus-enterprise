# gstack Activation — Design

## Principle

> Skill không phải ground truth. Knowledge không phải absolute truth.
> Specs + Architecture mới là source of truth.
>
> gstack không làm agent "nghĩ tốt hơn" — mà giúp agent
> "implement tốt hơn khi đã biết mình đang làm gì"

---

## 1. INDEX.md — Skill Catalog

**File**: `.agent/skills/INDEX.md`

Agent load skills từ `.agent/skills/`. Đọc INDEX trước khi chọn skill. Load 1-3 skills per task.

### Engineering Workflow

| Skill | When to use |
|-------|-------------|
| plan-eng-review | Architecture review, API design, system design changes |
| review | Code review & quality check after implementation |
| qa | Testing & validation before completing task |

### Go Backend

| Skill | When to use |
|-------|-------------|
| golang-grpc | gRPC handlers, servers, interceptors, proto patterns |
| golang-database | SQL queries, pgx, transactions, row scanning |
| golang-error-handling | Error wrapping, sentinel errors, error logging |
| golang-naming | Package/function/variable naming conventions |
| golang-observability | Structured logging, metrics, tracing, slog |
| golang-security | Input validation, crypto, secrets, injection prevention |
| golang-testing | Unit tests, table-driven, mocks, testify |

### Frontend

| Skill | When to use |
|-------|-------------|
| frontend-best-practices | React, TanStack Router/Query, Zustand patterns |
| component-reuse-checklist | GATE — before creating ANY new component |
| tailwindcss | Styling, design tokens, responsive layout |

---

## 2. DEV Skill Loading Protocol

### Conflict Resolution (BẮT BUỘC)

Priority khi conflict:

```
1. Specs + Architecture → ALWAYS win
2. NGAC domain rules → non-negotiable
3. Knowledge (validated, strict) → override skills
4. Reference Skills → apply CHỈ KHI không conflict
5. Knowledge (advisory / new) → consider, NOT override
```

**Skill vs Spec/Architecture:**
→ FOLLOW specs/architecture → DO NOT apply skill
→ Mark: "Skill [X] not applicable — conflicts with [spec/architecture]"

**Skill vs Knowledge (strict):**
→ FOLLOW knowledge → skill is overridden

**Skill vs Knowledge (advisory/new):**
→ Skill wins → knowledge chỉ tham khảo

**Skill vs Codebase:**
→ FOLLOW codebase pattern
→ Mark: "Skill outdated for current codebase"

**Skill vs Skill:**
→ Chọn: (1) phù hợp architecture, (2) phù hợp task context, (3) specific hơn

**Knowledge vs Knowledge:**
→ Chọn item match preconditions → KHÔNG chỉ dựa vào confidence

### Loading Steps

```
Step 1: Read .agent/skills/INDEX.md
Step 2: Task Analysis
  - Type: gRPC | DB | UI | testing | refactor
  - Has DB: yes/no
  - Has external call: yes/no
Step 3: Select 1-3 skills (Base + Optional)
Step 4: Apply patterns — MUST use skill if exists, DO NOT reinvent
```

### Trigger Table

| Task Type              | Base Skill              | Optional                              |
|------------------------|-------------------------|---------------------------------------|
| gRPC handler/server    | golang-grpc             | golang-database, golang-error-handling |
| SQL / DB queries       | golang-database         | golang-security                       |
| Go error patterns      | golang-error-handling   |                                       |
| Writing Go tests       | golang-testing          |                                       |
| Naming decisions       | golang-naming           |                                       |
| Frontend component     | frontend-best-practices | component-reuse-checklist (GATE)      |
| New UI component       | component-reuse-checklist | ux-design-system                    |
| Styling                | tailwindcss             | ux-design-system                      |
| Security concern       | golang-security         |                                       |

Fallback: No trigger match → scan INDEX.md → choose closest match.

---

## 3. Thinking Priority Changes

### Remove from non-DEV agents (CEO, BA, UX, QC, Autopilot)

```diff
 1. Specs + Architecture
 2. NGAC domain rules
 3. Role thinking
 4. Knowledge (strict)
 5. Knowledge (advisory)
 6. Knowledge (new)
-7. gstack thinking ← foundation
```

### DEV: replace with Reference Skills section

gstack không còn là thinking level — trở thành reference section riêng.

---

## 4. Workflow Skills → Pipeline Phases

| Skill | Pipeline Phase | Trigger |
|-------|---------------|---------|
| plan-eng-review | SA (Phase 4) | Architecture design, API changes, complex flows |
| review | SA-verify (Phase 7) | After DEV — check correctness, edge cases |
| qa | QA (Phase 8) | Before complete — validate behavior, permissions |
