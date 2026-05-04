# System Review — NGAC Platform

> Completed: 2026-05-01 | Status: Phase 1 COMPLETE

---

## Executive Summary

The NGAC platform is **functional but inconsistent**. Core features work (chat, drive, approval, assets, contacts), architecture is sound at the macro level (microservices + NGAC), but execution quality varies dramatically between services and modules.

**Overall Quality Score: 60%**

---

## Audit Reports

| Report | File | Key Findings |
|--------|------|-------------|
| 🎨 **Design Audit** | [design-audit.md](./design-audit.md) | Conflicting design docs, dual token system, hardcoded colors |
| 🏗️ **Architecture Audit** | [architecture-audit.md](./architecture-audit.md) | 2 services violate clean arch, 1 production bug found |
| ✨ **Feature Audit** | [feature-audit.md](./feature-audit.md) | Route files oversized, no pagination, inconsistent empty states |
| 🔍 **Code Audit** | [code-audit.md](./code-audit.md) | 8 swallowed errors, 0% frontend tests, God hook pattern |

---

## Top 10 Issues (Prioritized)

### 🔴 Critical (Fix Immediately)

| # | Issue | Impact | Location |
|---|-------|--------|----------|
| 1 | **TransferOwnership bug** — uses `ws.Name` instead of `ws.Id` for NGAC lookup | **Data corruption**: transferred ownership fails for renamed workspaces | `workspace/grpc/server.go:255` |
| 2 | **8 swallowed errors** — `ws, _ := s.GetWorkspace(...)` | **Silent failures**: nil pointer panics in production | `workspace/grpc/server.go` (8 locations) |
| 3 | **Dead DESIGN.md** — describes dark theme that doesn't exist | **Developer confusion**: new devs follow wrong spec | `/DESIGN.md` (root) |

### 🟠 High (Standardization Blockers)

| # | Issue | Impact | Location |
|---|-------|--------|----------|
| 4 | **Workspace service: no domain/store layers** | Violates clean architecture, blocks testing | `workspace/grpc/server.go` (387 lines monolith) |
| 5 | **Drive gRPC handler: 645 lines monolith** | Business logic in handler, hard to test | `drive/grpc/server.go` |
| 6 | **Dual token system** (legacy gray + M3) | Inconsistent UX, developer confusion | `index.css` + 28+ component files |
| 7 | **Hardcoded Tailwind colors** in Contacts | Bypasses design system entirely | `ContactsTable`, `ContactCard`, `ContactProfilePanel` |

### 🟡 Medium (Quality Improvements)

| # | Issue | Impact | Location |
|---|-------|--------|----------|
| 8 | **Route files too large** (4 routes 250-503 lines) | Hard to maintain, poor separation of concerns | `channels.$channelId`, `approval`, `drive`, `contacts` |
| 9 | **God hook pattern** — `useMessaging` 434 lines | Blocks testing, excessive re-renders | `hooks/useMessaging.ts` |
| 10 | **No pagination** anywhere | Performance issues at scale | All list endpoints |

---

## Cleanup Completed

### Archived Stale OpenSpec Changes: 26 items

All 26 orphaned changes (no status file, no active work) moved to `archive/2026-05-01-stale-cleanup/`:

<details>
<summary>Full list of archived changes</summary>

1. ai-product-os-upgrade
2. approval-module-frontend
3. approval-redesign-v2
4. chat-member-management
5. chat-ui-stitch-parity
6. clean-reseed-db
7. component-reusability-refactor
8. design-system-upgrade
9. drive-file-folder-management
10. fix-channel-access-denied
11. fix-file-display
12. fix-file-upload-pipeline
13. fix-shared-folders-display
14. fix-websocket-realtime
15. gstack-activation
16. lark-chat-header
17. lark-design-foundation
18. lark-ui-redesign
19. multi-user-chat-test
20. realtime-websocket-cache-injection
21. responsive-tailwind-refactor
22. stitch-full-ui-refactor
23. stitch-skills-integration
24. stitch-ui-parity-3phase
25. system-audit-bugfix
26. ux-ui-standardization
</details>

---

## Recommendations — Phase 2 Execution Order

### Wave 1: Fix Critical Bugs + Design Foundation (1-2 sessions)

1. **Fix TransferOwnership bug** — change `ws.Name` → `ws.Id` in NGAC lookup
2. **Fix all 8 swallowed errors** — add proper error handling in workspace service
3. **Deprecate root DESIGN.md** — add deprecation notice, link to `.stitch/DESIGN.md`
4. **Unify token system** — migrate legacy gray aliases to M3 semantic tokens in `index.css`

### Wave 2: Architecture Compliance (2-3 sessions)

5. **Refactor workspace service** — extract domain/ and store/ layers
6. **Refactor drive gRPC handler** — extract domain layer (ensureRoot, quota, access patterns)
7. **Fix Contact components** — replace hardcoded colors with semantic tokens

### Wave 3: Frontend Standardization (2-3 sessions)

8. **Split oversized routes** — extract ChannelView, ApprovalTabs, DriveGrid, ContactsView
9. **Split useMessaging** — separate channel queries, message mutations, WebSocket ops
10. **Create shared patterns** — MobileOverlay, EmptyState, StatusBadge

### Wave 4: Quality Gates (1-2 sessions)

11. **Add error boundaries** — top-level + per-module
12. **Add pagination** — cursor-based for all list endpoints
13. **PeekPanel: use IconButton** — replace inline close button

---

## Service Quality Grades

| Service | Architecture | Code Quality | Tests | Overall |
|---------|-------------|-------------|-------|---------|
| **Approval** | 🟢 A | 🟢 A | 🟢 A | 🟢 **A** |
| **Messaging** | 🟢 A | 🟢 A | 🟡 B | 🟢 **A-** |
| **Policy** | 🟢 A | 🟢 A | 🟢 A | 🟢 **A** |
| **Auth** | 🟢 A | 🟡 B | 🟡 B | 🟡 **B+** |
| **Asset** | 🟢 A | 🟡 B | 🟡 B | 🟡 **B+** |
| **Drive** | 🟡 B | 🟡 B | 🟡 C | 🟡 **B-** |
| **Workspace** | 🔴 D | 🔴 D | 🔴 D | 🔴 **D** |
| **Document** | 🔴 F | 🟡 C | 🔴 D | 🔴 **F** |

---

*Next step: Phase 2 execution starts with Wave 1 (Critical Bug Fixes + Design Foundation).*
