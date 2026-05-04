# ⚠️ DEPRECATED — DO NOT USE

> **This document is OBSOLETE.** It describes a dark-mode design system that was never fully implemented.
>
> **Canonical source of truth:** [`.stitch/DESIGN.md`](.stitch/DESIGN.md)
>
> The production system uses: **Manrope font**, **light-mode M3 tokens**, **`#2563EB` primary**.
> This file is preserved for historical reference only. All new work MUST follow `.stitch/DESIGN.md`.
>
> — Deprecated 2026-05-01 during System Standardization audit

---

# ~~NGAC Design System — Enterprise Workspace~~ (ARCHIVED)

> Synthesized from: Lark (layout + density), Linear (dark surfaces + borders),
> Notion (workspace patterns), Vercel (shadow engineering), Stripe (data precision).
> Purpose: ~~AI agent reference for consistent, enterprise-grade UI generation.~~ DEPRECATED.

---

## 1. Design Philosophy

NGAC is a **dark-mode-native enterprise workspace** — messaging, documents, drive, and
access control unified in one dense interface. The design language draws from Lark's
"zero toggle tax" approach: users never leave context to complete a task.

**Core Principles:**
- **Density over decoration** — 4px base unit, 13px workhorse text, 36px table rows
- **Darkness as medium** — surfaces defined by luminance stepping, not color
- **Function before form** — every visual element serves an information purpose
- **Context preservation** — side-panels over modals, inline editing over page navigation
- **Cool precision** — blue-undertone grays, single blue accent, no warm colors

---

## 2. Color System — 14-Shade Grayscale + Functional

### Surface Scale (cool blue undertone)

All surfaces use a carefully calibrated grayscale. Each step is visually
distinguishable but subtle — one shade = one hover state, two shades = active state.

| Token | Hex | Role |
|-------|-----|------|
| `gray-1` | `#08090a` | Deepest app background, behind everything |
| `gray-2` | `#0d1017` | Icon Rail background |
| `gray-3` | `#0f1115` | Sidebar / List Panel background |
| `gray-4` | `#141720` | Content area background |
| `gray-5` | `#1a1e26` | Card / elevated surface |
| `gray-6` | `#21252e` | Hover state on surfaces |
| `gray-7` | `#282d37` | Active/pressed surface, selected row bg |
| `gray-8` | `#323843` | Strong surface, toolbar bg |

### Content Scale

| Token | Hex | Role |
|-------|-----|------|
| `gray-9` | `#525a68` | Subtle icons, disabled controls |
| `gray-10` | `#6b7480` | Muted text — timestamps, placeholders |
| `gray-11` | `#8b929e` | Secondary text — descriptions, metadata |
| `gray-12` | `#c0c6d0` | Primary body text |
| `gray-13` | `#e2e5eb` | Headings, active nav items |
| `gray-14` | `#f0f2f5` | Brightest text (use sparingly) |

### Functional Colors (semantic, always paired with subtle bg)

| Role | Foreground | Background (8% opacity) | Use |
|------|-----------|------------------------|-----|
| **Primary / Action** | `#3370FF` | `rgba(51,112,255,0.08)` | CTAs, links, active states, brand |
| **Primary Hover** | `#4B83FF` | `rgba(51,112,255,0.12)` | Hover on primary elements |
| **Success** | `#22C55E` | `rgba(34,197,94,0.08)` | Online, completed, confirmed |
| **Warning** | `#F59E0B` | `rgba(245,158,11,0.08)` | Expiring, attention needed |
| **Error / Danger** | `#EF4444` | `rgba(239,68,68,0.08)` | Failed, destructive action |
| **Info** | `#06B6D4` | `rgba(6,182,212,0.08)` | Informational badges |

### Border & Divider

| Token | Value | Use |
|-------|-------|-----|
| `border-subtle` | `rgba(255,255,255,0.05)` | Table row dividers, sidebar section breaks |
| `border-default` | `rgba(255,255,255,0.08)` | Cards, inputs, panels |
| `border-strong` | `rgba(255,255,255,0.12)` | Active inputs, emphasized containers |
| `border-solid` | `#23252a` | Structural dividers (rail/sidebar boundary) |

### Overlay

| Token | Value | Use |
|-------|-------|-----|
| `overlay-backdrop` | `rgba(0,0,0,0.60)` | Modal/dialog backdrop |
| `overlay-panel` | `rgba(0,0,0,0.40)` | Side-panel overlay on mobile |

---

## 3. Typography — Inter Variable, Dense Scale

Font: **Inter Variable** with OpenType `"cv01", "ss03"` on ALL text.
Monospace: **JetBrains Mono** (fallback: `ui-monospace, SF Mono, Menlo`).

### Type Scale

