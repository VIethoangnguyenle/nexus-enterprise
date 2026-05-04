## Why

The NGAC frontend currently uses an improvised dark theme with bright gradients, glowing shadows, and generous spacing that feels "consumer app" rather than "enterprise tool". The target is pixel-fidelity with Lark's dark mode design language — dense, minimal, functional, professional. Phase 1 establishes the design foundation: color tokens, typography, primitives, and spacing rules that every subsequent phase (Sidebar, Chat List, Drive Table) will build upon.

## What Changes

- **Flatten color tokens** — replace indigo accent (`#6366f1`) with Lark's muted blue (`#3370ff`), remove gradient backgrounds and glow shadows, tighten border radius from 6-14px to 4-8px.
- **Rewrite Button primitive** — remove gradient + glow shadow from primary variant, make all variants flat and subtle per Lark's interaction language.
- **Tighten spacing** — reduce topbar height from 52px to 44px, list item padding from py-2.5 to py-1.5, panel headers from py-3 to py-2.
- **Update typography** — switch from Inter to system sans-serif stack, reduce font weights to match Lark's medium/regular pairing.
- **Restyle scrollbar** — make scrollbar invisible by default, visible only on hover (Lark pattern).
- **Update Avatar, Badge, Input, Select primitives** — align sizes, colors, borders with Lark design tokens.
- **Remove decorative elements** — eliminate `backdrop-blur`, gradient overlays, `shadow-glow`, and `shadow-lg` from workspace layout.

## Capabilities

### New Capabilities
- `lark-design-tokens`: CSS custom properties and Tailwind theme tokens matching Lark dark mode color system, spacing scale, typography, and border styles.

### Modified Capabilities
_(none — this is a visual-only change, no spec-level behavior changes)_

## Impact

- **Files affected**: `index.css` (design tokens), all 10 primitives in `components/primitives/`, workspace layout `_workspace.tsx`, topbar styling.
- **No API changes** — purely frontend CSS/component updates.
- **No routing changes** — same component structure, just restyled.
- **Risk**: visual regression across all modules (messaging, drive, documents, assets). Mitigated by phased approach and browser verification after each change.
