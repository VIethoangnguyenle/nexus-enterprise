# Component Reusability Refactor — Tasks

## Phase 1: Enforcement — Skill & Rules (GATE — trước mọi thứ)

- [x] **Task 1.1**: Create `component-reuse-checklist` skill
- [x] **Task 1.2**: Update `component-design.md` with "Reuse Before Create"
- [ ] **Task 1.3**: Update AGENTS.md forbidden patterns (deferred — low priority)

---

## Phase 2: Foundation — Upgrade Primitives & Composites

- [x] **Task 2.1**: Add `variant="error"` to Button primitive
- [x] **Task 2.2**: Upgrade Modal.tsx → M3 tokens + Modal.Header
- [x] **Task 2.3**: Create AlertBanner composite
- [x] **Task 2.4**: Create ConfirmDialog composite

---

## Phase 3: Refactor Domain Components

- [x] **Task 3.1**: Refactor DeleteConfirmDialog (136 → 48 LOC)
- [x] **Task 3.2**: Refactor MoveItemDialog (123 → 72 LOC)
- [x] **Task 3.3**: Clean FolderTreeSelect inline spinners (4 × SVG → Spinner)

---

## Phase 4: Verify

- [x] **Task 4.1**: Build verification — ✅ 2305 modules, 514ms, 0 errors
- [ ] **Task 4.2**: Visual regression check (needs manual browser test)