| Role | Size | Weight | Line-H | Letter-Sp | Token | Use |
|------|------|--------|--------|-----------|-------|-----|
| Page Title | 18px | 600 | 1.35 | -0.3px | `text-title` | Module headers: "Drive", "Messages" |
| Section | 15px | 600 | 1.40 | -0.15px | `text-section` | Panel headers, group labels |
| Body | 14px | 400 | 1.50 | 0 | `text-body` | Standard reading text |
| Body UI | 14px | 500 | 1.50 | 0 | `text-body-ui` | Nav items, table headers, form labels |
| Body Strong | 14px | 600 | 1.50 | 0 | `text-body-strong` | Active states, emphasis |
| Small | 13px | 400 | 1.45 | 0 | `text-small` | Sidebar items, secondary content |
| Small UI | 13px | 500 | 1.45 | 0 | `text-small-ui` | Sub-nav, tab labels |
| Caption | 12px | 400 | 1.40 | 0 | `text-caption` | Timestamps, file sizes |
| Caption UI | 12px | 500 | 1.40 | 0 | `text-caption-ui` | Column headers, badge text |
| Overline | 11px | 600 | 1.35 | 0.5px | `text-overline` | Section labels (uppercase) |
| Micro | 10px | 500 | 1.30 | 0.3px | `text-micro` | Status badges, counters |

### Rules
- **Maximum 18px** in workspace app — no display/hero typography
- **13px is the workhorse** — sidebar nav, metadata, secondary content
- **Three weights**: 400 (read), 500 (interact), 600 (announce)
- **Always set** `font-feature-settings: "cv01", "ss03"` on root element
- **Negative letter-spacing** ONLY at 15px+ sizes
- **Overline** is the ONLY role using uppercase + positive letter-spacing

---

## 4. Layout — 4-Column Workspace (Lark Pattern)

```
┌─────────────────────────────────────────────────────────────┐
│ ┌────┐ ┌──────────┐ ┌──────────────────────┐ ┌───────────┐ │
│ │    │ │          │ │                      │ │           │ │
│ │ R  │ │  List    │ │    Content           │ │   Peek    │ │
│ │ A  │ │  Panel   │ │    Area              │ │   Panel   │ │
│ │ I  │ │          │ │                      │ │           │ │
│ │ L  │ │  240px   │ │    flex-1            │ │   360px   │ │
│ │    │ │  resize  │ │                      │ │   slide   │ │
│ │48px│ │ 180-320  │ │  Table / Editor /    │ │  in/out   │ │
│ │    │ │          │ │  Chat / Grid         │ │           │ │
│ │    │ │          │ │                      │ │           │ │
│ └────┘ └──────────┘ └──────────────────────┘ └───────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Column 1: Icon Rail (48px)
- Background: `gray-2` (`#0d1017`)
- Module icons: 20px, `gray-10` default, `primary` when active
- Active indicator: 3px left border `primary`, bg `primary-bg`
- Workspace avatar: top, 32px rounded-lg
- User avatar: bottom, 28px
- Always visible, never collapses

### Column 2: List Panel (240px, resizable 180–320px)
- Background: `gray-3` (`#0f1115`)
- Right border: `border-solid` (`#23252a`)
- Context-specific content:
  - **Messaging**: Channel list with unread badges
  - **Drive**: Folder tree with breadcrumbs
  - **Documents**: Doc tree sidebar
  - **Settings**: Settings nav
- Section label: `text-overline`, `gray-10`, uppercase
- Items: `text-small-ui`, `gray-11`, padding 6px 12px, radius 6px
- Active item: `primary-bg`, `gray-13` text
- Collapse: hides to show Rail only (keyboard `[`)

### Column 3: Content Area (flex-1, min-width 480px)
- Background: `gray-4` (`#141720`)
- Header bar: 44px, `gray-3` bg, bottom border
- Content padding: 0 (tables edge-to-edge) or 20px (editors)
- This is where DataTable, ChatView, Editor, or Grid renders

### Column 4: Peek Panel (360px, conditional)
- Background: `gray-4` (`#141720`)
- Left border: `border-solid` (`#23252a`)
- Slide-in from right, 200ms ease-out
- Close: Escape key, close button, or click outside
- Content: item details, thread view, file preview, permissions
- Resize handle on left edge (min 280px, max 480px)

### Responsive Breakpoints

| Breakpoint | Width | Behavior |
|-----------|-------|----------|
| Wide | >1440px | All 4 columns visible |
| Standard | 1024–1440px | Rail + List + Content; Peek overlays |
| Compact | 768–1024px | Rail only + Content; List as overlay |
| Mobile | <768px | Bottom tab bar replaces Rail; full-screen pages |

---

## 5. Component Specifications

### Buttons

