## Context

NGAC's frontend uses Tailwind v4 with custom CSS tokens defined in `frontend/src/index.css`. The current token set has 7 background shades, 3 text colors, 1 accent, and a handful of semantic colors. Typography is applied via ad-hoc Tailwind utility classes with no formalized scale. Animations use a single `0.15s ease` for everything.

The `DESIGN.md` at project root has been updated with the target design system — synthesized from Lark (density + layout), Linear (dark surfaces), Notion (workspace patterns), Vercel (shadow engineering), and Stripe (data precision). This document is the source of truth for visual decisions.

Components are structured in three layers:
- **Primitives** (`components/primitives/`): Button, Input, Select, Badge, Avatar, etc.
- **Composites** (`components/composites/`): DataTable, PeekPanel, Modal, Card, Tabs
- **Patterns** (`components/patterns/`): Sidebar, FilePreviewCard

All primitives currently use inline Tailwind classes referencing `bg-bg-primary`, `text-text-primary`, etc.

## Goals / Non-Goals

**Goals:**
- Replace all CSS custom properties in `index.css` `@theme` block with the 14-shade grayscale + functional color system defined in `DESIGN.md`
- Define named typography utility classes (`.text-title`, `.text-body-ui`, `.text-caption-ui`, etc.) as Tailwind `@utility` directives
- Define motion tokens (durations + easings) as CSS custom properties
- Add depth/elevation token presets as utility classes
- Add double-ring focus utility (`.focus-ring`) consuming the new tokens
- Update all 10 primitive components to reference new token names
- Ensure zero visual regression on existing functionality — only the look should improve, not behavior

**Non-Goals:**
- Sidebar layout refactor (Rail/Panel split) — separate change
- Command palette (`Cmd+K`) implementation — separate change
- Context menu (right-click) system — separate change
- Drag and drop functionality — separate change
- DataTable advanced features (sort, resize, bulk actions) — separate change
- New component creation (Toast, Skeleton, Tooltip) — separate change

## Decisions

### Decision 1: Token naming convention — `gray-N` scale vs. semantic names

**Chosen**: Hybrid — CSS properties use `--gray-N` (1–14), Tailwind aliases map semantic names (`--color-bg-primary → var(--gray-4)`) for backward compatibility.

**Alternatives considered**:
- Pure semantic only (`--bg-primary`, `--bg-surface`) — not enough granularity for 14 shades
- Pure numeric only (`--gray-1` through `--gray-14`) — forces developers to memorize roles

**Rationale**: Hybrid lets DESIGN.md dictate exact gray values while Tailwind aliases provide DX-friendly names. Existing code referencing `bg-bg-primary` continues to work.

### Decision 2: Typography — CSS custom properties + `@utility` directives

**Chosen**: Define font-size/weight/line-height/letter-spacing combos as `@utility` classes (e.g., `@utility text-title { font-size: 18px; font-weight: 600; line-height: 1.35; letter-spacing: -0.3px; }`).

**Alternatives considered**:
- Tailwind `fontSize` theme extension — doesn't bundle weight/line-height/letter-spacing
- Component-level CSS modules — fragments the type system, hard to enforce consistency

**Rationale**: `@utility` directives in Tailwind v4 produce real utility classes that work in `className` strings. One class = one complete type role. This is the most ergonomic approach for the existing codebase.

### Decision 3: Focus ring — double-ring pattern

**Chosen**: `box-shadow: 0 0 0 2px var(--gray-4), 0 0 0 4px var(--color-accent)` — a 2px surface-colored gap then 2px accent ring.

**Alternatives considered**:
- CSS `outline` only — can't create the gap, looks harsh on dark surfaces
- `ring` Tailwind utilities — not enough control for the double-ring pattern

**Rationale**: Double-ring (learned from Vercel/Linear) creates visual separation between the focus indicator and the element. On dark backgrounds, the gap prevents the accent ring from bleeding into adjacent elements.

### Decision 4: Migration strategy — in-place token rename with Tailwind aliases

**Chosen**: Update `@theme` block to define new `--gray-*` tokens + remap old aliases (`--color-bg-primary: var(--gray-4)`) so existing code doesn't break. Then incrementally update components.

**Alternatives considered**:
- Big-bang rename all at once — risky, hard to verify
- Create parallel token system, deprecate old — bloats CSS

**Rationale**: Aliases mean zero breakage on day 1. Components can be migrated file-by-file, each using the audit results to map old → new.

## Risks / Trade-offs

- **[Risk] Token rename misses a reference** → Mitigation: grep for all `bg-`, `text-`, `border-` Tailwind class usage before and after. Verify with visual diff (open app, navigate all pages).
- **[Risk] Typography scale feels too dense on smaller screens** → Mitigation: 13px workhorse size is standard for Lark/Linear-class apps. If user feedback says too small, we can bump to 14px without breaking the scale.
- **[Risk] Focus rings clash with existing hover states** → Mitigation: Focus rings only appear on `:focus-visible` (keyboard navigation), not `:focus` (mouse clicks). This is standard best practice.
- **[Trade-off] 14-shade grayscale is more complex to maintain** → Accepted: The density of visual states (hover, active, selected, disabled) in enterprise apps requires this granularity. Fewer shades = impossible to distinguish states.
