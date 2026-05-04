# Design System: Nexus Hub

**Project ID:** 14852434379132121789

## 1. Visual Theme & Atmosphere

A sophisticated, high-density enterprise workspace platform with a "Daily App Balanced" density (6/10). The atmosphere is professional and trustworthy — clean surfaces with subtle blue-tinted depth. Inspired by Lark/Feishu design language with Material Design 3 color tokens. The interface prioritizes information density while maintaining generous breathing room through careful whitespace management.

## 2. Color Palette & Roles

### Core Surfaces

- **Pure White** (#FFFFFF / `surface-container-lowest`) — Primary card fills, panel backgrounds, content containers
- **Airy Blue Tint** (#F9F9FF / `background`, `surface`) — Page-level background, adds subtle warmth
- **Frosted Blue** (#F0F3FF / `surface-container-low`) — Table headers, search input backgrounds, secondary surfaces
- **Soft Blue Mist** (#E7EEFF / `surface-container`) — Active sidebar items, subtle hover states
- **Pale Periwinkle** (#DEE8FF / `surface-container-high`) — Elevated surface areas
- **Lavender Steel** (#D8E3FB / `surface-container-highest`, `surface-variant`) — Strong surface contrast

### Text Hierarchy

- **Deep Navy Ink** (#111C2D / `on-surface`, `on-background`) — Primary text, headings, maximum contrast
- **Slate Charcoal** (#434655 / `on-surface-variant`) — Secondary text, descriptions, metadata
- **Cool Gray** (#737686 / `outline`) — Tertiary text, labels, department counts
- **Soft Boundary** (#C3C6D7 / `outline-variant`) — Borders, dividers, structural lines

### Brand & Accent

- **Royal Blue** (#004AC6 / `primary`) — Primary buttons, active navigation, brand anchor
- **Bright Blue** (#2563EB / `primary-container`) — Primary container fills
- **Whisper Blue** (#DBE1FF / `primary-fixed`) — Active item backgrounds in navigation trees
- **Light Lavender** (#B4C5FF / `primary-fixed-dim`) — Hover states on primary items
- **Deep Navy** (#003EA8 / `on-primary-fixed-variant`) — Active text on primary-fixed backgrounds

### Secondary & Tertiary

- **Storm Blue** (#505F76 / `secondary`) — Secondary text elements
- **Sky Wash** (#D0E1FB / `secondary-container`) — Secondary container fills, fallback avatars
- **Warm Slate** (#515659 / `tertiary`) — Tertiary text, subtle labels

### Status Colors (Semantic — NOT from design tokens)

- **Online Green** — `bg-green-50 text-green-700 border-green-200` (badge), `bg-green-500` (dot)
- **Offline Gray** — `bg-slate-100 text-slate-600 border-slate-200` (badge), `bg-slate-300` (dot)
- **In Meeting Orange** — `bg-orange-50 text-orange-700 border-orange-200` (badge), `bg-orange-400` (dot)

### Error

- **Signal Red** (#BA1A1A / `error`) — Error states, destructive actions

## 3. Typography Rules

- **Font Family:** Manrope — used across ALL text elements (display, body, buttons, labels)
- **Display/H1:** 32px/40px, letter-spacing -0.02em, weight 700 — Page titles, hero sections
- **Heading/H2:** 24px/32px, letter-spacing -0.01em, weight 600 — Section headers, panel titles
- **Subheading/H3:** 20px/28px, weight 600 — Card titles, subsection headers
- **Body Large:** 16px/24px, weight 400 — Primary body text
- **Body Medium:** 14px/20px, weight 400 — Default body, table cells, descriptions
- **Body Small:** 13px/18px, weight 400 — Search inputs, compact metadata
- **Button:** 14px/20px, weight 600 — All buttons, action text
- **Label Caps:** 11px/16px, letter-spacing 0.05em, weight 700 — Table headers, uppercase labels, detail section headers

## 4. Component Stylings

### Buttons

- **Primary:** `bg-primary text-on-primary py-2 px-4 rounded-lg` — Solid royal blue, white text, gently rounded (8px)
- **Secondary/Outlined:** `bg-surface-container-lowest border border-outline-variant text-on-surface px-4 py-2 rounded-lg` — Ghost button with border
- **Icon Button:** `p-2 rounded-lg border border-outline-variant` — Square-ish icon actions
- **Full-Width CTA:** `py-3 rounded-xl` — Used in profile panel actions, larger corner radius (12px)

### Avatars

- **Table Row:** `w-10 h-10 rounded-full` (40×40px) — With `border border-outline-variant/20`
- **Profile Panel:** `w-24 h-24 rounded-full border-4 border-white shadow-md` (96×96px)
- **Fallback (Initials):** `bg-secondary-container text-on-secondary-container font-medium text-sm`
- **Status Dot (small):** `w-3 h-3 rounded-full border-2 border-white` — Positioned absolute bottom-right
- **Status Dot (large):** `w-5 h-5 rounded-full border-2 border-white` — Profile panel

### Cards & Containers

- **Table Container:** `bg-surface-container-lowest rounded-xl shadow-[0_4px_16px_rgba(0,0,0,0.04)] border border-outline-variant` — Whisper-soft shadow, generous 12px corners
- **Table Header:** `bg-surface-container-low border-b border-outline-variant` — Tinted header strip
- **Detail Info Container:** `bg-surface-container-low border-y border-outline-variant/30` — For grouped info rows

### Status Badges

- **Shape:** Inline-flex, `px-2 py-0.5 rounded text-xs font-medium` — Pill-ish with subtle border
- **States:** Online (green-50/700/200), Offline (slate-100/600/200), Busy (orange-50/700/200)

### Navigation Sidebar

- **Width:** 256px (`w-64`)
- **Active Group Header:** `bg-surface-container text-primary font-medium`
- **Active Sub-item:** `bg-primary-fixed text-on-primary-fixed-variant` with filled icon
- **Inactive Sub-item:** `text-on-surface-variant hover:bg-surface-container-low`
- **Indent:** `pl-9 pr-2` for sub-items under group headers

### Inputs

- **Search:** `bg-surface-container-low border border-outline-variant rounded-full` — Pill-shaped
- **Focus:** `focus:border-primary focus:ring-2 focus:ring-primary/10`

## 5. Layout Principles

### Grid Architecture

- **3-Column Desktop:** Global Sidebar (72px) + Context Sidebar (256px) + Main Content (flex-1)
- **Profile Panel:** Fixed right-side panel, `w-96` (384px), `fixed right-0 top-14 bottom-0`, shadow-xl
- **Main Content Header:** `px-8 py-6 border-b border-outline-variant bg-surface-container-lowest shadow-sm`
- **Content Area:** `p-8 bg-background` — Generous 32px padding
- **Table Grid Template:** `grid-cols-[auto_1.5fr_1fr_1.5fr_1fr]` — Avatar + Name + Role + Email + Status

### Spacing Scale (4px unit base)

- **xs:** 4px (`space-xs`) — Micro gaps, icon margins
- **sm:** 8px (`space-sm`) — Between related items
- **md:** 16px (`space-md`) — Standard component padding
- **lg:** 24px (`space-lg`, `gutter`) — Section spacing
- **margin:** 32px — Page content margins
- **xl:** 40px (`space-xl`) — Major section separations

### Responsive Strategy

- **Mobile (<768px):** Single column, sidebar as overlay/drawer
- **Tablet (768–1024px):** Collapsible sidebar, simplified columns
- **Desktop (1024px+):** Full 3-column layout with all panels

## 6. Design System Notes for Stitch Generation

When generating new screens for Nexus Hub, use this block:

**DESIGN SYSTEM (REQUIRED):**

- Platform: Web, Desktop-first
- Theme: Light, professional, enterprise-density Lark-inspired
- Font: Manrope (400/500/600/700)
- Primary: Royal Blue (#004AC6) for actions, Deep Navy (#003EA8) for active text
- Background: Airy Blue Tint (#F9F9FF), cards on Pure White (#FFFFFF)
- Text: Deep Navy Ink (#111C2D) primary, Slate Charcoal (#434655) secondary
- Borders: Soft Boundary (#C3C6D7), 1px structural lines
- Buttons: Gently rounded (8px), solid fill primary, ghost outline secondary
- Cards: Generously rounded (12px), whisper-soft shadow `0_4px_16px_rgba(0,0,0,0.04)`
- Density: Balanced (6/10), information-rich but breathable
- Icons: Material Symbols Outlined, 20px default
