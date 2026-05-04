---
name: component-reuse-checklist
description: Mandatory pre-flight checklist before creating any new UI component. Ensures existing primitives and composites are reused instead of duplicated. This is a GATE — no new component passes without completing this checklist.
user-invocable: false
---

# Component Reuse Checklist — MANDATORY Gate

**This skill is a HARD GATE.** Before writing ANY new React component — whether from Stitch export, a new feature, or a refactor — you MUST complete this checklist. No exceptions.

## When This Skill Triggers

- Creating a new `.tsx` file in `components/`
- Adding a modal, dialog, banner, button, spinner, or any interactive UI pattern
- Importing HTML/CSS from Stitch and converting to React
- Building a new feature that needs UI components

## Pre-Flight Checklist

Before writing a single line of JSX, answer these 5 questions:

### 1. Does this EXACT pattern exist in `primitives/`?

Check the inventory below. If yes → **USE IT. Do not rewrite.**

| Primitive | File | Variants/Props |
|-----------|------|----------------|
| `Button` | `primitives/Button.tsx` | primary, secondary, danger, ghost, success, outline, error |
| `IconButton` | `primitives/IconButton.tsx` | icon, size, onClick |
| `Input` | `primitives/Input.tsx` | label, error, disabled |
| `Textarea` | `primitives/Textarea.tsx` | label, rows |
| `Select` | `primitives/Select.tsx` | options, label |
| `Badge` | `primitives/Badge.tsx` | variant, size |
| `Avatar` | `primitives/Avatar.tsx` | src, name, size |
| `Spinner` | `primitives/Spinner.tsx` | size: sm, md, lg |
| `Text` | `primitives/Text.tsx` | variant, muted |
| `Heading` | `primitives/Heading.tsx` | level, className |

### 2. Does this PATTERN exist in `composites/`?

| Composite | File | Use Case |
|-----------|------|----------|
| `Modal` | `composites/Modal.tsx` | Any overlay dialog — `Modal`, `Modal.Header`, `Modal.Body`, `Modal.Actions` |
| `ConfirmDialog` | `composites/ConfirmDialog.tsx` | Any destructive/confirm action (delete, leave, revoke) |
| `AlertBanner` | `composites/AlertBanner.tsx` | Inline warning/error/info/success banners |
| `Card` | `composites/Card.tsx` | Content containers |
| `Tabs` | `composites/Tabs.tsx` | Tab navigation |
| `DataTable` | `composites/DataTable.tsx` | Data tables with columns |
| `PeekPanel` | `composites/PeekPanel.tsx` | Side peek panels |
| `Breadcrumbs` | `composites/Breadcrumbs.tsx` | Navigation breadcrumbs |
| `Timeline` | `composites/Timeline.tsx` | Activity timelines |

### 3. Does a SIMILAR component exist in another module?

Search with: `grep -r "pattern" frontend/src/components/`

If found → **EXTRACT to `composites/` or `primitives/`, then use.**

### 4. Am I about to copy-paste >5 lines of JSX?

If YES → **STOP. Extract to a component first.**

### 5. Can this be reused in ≥2 places?

If YES → Create in `composites/` (domain-agnostic) or `patterns/` (domain-aware).
If NO → OK to create in the specific module folder (e.g., `drive/`).

## Decision Tree

```
Need a UI element?
│
├─ Is it a button? ──────────────── → <Button variant="...">
├─ Is it a spinner? ─────────────── → <Spinner size="...">
├─ Is it a close/icon button? ───── → <IconButton icon={X}>
├─ Is it a modal/dialog? ───────── → <Modal> + sub-components
├─ Is it a confirm/destructive? ── → <ConfirmDialog>
├─ Is it an alert/warning banner? → <AlertBanner variant="...">
├─ Is it a data table? ──────────── → <DataTable>
├─ Is it a tab group? ──────────── → <Tabs>
│
├─ None of the above?
│  ├─ Can existing component be EXTENDED with a new prop/variant?
│  │  └─ YES → Extend existing component
│  │  └─ NO → Create new component (document why in comment)
```

## Forbidden Patterns

| ❌ FORBIDDEN | ✅ REQUIRED | Why |
|-------------|------------|-----|
| Inline SVG spinner | `<Spinner size="sm" />` | Spinner primitive exists |
| Inline modal shell (backdrop + container) | `<Modal>` compound | Modal handles backdrop, ESC, click-outside |
| Inline close button (`<button><X/></button>`) | `<IconButton icon={X} />` | IconButton handles styling + a11y |
| Inline danger button with 10+ classes | `<Button variant="error">` | Button primitive has variant system |
| Copy-paste >5 lines JSX between files | Extract to shared component | DRY principle |
| `<div className="fixed inset-0 z-50...">` for dialogs | `<Modal>` | Modal manages z-index + overlay |
| Raw `<svg>` for icons already in lucide-react | `import { Icon } from 'lucide-react'` | Consistent icon system |

## Stitch-to-React Conversion Rule

When converting Stitch HTML/CSS to React components:

1. **Map Stitch elements → existing components FIRST**
   - Stitch button → `<Button variant="...">`
   - Stitch dialog → `<Modal>`
   - Stitch alert → `<AlertBanner>`
   - Stitch spinner → `<Spinner>`

2. **Extract Stitch tokens → design system tokens**
   - Stitch colors → M3 token classes (`bg-primary`, `text-on-surface`, etc.)
   - Stitch spacing → 4px scale (`p-1` through `p-8`)
   - Stitch typography → font utilities (`font-h3`, `text-body-md`, etc.)

3. **If Stitch has a new pattern not in our system**
   - Create it as a COMPOSITE or PRIMITIVE (not inline)
   - Document it in this inventory
   - Ensure it's generic enough for reuse

## Self-Correction

After writing a component, verify:
- [ ] No inline SVG spinners (use `<Spinner>`)
- [ ] No inline modal shells (use `<Modal>`)
- [ ] No inline button styling (use `<Button variant>`)
- [ ] No copy-pasted JSX blocks
- [ ] No hardcoded close buttons (use `<IconButton>`)
- [ ] Domain component LOC < 50 (if more → probably not composing correctly)

**If ANY check fails → FIX before committing.**
