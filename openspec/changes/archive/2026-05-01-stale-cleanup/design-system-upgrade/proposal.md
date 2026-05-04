## Why

NGAC's frontend UI is functional but lacks the visual polish and information density expected of an enterprise workspace platform. After auditing the current design against Lark, Linear, Notion, Vercel, and Stripe design systems, we identified critical gaps: a shallow color palette (7 tokens vs. industry-standard 14+), no formalized typography scale, missing accessibility focus states, and one-speed animations. These deficiencies make the app feel like a prototype rather than a production enterprise tool. Upgrading the design tokens and CSS foundation is the highest-impact, lowest-effort improvement available — every component benefits immediately.

## What Changes

- **Expand color tokens** from 7 background shades to a 14-shade grayscale with cool blue undertone, plus semantic functional color pairs (fg + subtle bg for each status)
- **Formalize typography scale** as 11 named roles (18px max, 13px workhorse) with three-weight system (400/500/600), replacing ad-hoc Tailwind classes
- **Add motion tokens** — tiered durations (50ms instant → 300ms slow) and named easings, replacing universal `0.15s ease`
- **Add focus ring system** — double-ring pattern (`surface gap + accent ring`) on ALL interactive elements for WCAG AA accessibility
- **Update border tokens** — add `border-strong` (0.12 opacity) level and `border-solid` for structural dividers
- **Add depth/elevation tokens** — 6 named levels from Recessed to Overlay with shadow presets
- **Update `index.css`** — migrate all `@theme` tokens to match new system
- **Update primitive components** — Button, Input, Select, Badge, Avatar to consume new tokens

## Capabilities

### New Capabilities
- `design-tokens`: CSS custom property system — 14-shade grayscale, functional colors, typography scale, motion tokens, depth levels, border scale, and focus ring utilities
- `focus-accessibility`: Double-ring focus indicator system for all interactive elements (buttons, inputs, links, nav items)

### Modified Capabilities
- `lark-sidebar-layout`: Sidebar nav items updated to use new `gray-*` tokens and `text-small-ui` typography role instead of inline values
- `lark-data-table-layout`: DataTable headers/rows updated to use new `text-caption-ui` role and `gray-*` surface tokens
- `lark-messaging-layout`: Chat editor and message rows updated to use new typography and color tokens

## Impact

- **`frontend/src/index.css`**: Major rewrite of `@theme` block — all token names change
- **`frontend/src/components/primitives/*`**: All 10 primitives update to new token names
- **`frontend/src/components/composites/*`**: DataTable, PeekPanel, Modal, Card update tokens
- **`frontend/src/components/patterns/Sidebar.tsx`**: Color/typography token references change
- **`frontend/src/routes/_workspace/*`**: Inline Tailwind classes referencing old tokens need update
- **No backend changes** — purely frontend CSS/component work
- **No breaking API changes** — visual-only, all behavior preserved
- **`DESIGN.md`**: Already updated with target design system (serves as source of truth)