| Variant | Background | Text | Border | Hover | Use |
|---------|-----------|------|--------|-------|-----|
| Primary | `#3370FF` | `#fff` | none | `#4B83FF` | Main CTA |
| Secondary | `gray-5` | `gray-12` | `border-default` | `gray-6` | Secondary action |
| Ghost | transparent | `gray-11` | none | `gray-6` bg | Tertiary action |
| Danger | `rgba(239,68,68,0.08)` | `#EF4444` | none | `rgba(239,68,68,0.15)` | Destructive |
| Icon | transparent | `gray-10` | none | `gray-6` bg | Toolbar icon |

All buttons: radius 6px, font `text-small-ui` (13px/500), padding 6px 14px (md).
**Focus ring**: `0 0 0 2px gray-4, 0 0 0 4px #3370FF` (double ring).

### DataTable

- **Header**: `gray-3` bg, `text-caption-ui` (12px/500 uppercase), `gray-10` text
- **Row height**: 36px (dense) / 44px (comfortable)
- **Row border**: `border-subtle` bottom
- **Row hover**: `gray-6` bg
- **Row selected**: `primary-bg` (`rgba(51,112,255,0.08)`)
- **Cell padding**: 0 12px
- **Sort icon**: `gray-10`, active `primary`
- **Checkbox column**: 36px width
- **Context menu**: right-click → dropdown with actions

### Sidebar Nav Item

```
┌─────────────────────────────────┐
│ [icon 16px]  Label        [+]  │  ← 13px/500, gray-11
│              ────────────────  │
│              # channel-1       │  ← 13px/400, gray-11, pl-36px
│              # channel-2  (3)  │  ← unread count badge
│              # channel-3       │
└─────────────────────────────────┘

Active parent:  bg primary-bg, text gray-13, icon primary
Active child:   text primary, font-weight 500
Hover:          bg gray-6, text gray-12
```

### PeekPanel

- Width: 360px default, resizable 280–480px
- Header: 44px, `text-body-strong`, close IconButton top-right
- Tabs (optional): `text-small-ui`, bottom border indicator
- Animation: `transform: translateX(100%) → translateX(0)`, 200ms ease-out
- Close animation: reverse, 150ms ease-in

### Modal / Dialog

- Backdrop: `overlay-backdrop` (`rgba(0,0,0,0.60)`)
- Container: `gray-5` bg, `border-default` border, 12px radius
- Max-width: sm(380px) / md(480px) / lg(640px)
- Animation: fade-in + scale(0.97→1), 200ms ease-out
- Close: Escape key, click backdrop, close button
- Title: `text-section` (15px/600)

### Input / Form Controls

- Height: 32px (compact) / 36px (default)
- Background: `gray-1` (`#08090a`) — recessed look
- Border: `border-default`, focus: `#3370FF`
- Text: `gray-12`, placeholder: `gray-10`
- Radius: 6px
- Focus: `0 0 0 2px gray-4, 0 0 0 4px #3370FF`
- Error: border `#EF4444`, helper text below in `#EF4444`

### Badges & Status

| Type | Background | Text | Radius | Size |
|------|-----------|------|--------|------|
| Count (unread) | `#3370FF` | `#fff` | 9px | min-w 18px, `text-micro` |
| Status dot | semantic color | — | 50% | 8px |
| Tag | `gray-6` | `gray-12` | 4px | `text-caption` |
| Pill | semantic bg (8%) | semantic fg | 9999px | `text-micro` |

### Toast / Notification

- Position: bottom-right, 16px from edges
- Background: `gray-5` with `border-default`
- Shadow: `0 8px 24px rgba(0,0,0,0.4)`
- Duration: 4s default, 8s for errors
- Animation: slide-up 200ms ease-out, slide-down 150ms ease-in

---

## 6. Depth & Elevation

| Level | Treatment | Use |
|-------|-----------|-----|
| Recessed | `gray-1` bg (darkest) | Input fields, recessed wells |
| Base | `gray-3` / `gray-4` bg | Sidebar, content backgrounds |
| Surface | `gray-5` bg + `border-default` | Cards, elevated panels |
| Raised | `gray-5` bg + `0 4px 12px rgba(0,0,0,0.3)` | Dropdowns, tooltips |
| Floating | `gray-5` bg + `0 8px 24px rgba(0,0,0,0.4)` | Command palette, toasts |
| Overlay | `gray-5` bg + `0 16px 48px rgba(0,0,0,0.5)` | Modals, full dialogs |

**Philosophy** (learned from Linear + Vercel): On dark surfaces, elevation is
communicated through **background luminance stepping** — each layer slightly lighter.
Shadows supplement but don't drive depth. Borders (`rgba(255,255,255,0.08)`) are
the primary visual separator.

---

## 7. Motion & Animation

