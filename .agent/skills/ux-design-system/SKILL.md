---
name: ux-design-system
description: UX/UI Design System — strict visual discipline standards for spacing, alignment, typography hierarchy, component consistency, visual balance, and self-correction. Use when building, modifying, or reviewing any UI component. This is a mandatory standard, not a guideline.
user-invocable: false
---

# UX/UI Design System — NGAC Platform

**This is a STRICT STANDARD, not a guideline. Violations make the implementation INVALID.**

Every UI change — whether a new page, a component edit, or a layout refactor — MUST comply with these rules. After every implementation, run the Self-Correction Checklist (Section 10) before reporting "done".

---

## 1. Spacing System

### 1.1 Allowed Scale

Only these spacing values are permitted:

| Token | Value | Tailwind | Usage |
|-------|-------|----------|-------|
| `xs` | 4px | `p-1`, `gap-1` | Icon-text gap, tight inline spacing |
| `sm` | 8px | `p-2`, `gap-2` | Compact component padding |
| `md` | 12px | `p-3`, `gap-3` | Minimum component padding |
| `base` | 16px | `p-4`, `gap-4` | Standard section spacing |
| `lg` | 20px | `p-5`, `gap-5` | Comfortable spacing |
| `xl` | 24px | `p-6`, `gap-6` | Major section separation |
| `2xl` | 32px | `p-8`, `gap-8` | Page-level spacing |

**Arbitrary values are FORBIDDEN.** Do not use `p-[13px]`, `gap-[18px]`, or any value not in this table.

### 1.2 Spacing Rules

```
Component internal padding  ≥ 12px  (p-3)
Section spacing             ≥ 16px  (gap-4, space-y-4)
Major section gap           ≥ 24px  (gap-6, mt-6)
Container edge padding      ≥ 12px  (px-3, py-3 minimum)
```

**No element may touch a container edge.** Every container MUST define padding.

### 1.3 Enforcement

Every component MUST explicitly set:
- **padding** — inner spacing
- **margin or gap** — outer spacing / sibling spacing
- **gap** — child element spacing (when using flex/grid)

```tsx
// ✅ Correct — explicit spacing at every level
<div className="p-4">                           {/* Container padding */}
  <div className="flex items-center gap-3">     {/* Children gap */}
    <Icon size={16} />
    <span className="text-body">Label</span>
  </div>
  <div className="mt-4">                        {/* Section spacing */}
    <p className="text-small text-gray-10">Description</p>
  </div>
</div>

// ❌ Wrong — no spacing defined
<div>
  <div>
    <Icon /><span>Label</span>
  </div>
  <div><p>Description</p></div>
</div>
```

---

## 2. Alignment System

### 2.1 Rules

- All elements MUST align to a consistent grid
- Left edges of content within a section MUST line up vertically
- Icons MUST align with text baseline using `items-center`
- Action buttons in a row MUST be the same height
- Form labels and inputs MUST align horizontally

### 2.2 Patterns

```tsx
// ✅ Aligned — icons + text + actions
<div className="flex items-center justify-between px-4 py-3">
  <div className="flex items-center gap-2">
    <Icon size={16} />
    <span className="text-body">Title</span>
  </div>
  <div className="flex items-center gap-1">
    <Button size="sm">Edit</Button>
    <Button size="sm">Delete</Button>
  </div>
</div>

// ❌ Misaligned — different icon sizes, uneven buttons
<div className="flex justify-between p-2">
  <div><Icon size={24} /><span>Title</span></div>
  <div><button className="p-1">Edit</button><button className="p-3">Delete</button></div>
</div>
```

### 2.3 Forbidden

- Misaligned buttons (different heights/padding)
- Floating elements without grid context
- Uneven spacing between siblings
- Text not vertically centered with adjacent icons

---

## 3. Typography Hierarchy

### 3.1 Required Levels

Every screen MUST use a clear hierarchy. Map to NGAC design tokens:

| Level | Utility | Size | Weight | Usage |
|-------|---------|------|--------|-------|
| **Page Title** | `text-title` | 18px | 600 | One per page, main heading |
| **Section Title** | `text-section` | 15px | 600 | Section headings |
| **Body** | `text-body` | 14px | 400 | Primary content text |
| **Body UI** | `text-body-ui` | 14px | 500 | Interactive labels |
| **Body Strong** | `text-body-strong` | 14px | 600 | Emphasized inline text |
| **Small** | `text-small` | 13px | 400 | Secondary content, descriptions |
| **Caption** | `text-caption` | 12px | 400 | Metadata, timestamps |
| **Overline** | `text-overline` | 11px | 600 | Section labels, uppercase |
| **Micro** | `text-micro` | 10px | 500 | Badges, counters |

