---
name: tailwindcss
description: Tailwind CSS v4 styling patterns for NGAC Platform — CSS-first configuration, design tokens via @theme, custom @utility directives, responsive design with mobile-first breakpoints, and dark mode. Use when styling components, adding responsive behavior, or configuring the design system.
user-invocable: false
---

# Tailwind CSS v4 — NGAC Platform

This project uses **Tailwind CSS v4** with CSS-first configuration. All design tokens are defined in `@theme {}` blocks in `frontend/src/index.css`.

## Project Setup

```
frontend/
├── src/
│   ├── index.css          ← @import "tailwindcss" + @theme + @utility
│   └── ...
├── package.json           ← tailwindcss: ^4.2.4, @tailwindcss/vite
└── vite.config.js         ← uses @tailwindcss/vite plugin
```

**No `tailwind.config.js` needed** — Tailwind v4 uses CSS-first configuration.

## Design Token System (`@theme`)

All tokens are defined in `index.css` inside `@theme {}`:

```css
@import "tailwindcss";

@theme {
  /* Grayscale (14 shades, cool blue undertone) */
  --color-gray-1: #08090a;
  --color-gray-4: #141720;   /* bg-primary */
  --color-gray-13: #e2e5eb;  /* text-primary */

  /* Semantic aliases */
  --color-bg-primary: var(--color-gray-4);
  --color-accent: #3370ff;
  --color-danger: #EF4444;

  /* Border radius */
  --radius-sm: 4px;
  --radius-md: 6px;

  /* Shadows */
  --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.3);

  /* Motion */
  --duration-fast: 100ms;
  --ease-out: cubic-bezier(0.16, 1, 0.3, 1);

  /* Animations */
  --animate-fade-in: fade-in var(--duration-normal) var(--ease-out);
}
```

These generate Tailwind classes: `bg-gray-4`, `text-accent`, `rounded-sm`, `shadow-sm`, `animate-fade-in`, etc.

## Custom Typography Utilities (`@utility`)

```css
@utility text-title {
  font-size: 18px; font-weight: 600; line-height: 1.35; letter-spacing: -0.3px;
}
@utility text-body {
  font-size: 14px; font-weight: 400; line-height: 1.50;
}
@utility text-small {
  font-size: 13px; font-weight: 400; line-height: 1.45;
}
@utility text-caption {
  font-size: 12px; font-weight: 400; line-height: 1.40;
}
```

Use: `<h1 className="text-title text-gray-13">Title</h1>`

## Responsive Design (Mobile-First)

### Breakpoints

| Prefix | Min-width | Use |
|--------|-----------|-----|
| *(base)* | 0px | Mobile styles (write these first) |
| `md:` | 768px | Tablet |
| `lg:` | 1024px | Desktop |
| `xl:` | 1280px | Large screens |

### Pattern

```tsx
// ✅ Mobile-first: base = mobile, add complexity up
<div className="flex flex-col md:flex-row lg:gap-6">
<div className="p-3 md:p-4 lg:p-6">
<div className="hidden lg:flex">  {/* Show only on desktop */}
<div className="lg:hidden">       {/* Show only on mobile/tablet */}

// ❌ Desktop-first (WRONG for this project)
<div className="flex-row md:flex-col">
```

### Layout Shells

```tsx
// App shell: mobile bottom nav + desktop sidebar
<div className="flex flex-col lg:flex-row h-dvh">
  <Sidebar className="hidden lg:flex lg:w-40" />
  <MobileNav className="fixed bottom-0 inset-x-0 h-14 lg:hidden" />
  <main className="flex-1 min-w-0 pb-14 lg:pb-0">
    <Outlet />
  </main>
</div>
```

### Data Views

```tsx
// Mobile: cards, Desktop: table
<div className="hidden lg:block">
  <table>...</table>
</div>
<div className="lg:hidden space-y-2">
  {items.map(item => <Card key={item.id} />)}
</div>
```

## Dark Mode

The NGAC platform is **dark-first** — no `dark:` prefix needed. All tokens are already dark values. If light mode is ever added, use `dark:` prefix on light-mode base styles.

## Component Styling Rules

1. **Use Tailwind utilities for layout**: `flex`, `grid`, `p-*`, `m-*`, `gap-*`
2. **Use tokens for colors**: `bg-gray-4`, `text-accent`, `border-border`
3. **Use `@utility` for typography**: `text-title`, `text-body`, `text-small`
4. **Use responsive prefixes**: `md:`, `lg:` for adaptive layout
5. **Keep custom CSS minimal**: Only for complex component styles (sidebar, chat header)
6. **No `!important`**: Refactor specificity instead
7. **No arbitrary color values**: Use existing `@theme` tokens

## Common Patterns

### Button

```tsx
<button className="px-3 py-2 bg-accent text-white rounded-md text-small
  hover:bg-accent-hover transition-colors duration-fast
  disabled:opacity-50 disabled:cursor-not-allowed
  min-h-11 min-w-11">
  Submit
</button>
```

### Card

```tsx
<div className="bg-gray-3 border border-border rounded-lg p-4 hover:bg-gray-4 
  transition-colors cursor-pointer">
  <h3 className="text-body-strong text-gray-13 truncate">{title}</h3>
  <p className="text-small text-gray-10 mt-1 line-clamp-2">{desc}</p>
</div>
```

### Input

```tsx
<input className="w-full px-3 py-2 bg-gray-3 border border-border rounded-md 
  text-body text-gray-13 placeholder:text-gray-8
  focus:border-accent focus:outline-none transition-colors" />
```

### Responsive Container

```tsx
<div className="px-3 md:px-4 lg:px-6 py-3 md:py-4">
  {/* Content with responsive padding */}
</div>
```

## Anti-Patterns

```tsx
// ❌ Inline styles
<div style={{ padding: '16px' }}>

// ❌ Arbitrary values for existing tokens
<div className="bg-[#3370ff]">     // use bg-accent

// ❌ Desktop-first responsive
<div className="flex-row md:flex-col">

// ❌ !important for responsive
.sidebar { width: 100% !important; }

// ❌ max-width media queries in CSS
@media (max-width: 768px) { ... }  // use Tailwind md: prefix
```
