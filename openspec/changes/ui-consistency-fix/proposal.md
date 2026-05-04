# UI Consistency Fix — System-Wide Stitch Enforcement

## Evidence Summary
- Backend: ✅ Exists — all services (approval, messaging, drive, workspace) are complete
- Frontend: ⚠️ Partial — modules exist but UI inconsistencies detected
- Proto: ✅ Complete — all gRPC contracts defined
- DB: ✅ Complete — all schemas migrated
- Dependencies: ✅ All services functional
- Stitch Project: ✅ "Nexus Hub" (14852434379132121789) — active design system

## Product Assessment
- Size: **M** (frontend-only fixes across 3 modules — no backend changes)
- Risk: **Low** (pure CSS/component adjustments, no logic changes)
- Target user: **All workspace users** (everyone sees inconsistent UI)
- Core action: Fix visual inconsistencies to match Stitch design system exactly

## Root Cause Analysis
1. Stitch design was not enforced — DEV interpreted design freely
2. No UI spec layer between Stitch and code
3. QA did not validate UI against Stitch
4. Modules evolved independently without cross-module consistency checks

## System-Level Requirements

### R1: Stitch is Source of Truth
- ALL UI must match latest Stitch designs from "Nexus Hub" project
- No using old/cached versions — always download latest
- No fabricating UI patterns — code implements design, not reinterprets

### R2: UI Spec Layer
- Every screen MUST have a UI spec derived from Stitch
- Spec includes: layout, components, actions, interactions
- DEV reads spec before coding; no spec → no coding

### R3: QA Validation
- QA validates UI vs Stitch
- Missing actions → REJECT
- Visual mismatch → REJECT

### R4: Forbidden
- ❌ Code UI without spec
- ❌ Interpret design freely
- ❌ Skip actions defined in design
- ❌ Use raw HTML elements when primitives exist

## Codebase Scan — Issues Found

### Module 1: Approval (`/approval`)
- **Stitch compliance**: ✅ Good — uses semantic tokens, Tabs, DataTable, ResponsiveDetailPanel
- **Issues detected**:
  - Tab layout uses horizontal scroll but no visual indicator of scrollability on mobile
  - Action button patterns use mix of primitives and inline patterns

### Module 2: Drive (`/drive`)
- **Stitch compliance**: ⚠️ Partial — mixes raw `<button>` elements with Button primitive
- **Issues detected**:
  - "New Folder" and "Upload" buttons use raw `<button>` with manual classes instead of `<Button>` primitive
  - View toggle (list/grid) uses raw `<button>` instead of IconButton primitive
  - Sidebar header uses plain text instead of consistent pattern
  - Drive sidebar "NAVIGATION" is all-caps raw text, inconsistent with other module headers

### Module 3: Chat (`/channels`)
- **Stitch compliance**: ✅ Good — uses semantic tokens, channel list, message bubbles
- **Issues detected**:
  - Section headers ("DEPARTMENTS", "DIRECT MESSAGES") are raw text
  - Channel list panel uses ad-hoc styling patterns

### Cross-Module Consistency Issues
1. **Raw `<button>` vs `<Button>` primitive**: Drive uses raw buttons; Approval uses Button primitive
2. **Header inconsistency**: Each module uses different header patterns
3. **Sidebar section labels**: Inconsistent casing and styling
4. **Navigation patterns**: Approval=tabs, Chat=sub-sidebar+tabs, Drive=sub-sidebar+filters

## Scope

### In scope
1. **Download latest Stitch screens** for all 3 modules (approval, chat, drive)
2. **Drive module**: Replace raw `<button>` elements with Button/IconButton primitives
3. **Drive module**: Normalize sidebar header to use consistent pattern
4. **Chat module**: Normalize section headers styling
5. **Cross-module**: Ensure consistent page header pattern
6. **Cross-module**: Ensure action button patterns are consistent

### Out of scope
- Backend changes (none needed)
- New features or functional changes
- Mobile-first redesign (separate initiative)
- Performance optimization

### Deferred
- Design-system-level component library extraction
- Automated Stitch compliance testing (CI/CD integration)
- Mobile breakpoint redesign

## Success Criteria
- All modules use Button/IconButton primitives (no raw `<button>` with manual styling)
- All module headers follow consistent pattern from Stitch
- Section labels follow consistent pattern across all module sidebars
- UI matches latest Stitch designs exactly
- Build passes with zero regressions
- Visual QA confirms Stitch token compliance at 1280px desktop