### 3.2 Color Hierarchy

Combine with text color tokens for visual weight:

| Purpose | Color Token | Tailwind Class |
|---------|-------------|----------------|
| Primary text | `--color-text-primary` | `text-gray-13` |
| Secondary text | `--color-text-secondary` | `text-gray-11` |
| Muted/metadata | `--color-text-muted` | `text-gray-10` |
| Disabled | — | `text-gray-8` |

### 3.3 Rules

- Use **weight + size + color** together to create differentiation
- Do NOT use the same style for everything
- Page title (`text-title`) appears exactly **once** per page
- Metadata always uses a lighter color than body text

---

## 4. Component Consistency

### 4.1 Dimensions

All instances of the same component type MUST use identical dimensions:

| Component | Height | Padding | Icon Size |
|-----------|--------|---------|-----------|
| Button (sm) | 32px (`h-8`) | `px-3 py-1.5` | 14px |
| Button (md) | 36px (`h-9`) | `px-4 py-2` | 16px |
| Button (lg) | 44px (`h-11`) | `px-5 py-2.5` | 18px |
| Input | 36px (`h-9`) | `px-3 py-2` | — |
| Icon button | 32px (`h-8 w-8`) | centered | 16px |
| Nav item | 36px (`h-9`) | `px-3` | 16px |
| Avatar (sm) | 28px | — | — |
| Avatar (md) | 36px | — | — |

### 4.2 Icon Sizing

Icons MUST use consistent sizes:

| Context | Size | Usage |
|---------|------|-------|
| Inline with text | 14–16px | Buttons, nav items, labels |
| Standalone action | 16–18px | Toolbar, header actions |
| Empty state | 48–64px | Centered illustrations |

### 4.3 Rules

- Buttons in the same context MUST be the same height
- Inputs in the same form MUST have the same padding
- Icons in the same group MUST be the same size
- Random sizes across similar elements are FORBIDDEN

---

## 5. Layout Structure

### 5.1 Container Rules

Every container MUST:
- Have consistent padding (minimum `p-3`, standard `p-4` to `p-6`)
- Use `flex` or `grid` for layout — no arbitrary positioning
- Define explicit `gap` between children

### 5.2 Panel Separation

- Use **subtle borders** (`border-border`) OR **spacing** (`gap-4`) to separate panels
- Never rely on color alone for panel distinction
- Adjacent panels with the same background MUST have a visible separator

```tsx
// ✅ Clear separation
<div className="flex h-full">
  <aside className="w-60 border-r border-border-solid">...</aside>
  <main className="flex-1">...</main>
</div>

// ❌ No separation
<div className="flex h-full">
  <aside className="w-60">...</aside>     {/* same bg, no border */}
  <main className="flex-1">...</main>
</div>
```

---

## 6. Visual Balance

### 6.1 Every screen MUST pass these checks:

- No **crowded areas** — too many elements packed together
- No **awkward empty space** — large gaps with no purpose
- **Even distribution** of visual weight across the viewport

### 6.2 Fix by:

- Adjusting spacing between sections (use the spacing scale)
- Grouping related elements closer together
- Aligning sections to consistent left/right margins
- Using consistent vertical rhythm

---

## 7. Empty State Design

### 7.1 Every empty state MUST include:

```tsx
<div className="flex flex-col items-center justify-center py-16 text-center">
  <Icon size={48} className="text-gray-8 mb-4" />
  <h3 className="text-body-strong text-gray-12 mb-1">No items yet</h3>
  <p className="text-small text-gray-10 mb-4 max-w-xs">
    Create your first item to get started.
  </p>
  <Button>Create Item</Button>
</div>
```

Requirements:
- Centered icon or illustration (48–64px)
- Clear, descriptive message (not just "Empty")
- Proper spacing (`py-16` or equivalent)
- Optional CTA button
- Must feel **intentional**, not like a placeholder

---

## 8. Interaction Consistency

### 8.1 State Feedback

