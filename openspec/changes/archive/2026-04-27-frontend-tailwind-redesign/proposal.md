# Frontend Tailwind Redesign — Lark-inspired UI

## What

Full frontend migration from monolithic vanilla CSS (838 lines in `index.css`) to **Tailwind CSS v4** with a **Lark-inspired 3-column layout** and **custom component architecture**. Redesign the entire UI with a "Constellation" visual concept and enterprise-grade aesthetics.

## Why

### Current Problems

1. **Monolithic CSS** — All 838 lines in a single `index.css`. No scoping, no co-location with components. Adding features = scrolling to find the right section.
2. **No reusable components** — Route files contain inline HTML. No shared Button, Card, Modal primitives. Every page rebuilds UI from raw CSS classes.
3. **2-column layout limits** — Current sidebar + content layout doesn't support the information density NGAC needs (channel list, document list, asset list).
4. **No design system** — CSS variables exist but no component abstractions. Inconsistent spacing, sizing, and interaction patterns across pages.

### Target State

- **Tailwind v4** — CSS-first configuration, utility classes co-located with components
- **3-column layout** — Lark-style: icon rail (48px) + list panel (~280px, collapsible) + content area
- **Custom component library** — Primitives → Composites → Patterns → Layouts (4-layer architecture)
- **Constellation visual concept** — AI-generated illustrations for empty states, leveraging the NGAC graph metaphor (nodes + edges = stars + constellation lines)
- **Compact enterprise density** — ~13-14px base font, tight spacing, professional feel

## Scope

### In Scope
- Tailwind v4 installation and configuration
- 10+ primitive components (Button, Input, Badge, Avatar, etc.)
- 6+ composite components (Card, Modal, DataTable, Tabs, etc.)
- 5+ pattern components (AppRail, ListPanel, ChatView, etc.)
- 3 layout components (AppLayout, AuthLayout, AssetLayout)
- Redesign all 15 route pages
- AI-generated Constellation illustrations for empty states
- Update 3 existing test files
- Delete `index.css` monolith

### Out of Scope
- Backend changes (zero backend impact)
- New features or API changes
- Mobile responsiveness (future change)

## Success Criteria

1. All pages render with new Tailwind-based design
2. 3-column layout functional with rail collapse
3. Zero regression in `test_app.sh` (59/59 pass)
4. Frontend tests updated and passing (17/17)
5. `index.css` reduced to Tailwind directives only (~10 lines)
6. Lighthouse performance score ≥ 90
