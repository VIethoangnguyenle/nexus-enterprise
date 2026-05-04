# UX/UI Standardization — Platform-Wide Visual Discipline

## What

Apply the newly created **UX Design System** skill (`ux-design-system`) across all frontend modules to enforce consistent spacing, alignment, typography hierarchy, component sizing, and visual balance. This is a code-level refactor — no new features, only visual quality improvements.

## Why

After the responsive-first refactor, the platform is **functionally correct** across breakpoints but **visually inconsistent**. An audit reveals:

- **~40 files** use raw Tailwind text sizes (`text-xs`, `text-sm`, `text-2xl`) instead of design token utilities (`text-body`, `text-caption`, `text-section`)
- **~10 files** use arbitrary font sizes (`text-[0.65rem]`, `text-[0.7rem]`) bypassing the typography scale
- **Inline styles** remain in documents table, DataTable, and DriveFileRow for heights that should be standardized
- **Spacing inconsistencies** — some components use `p-2`, others `p-4`, adjacent sections use different padding values without visual rationale
- **Settings page** is a placeholder with no proper empty state design
- **Component sizing** is inconsistent (filter buttons use arbitrary `px-2.5 py-1 text-xs` instead of standard button primitives)

## Scope

### In scope
- Replace all raw Tailwind typography classes with design token utilities across all `.tsx` files
- Eliminate arbitrary `text-[*]` values — use the typography scale
- Standardize component padding/heights to the spacing scale (4/8/12/16/20/24/32px)
- Remove inline `style={{ height: 36 }}` in favor of Tailwind classes
- Fix empty states (Settings) to follow the EmptyState component pattern
- Ensure every section header uses consistent typography hierarchy
- Apply self-correction checklist to each module

### Out of scope
- New features or functionality
- Backend changes
- Responsive layout changes (already completed)
- Design token additions (existing tokens are sufficient)

## Success criteria
- Zero `text-[arbitrary]` values in `.tsx` files
- All typography uses design token utilities (`text-title`, `text-body`, etc.)
- All spacing values from the 4/8/12/16/20/24/32px scale
- Self-correction checklist passes for every screen
- Production build passes (0 errors)