| Category | Duration | Easing | Use |
|----------|----------|--------|-----|
| Instant | 50ms | linear | Color transitions, opacity |
| Fast | 100ms | ease-out | Button states, icon swaps |
| Normal | 200ms | `cubic-bezier(0.16, 1, 0.3, 1)` | Panel slide, dropdown, tabs |
| Slow | 300ms | `cubic-bezier(0.16, 1, 0.3, 1)` | Modal, overlay, page transition |
| Spring | 250ms | `cubic-bezier(0.175, 0.885, 0.32, 1.275)` | Pop-in, reaction, toast |

### Named Animations
- `slide-in-right`: Panel peek in (200ms, ease-out)
- `slide-out-right`: Panel peek out (150ms, ease-in)
- `fade-in`: Overlay appearance (200ms)
- `scale-in`: Modal/dropdown (200ms, 0.97→1 + fade)
- `msg-slide-up`: New chat message (200ms, ease-out)
- `reaction-pop`: Emoji reaction (250ms, spring)

---

## 8. Scrollbar

Lark-style: invisible until container hover.

```css
::-webkit-scrollbar { width: 4px; height: 4px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background: transparent; border-radius: 2px; }
*:hover > ::-webkit-scrollbar-thumb { background: rgba(255,255,255,0.12); }
```

---

## 9. Accessibility

- **Focus ring** on ALL interactive elements: `0 0 0 2px [surface], 0 0 0 4px #3370FF`
- **Minimum contrast**: `gray-11` on `gray-3` = 4.8:1 (passes WCAG AA)
- **Touch targets**: minimum 32px height for buttons and nav items
- **Keyboard navigation**: Tab through all interactive, Escape closes panels/modals
- **Screen reader**: aria-labels on icon-only buttons, semantic headings

---

## 10. Do's and Don'ts

### Do
- Use 14-shade grayscale — each shade has a specific role, don't skip
- Keep max text size at 18px — this is an app, not a marketing page
- Use `gray-12` for body text, `gray-13` for headings — never pure white
- Apply focus rings on ALL interactive elements — double ring pattern
- Use side-panels (PeekPanel) over modals wherever possible
- Keep table rows at 36px — density is a feature
- Use `border-subtle` (0.05) for internal dividers, `border-default` (0.08) for containers
- Apply semantic colors ONLY for status — blue for action, not decoration
- Use shadow-as-border technique from Vercel for subtle elevation

### Don't
- Don't use `#ffffff` as text — max is `gray-14` (`#f0f2f5`), and rarely
- Don't exceed 18px font size in workspace — no display/hero typography
- Don't use weight 700 — maximum is 600, with 500 as the workhorse
- Don't introduce warm colors — palette is cool gray with blue accent only
- Don't use modals for detail views — use PeekPanel to preserve context
- Don't make table rows > 44px — defeats enterprise density
- Don't use card-grid for data lists — tables are the enterprise standard
- Don't add decorative gradients or glow effects — flat, precise, functional
- Don't use positive letter-spacing except on Overline role

---

## 11. Agent Quick Reference

### Colors (copy-paste ready)
```
Background:  #08090a / #0d1017 / #0f1115 / #141720 / #1a1e26
Hover/Active: #21252e / #282d37 / #323843
Text:        #6b7480 / #8b929e / #c0c6d0 / #e2e5eb
Accent:      #3370FF (action) / #4B83FF (hover)
Border:      rgba(255,255,255,0.05) / rgba(255,255,255,0.08) / #23252a
Success:     #22C55E    Warning: #F59E0B    Error: #EF4444
```

### Typography (copy-paste ready)
```
Font:        Inter Variable, font-feature-settings: "cv01", "ss03"
Mono:        JetBrains Mono, ui-monospace
Sizes:       18/15/14/13/12/11/10 px
Weights:     400 (read) / 500 (interact) / 600 (announce)
```

### Layout (copy-paste ready)
```
Rail:        48px,  bg #0d1017
List Panel:  240px, bg #0f1115, border-right #23252a
Content:     flex-1, bg #141720
Peek Panel:  360px, bg #141720, border-left #23252a, slide-in 200ms
```

### Component prompts
- "Sidebar nav item: 13px Inter weight 500, gray-11, padding 6px 12px, radius 6px. Active: rgba(51,112,255,0.08) bg, gray-13 text. Hover: gray-6 bg."
- "DataTable: gray-3 header, 12px/500 uppercase gray-10. Rows: 36px, border-subtle bottom. Hover: gray-6. Selected: primary-bg."
- "PeekPanel: 360px, gray-4 bg, border-left #23252a, slide-in-right 200ms. Header: 44px, 14px/600 title, close icon button."
- "Button primary: #3370FF bg, white text, 6px radius, 13px/500, padding 6px 14px. Focus: double ring 2px gap + 2px #3370FF."
