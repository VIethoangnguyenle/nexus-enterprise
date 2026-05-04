# Responsive-First UI + Tailwind CSS Refactor

## What

Comprehensive responsive-first refactoring of the entire NGAC frontend, plus codifying responsive rules into AGENTS.md and the frontend-best-practices skill. The project already uses Tailwind CSS v4 but lacks systematic responsive patterns.

## Why

1. **Current state is desktop-only**: Only the auth layout (`_auth.tsx`) has `hidden lg:flex` — every other layout breaks on mobile/tablet.
2. **Mixed CSS approach**: Tailwind v4 is installed and used for utility classes, but the styling.md reference says "Vanilla CSS only — no Tailwind" — this is contradictory and confusing for agents.
3. **Enterprise expectation**: Users access the platform on tablets (iPad in meetings) and phones (checking messages on the go). A desktop-only app loses these use cases.
4. **Agent rule gap**: No responsive rules in AGENTS.md means every new feature ships desktop-only.

## Scope

### Part A: Rule & Skill Updates (Documentation)
1. **AGENTS.md** — Add responsive-first rules to the non-negotiable section
2. **frontend-best-practices skill** — Add `responsive.md` reference with detailed patterns
3. **Update `styling.md`** — Fix the "no Tailwind" contradiction, document actual Tailwind v4 usage
4. **Add tailwindcss skill** — NGAC-specific Tailwind v4 patterns and design token integration

### Part B: CSS Infrastructure
1. **Refactor `index.css`** — Replace `@media (max-width)` desktop-first queries with Tailwind v4 responsive utilities where possible
2. **Keep `@theme` tokens** — Design tokens stay in CSS (this is correct for Tailwind v4)
3. **Move component styles to Tailwind utilities** — Sidebar, chat header, message bubble, etc.

### Part C: Layout Responsive Migration
Each layout route gets responsive treatment:

| Route | Current | Target |
|-------|---------|--------|
| `_auth.tsx` | ✅ Split (has `lg:`) | Polish only |
| `_workspace.tsx` | ❌ 3-column fixed | Drawer nav + stacked on mobile |
| `_drive.tsx` | ❌ Sidebar + table | Collapsed sidebar + card list on mobile |
| `_assets.tsx` | ❌ Sidebar + table | Bottom nav + simplified list on mobile |
| Channel view | ❌ Fixed panels | Full-screen chat on mobile |
| Documents | ❌ Fixed layout | Simplified on mobile |

## Out of Scope

- No backend changes
- No new features — only layout/styling adjustments
- No design system color/token changes (keep existing palette)
