# Responsive Design Patterns

## Core Principle

**Mobile-first**: Write base styles for mobile, add complexity with breakpoint prefixes.

```tsx
// ✅ Mobile-first (correct)
<div className="flex-col md:flex-row lg:gap-6">

// ❌ Desktop-first (wrong)
<div className="flex-row md:flex-col">  // don't undo desktop styles for mobile
```

## Breakpoints

| Prefix | Min-width | Use case |
|--------|-----------|----------|
| *(none)* | 0px | Mobile default — always write this first |
| `sm:` | 640px | Large phones (rarely used) |
| `md:` | 768px | Tablet portrait |
| `lg:` | 1024px | Laptop / tablet landscape |
| `xl:` | 1280px | Desktop |
| `2xl:` | 1536px | Large monitor |

**Primary breakpoints**: `md:` and `lg:`. Use `sm:` and `xl:` only when needed.

## Layout Patterns

### App Shell — Workspace

```
MOBILE (< 768px)              TABLET (768-1024px)          DESKTOP (≥ 1024px)
┌──────────────────┐          ┌──────┬───────────┐         ┌────┬──────┬──────────┐
│                  │          │ List │           │         │Rail│ List │          │
│   Content Area   │          │Panel │  Content  │         │    │Panel │ Content  │
│  (full screen)   │          │      │           │         │ 56 │240px │  flex-1  │
│                  │          │ 240  │  flex-1   │         │ px │      │          │
│                  │          │  px  │           │         │    │      │          │
├──────────────────┤          └──────┴───────────┘         └────┴──────┴──────────┘
│ ☰  📄  💬  📁  👤│
└──────────────────┘

Implementation:
- Rail: hidden lg:flex lg:flex-col lg:w-14
- ListPanel: fixed inset-0 z-40 lg:relative lg:w-60 lg:z-auto
- Content: flex-1 min-w-0 pb-14 lg:pb-0
- MobileNav: fixed bottom-0 inset-x-0 h-14 lg:hidden
```

### Data Views — Table vs Cards

```tsx
// Mobile: card list | Desktop: table
<div className="hidden lg:block">
  <DataTable columns={fullColumns} data={items} />
</div>
<div className="lg:hidden space-y-2 p-3">
  {items.map(item => <ItemCard key={item.id} item={item} />)}
</div>
```

### Side Panels — Peek Panel

```tsx
// Mobile: full-screen overlay | Desktop: side panel
<div className={`
  fixed inset-0 z-50
  lg:relative lg:inset-auto lg:z-auto lg:w-80 lg:border-l lg:border-border
`}>
  <PanelContent />
</div>
```

### Modals

```tsx
// Mobile: full-screen | Desktop: centered dialog
<div className="fixed inset-0 md:inset-auto md:top-1/2 md:left-1/2 
  md:-translate-x-1/2 md:-translate-y-1/2 md:max-w-lg md:rounded-lg">
```

## Per-Module Responsive Patterns

### Chat / Messaging
- Mobile: Full-screen chat, back button to channel list
- Tablet: Split view (list + chat)
- Desktop: 3-column (rail + list + chat + optional thread panel)

### Drive
- Mobile: Card list (filename, size, date only), no sidebar
- Tablet: Simplified table (fewer columns), collapsed sidebar
- Desktop: Full table + sidebar + context panel

### Assets
- Mobile: Stacked dashboard cards, simplified list view
- Tablet: 2-column dashboard, reduced table
- Desktop: Full dashboard grid + data table

### Auth
- Mobile: Form only (no illustration)
- Desktop: Split layout (illustration + form)

## Component Responsive Checklist

For every component, verify:

- [ ] Renders correctly at 375px width
- [ ] No horizontal overflow
- [ ] Touch targets ≥ 44px (`min-h-11`)
- [ ] Text truncates properly (no wrapping that breaks layout)
- [ ] No content hidden without alternative access
- [ ] No hover-only interactions

## Spacing Scale

Reduce spacing on smaller screens:

```tsx
// Section spacing
<div className="py-3 md:py-4 lg:py-6">

// Container padding
<div className="px-3 md:px-4 lg:px-6">

// Gap between items
<div className="gap-2 md:gap-3 lg:gap-4">
```

## Typography Scale

```tsx
// Page title
<h1 className="text-lg md:text-xl lg:text-2xl font-bold">

// Section heading
<h2 className="text-base md:text-lg font-semibold">

// Body text stays consistent (14px is readable everywhere)
<p className="text-body">
```

## Navigation Patterns

### Bottom Nav (Mobile)

```tsx
<nav className="fixed bottom-0 inset-x-0 h-14 bg-gray-2 border-t border-border
  flex items-center justify-around lg:hidden">
  {navItems.map(item => (
    <button key={item.id} className="flex flex-col items-center gap-1 
      min-h-11 min-w-11 justify-center">
      <Icon size={20} />
      <span className="text-[10px]">{item.label}</span>
    </button>
  ))}
</nav>
```

### Hamburger Menu (Mobile)

```tsx
<button
  className="lg:hidden min-h-11 min-w-11 flex items-center justify-center"
  onClick={() => setMenuOpen(!menuOpen)}
>
  <MenuIcon size={20} />
</button>
```

## Testing

Before marking a frontend task complete:

1. Open DevTools → Toggle device toolbar
2. Test at **375px** (iPhone SE)
3. Test at **768px** (iPad portrait)
4. Test at **1280px** (laptop)
5. Test at **1920px** (monitor)

Checklist per breakpoint:
- No horizontal scrollbar
- No overlapping elements
- No text cutoff without ellipsis
- All interactive elements reachable
- Navigation accessible