| State | Visual Treatment |
|-------|-----------------|
| **Default** | Base styles |
| **Hover** | `hover:bg-gray-6` or `hover:bg-bg-hover` — subtle background shift |
| **Active/Pressed** | `active:scale-[0.98]` or `bg-accent-bg` — distinct feedback |
| **Focus** | `focus-ring` utility — 2px accent outline |
| **Disabled** | `opacity-50 cursor-not-allowed` |
| **Selected** | `bg-accent-bg border-accent` or `text-accent` |

### 8.2 Mobile Rules

- No **hover-only** actions — every action MUST have a tap/click equivalent
- Touch targets MUST be ≥ 44px (`min-h-11 min-w-11`)
- Context menus MUST work via long-press or visible button on mobile

---

## 9. Color & Depth

### 9.1 Surface Layers

| Layer | Token | Usage |
|-------|-------|-------|
| App background | `bg-gray-1` | Root body |
| Panel background | `bg-gray-3`, `bg-gray-4` | Sidebars, main content |
| Surface (elevated) | `bg-gray-5`, `bg-gray-6` | Cards, popovers |
| Hover surface | `bg-gray-6` | Interactive hover |
| Modal overlay | `bg-black/50` | Backdrop |

### 9.2 Rules

- Maintain **contrast** between adjacent layers (minimum 2 shades apart)
- Avoid flat, indistinguishable surfaces
- Elevated elements (modals, dropdowns) MUST use `shadow-md` or higher

---

## 10. Self-Correction Checklist (MANDATORY)

**Run this checklist BEFORE reporting any UI work as "done".**

### Pre-completion Verification

```
□ SPACING — Is every container padded? Are gaps consistent?
    → No element touches a container edge
    → All values from the spacing scale (4/8/12/16/20/24/32px)
    → Sibling elements have equal spacing

□ ALIGNMENT — Are all left edges aligned? Icons centered with text?
    → Buttons in a row are the same height
    → Form labels and inputs align horizontally
    → No floating/misaligned elements

□ HIERARCHY — Is typography differentiated?
    → Page has exactly one title
    → Body text differs from metadata
    → Weight + size + color all contribute

□ CONSISTENCY — Are same-type components identical?
    → All buttons same height within context
    → All icons same size within context
    → Input fields have consistent padding

□ BALANCE — Does the screen feel even?
    → No crowded sections
    → No unexplained empty areas
    → Visual weight distributed evenly

□ INTERACTION — Do all states have feedback?
    → Hover shows subtle change
    → Active shows clear state
    → Disabled is visually distinct
    → Mobile: no hover-only actions

□ EMPTY STATES — Are they designed?
    → Centered icon + message
    → Proper spacing
    → Feels intentional
```

### Self-Correction Loop

After checking the above:

1. **Review** — Visually scan the entire viewport
2. **Identify** — Find any imbalance, misalignment, or inconsistency
3. **Fix** — Adjust spacing, alignment, or sizing
4. **Re-check** — Verify the fix doesn't break other areas

**Repeat until the UI feels "clean and balanced".**

---

## 11. Anti-Patterns (FORBIDDEN)

| Anti-Pattern | Fix |
|-------------|-----|
| Random spacing values (`p-[13px]`) | Use spacing scale (`p-3`) |
| Elements touching container edges | Add container padding (`p-3` minimum) |
| Misaligned content (uneven left edges) | Use consistent `px-*` on siblings |
| Overcrowded sections | Add `gap-4` or `space-y-4` between items |
| Excessive empty space | Reduce padding or center content |
| Inconsistent component sizing | Standardize via component props |
| Same typography style for everything | Use hierarchy levels |
| Color-only panel separation | Add `border-r border-border` |
| Hover-only interactions on mobile | Add visible tap targets |
| Layout shift on content load | Use skeleton placeholders or fixed dimensions |

---

## 12. Definition of Done

A UI is COMPLETE only when:

- ✅ All spacing follows the 4/8/12/16/20/24/32px scale
- ✅ All elements are aligned (left edges, icons, buttons)
- ✅ Typography hierarchy is clear (title → body → metadata)
- ✅ Same-type components are visually identical
- ✅ Screen is visually balanced (no crowding, no awkward gaps)
- ✅ Interaction states provide clear feedback
- ✅ Empty states are designed intentionally
- ✅ Self-correction checklist passes with all items checked

**"It works" is NOT sufficient. It must look clean, balanced, and intentional.**
