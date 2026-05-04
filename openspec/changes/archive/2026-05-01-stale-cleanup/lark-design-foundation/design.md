## Context

The NGAC frontend uses Tailwind v4 CSS-first theme tokens in `index.css`, with 10 primitive components (`Button`, `Avatar`, `Badge`, `Input`, `Select`, `Heading`, `Text`, `IconButton`, `Spinner`, `Textarea`) and a workspace layout (`_workspace.tsx`) that composes `AppRail` + `ListPanel` + main content. Current visual language uses indigo accent (#6366f1), gradient buttons, glow shadows, and generous spacing — characteristic of a consumer app. Target is Lark dark mode: flat, dense, muted enterprise aesthetic.

## Goals / Non-Goals

**Goals:**
- Replace all design tokens with Lark-accurate values (colors, spacing, radius, typography)
- Flatten all primitive components — remove gradients, glows, decorative effects
- Tighten spacing globally to match Lark's information density
- Establish a design token contract that subsequent phases (Sidebar, Chat List, Drive) build upon

**Non-Goals:**
- Rewriting component structure or adding new components (Phase 2+)
- Changing routing, state management, or data fetching logic
- Sidebar redesign (Phase 2)
- Rich chat list items (Phase 3)
- Drive table columns (Phase 4)

## Decisions

### Decision 1: Color Token Mapping

**Choice**: Map directly from Lark screenshot color-picks to CSS custom properties.

| Token | Current | New (Lark) | Rationale |
|-------|---------|------------|-----------|
| `--color-bg-primary` | #0f1218 | #0f1115 | Slightly warmer near-black |
| `--color-bg-secondary` | #161b22 | #161a20 | Panel background |
| `--color-bg-tertiary` | #1c2128 | #1e2228 | Elevated surface |
| `--color-bg-rail` | #0d1117 | #0d1017 | Sidebar rail |
| `--color-bg-hover` | rgba(255,255,255,0.04) | #1f242b | Solid hover (Lark uses solid, not alpha) |
| `--color-bg-active` | rgba(99,102,241,0.12) | rgba(51,112,255,0.12) | Blue-tinted active |
| `--color-accent` | #6366f1 | #3370ff | Lark's primary blue |
| `--color-accent-hover` | #818cf8 | #4a85ff | Lighter blue on hover |
| `--color-text-primary` | #e8eaed | #e6eaf0 | Slightly cooler white |
| `--color-text-secondary` | #8b949e | #9aa4b2 | Mid-gray |
| `--color-text-muted` | #636e7b | #6b7480 | Dimmed text |
| `--color-border` | rgba(255,255,255,0.06) | rgba(255,255,255,0.08) | Slightly more visible |

**Alternatives considered**: Using Lark's exact hex values requires screenshot color-picking which may have minor inaccuracies. Accepted because visual matching > exact hex precision.

### Decision 2: Remove All Gradients and Glow Effects

**Choice**: Button primary becomes flat `bg-accent text-white`, no gradient, no shadow-glow.

Current primary button:
```
bg-gradient-to-r from-accent to-[#8b5cf6] text-white
shadow-[0_4px_15px_var(--color-accent-glow)]
hover:shadow-[0_6px_20px_var(--color-accent-glow)] hover:-translate-y-px
```

New:
```
bg-accent text-white hover:bg-accent-hover
```

**Rationale**: Lark uses zero decorative effects on buttons. Flat, functional, invisible until needed.

### Decision 3: Border Radius Tightening

**Choice**: `--radius-sm: 4px`, `--radius-md: 6px`, `--radius-lg: 8px` (from 6/10/14).

Lark uses very tight radii. No pill shapes, no large rounds except avatars.

### Decision 4: Topbar Height Reduction

**Choice**: 52px → 44px. Matches Lark's compact header density.

### Decision 5: System Font Stack

**Choice**: Keep `'Inter'` as primary but add system fallback. Inter is close enough to Lark's system sans-serif and already loaded.

### Decision 6: Scrollbar Behavior

**Choice**: Make scrollbar track fully transparent, thumb only visible on container hover. Width 4px.

```css
::-webkit-scrollbar { width: 4px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background: transparent; border-radius: 2px; }
*:hover > ::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.10); }
```

## Risks / Trade-offs

- **Visual regression across all modules** → Mitigated: tokens are centralized in `index.css`, so one file change propagates everywhere. Browser verification after each step.
- **Button readability with flatter style** → Mitigated: sufficient color contrast between flat blue and dark backgrounds.
- **Spacing reduction may cause text clipping** → Mitigated: only reduce padding, not remove it. Test with long text content.
