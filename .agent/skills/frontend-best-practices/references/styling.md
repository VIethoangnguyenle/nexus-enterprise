# CSS & Styling

## Tech Stack
- **Tailwind CSS v4** — Primary styling via utility classes in JSX
- **CSS-first config** — Design tokens via `@theme {}` in `index.css`
- **Custom utilities** — `@utility` directive for typography roles
- Dark-first palette (enterprise workspace aesthetic)

## Tailwind v4 Config (CSS-first)

All design tokens live in `@theme {}` block in `index.css`:

```css
@import "tailwindcss";

@theme {
  --color-gray-1: #08090a;
  --color-accent: #3370ff;
  --font-sans: 'Inter Variable', ...;
  /* ... */
}
```

This generates Tailwind classes automatically: `bg-gray-1`, `text-accent`, `font-sans`, etc.

## Rules

1. **Use Tailwind utility classes in JSX** — not inline styles, not custom CSS classes for simple layout
2. **Responsive: mobile-first** with breakpoint prefixes: `md:`, `lg:`, `xl:`
3. **Design tokens** via `@theme` in `index.css` — don't use arbitrary values for colors/spacing that exist as tokens
4. **Custom `@utility`** for complex typography roles: `text-title`, `text-body`, `text-small`, etc.
5. **Animations** via `@keyframes` + `animate-*` utilities in CSS
6. **Dark mode**: already default — maintain dark-first palette, no `dark:` prefix needed
7. **Component CSS** (`.lark-sidebar`, `.chat-header`, etc.) — only for complex multi-property styles that can't be utilities
8. **`className` conditional**: `` `card ${active ? 'active' : ''}` ``

## When to use custom CSS vs Tailwind

| Use Tailwind utilities | Use custom CSS in `index.css` |
|----------------------|-------------------------------|
| Layout (flex, grid, padding) | Complex component styles (sidebar, chat header) |
| Responsive breakpoints (`md:`, `lg:`) | `@keyframes` animations |
| Colors, typography, spacing | Scrollbar styles |
| Hover/focus states | TipTap editor styles |
| Simple borders, shadows | `@utility` typography roles |

## Anti-patterns

```tsx
// ❌ Inline styles for layout
<div style={{ padding: '16px', display: 'flex' }}>

// ✅ Tailwind utilities
<div className="p-4 flex">

// ❌ Custom CSS for simple responsive
.my-container { @media (max-width: 768px) { ... } }

// ✅ Tailwind responsive in JSX
<div className="flex-col md:flex-row">

// ❌ Arbitrary values for existing tokens
<div className="bg-[#3370ff]">

// ✅ Use token
<div className="bg-accent">
```
