## 1. Color Token Foundation

- [x] 1.1 Define `--gray-1` through `--gray-14` CSS custom properties in `frontend/src/index.css` `@theme` block
- [x] 1.2 Define functional color pairs (primary, success, warning, error, info) with foreground + 8% bg variants
- [x] 1.3 Define border tokens: `--border-subtle`, `--border-default`, `--border-strong`, `--border-solid`
- [x] 1.4 Remap existing semantic aliases (`--color-bg-primary`, `--color-bg-secondary`, etc.) to point to new `--gray-N` values for backward compatibility
- [x] 1.5 Set `font-feature-settings: "cv01", "ss03"` on root element

## 2. Typography Utility Classes

- [x] 2.1 Define `@utility text-title` (18px/600/-0.3px)
- [x] 2.2 Define `@utility text-section` (15px/600/-0.15px)
- [x] 2.3 Define `@utility text-body`, `text-body-ui`, `text-body-strong` (14px at 400/500/600)
- [x] 2.4 Define `@utility text-small`, `text-small-ui` (13px at 400/500)
- [x] 2.5 Define `@utility text-caption`, `text-caption-ui` (12px at 400/500)
- [x] 2.6 Define `@utility text-overline` (11px/600/uppercase/0.5px tracking)
- [x] 2.7 Define `@utility text-micro` (10px/500/0.3px tracking)

## 3. Motion & Depth Tokens

- [x] 3.1 Define duration custom properties: `--duration-instant` (50ms), `--duration-fast` (100ms), `--duration-normal` (200ms), `--duration-slow` (300ms)
- [x] 3.2 Define easing custom properties: `--ease-out`, `--ease-in`, `--ease-in-out`, `--ease-spring`
- [x] 3.3 Define depth/elevation utility classes or shadow custom properties for 6 named levels (Recessed → Overlay)
- [x] 3.4 Update existing `@keyframes` animations to use new duration/easing tokens

## 4. Focus Accessibility

- [x] 4.1 Define `.focus-ring` utility class with double-ring `box-shadow` on `:focus-visible`
- [x] 4.2 Apply focus-ring to Button component (`components/primitives/Button.tsx`)
- [x] 4.3 Apply focus-ring to Input, Select, Textarea components
- [x] 4.4 Apply focus-ring to IconButton component
- [x] 4.5 Apply focus-ring to Sidebar navigation items and links

## 5. Primitive Component Updates

- [x] 5.1 Update `Button.tsx` — new token names for variant styles, add loading state indicator
- [x] 5.2 Update `Input.tsx` — recessed `gray-1` bg, `border-default`, focus ring
- [x] 5.3 Update `Select.tsx` — same recessed pattern as Input
- [x] 5.4 Update `Textarea.tsx` — same recessed pattern as Input
- [x] 5.5 Update `Badge.tsx` — use functional color pairs (semantic bg + fg)
- [x] 5.6 Update `Avatar.tsx` — ensure contrast on new surface colors
- [x] 5.7 Update `Spinner.tsx` — accent color token reference
- [x] 5.8 Update `Heading.tsx` — map to `text-title` / `text-section` roles
- [x] 5.9 Update `Text.tsx` — map variants to new typography roles
- [x] 5.10 Update primitives `index.ts` barrel export (if needed)

## 6. Composite Component Updates

- [x] 6.1 Update `DataTable.tsx` — `text-caption-ui` headers, `gray-3` header bg, 36px row height, `border-subtle` dividers, `gray-6` hover, `primary-bg` selected
- [x] 6.2 Update `PeekPanel.tsx` — `gray-4` bg, `border-solid` left border, slide animation with `--duration-normal` + `--ease-out`
- [x] 6.3 Update `Modal.tsx` — `gray-5` bg, `border-default` border, overlay shadow, `text-section` title
- [x] 6.4 Update `Card.tsx` — `gray-5` bg, `border-default` border
- [x] 6.5 Update `Tabs.tsx` — `text-small-ui` tab labels, accent bottom indicator

## 7. Pattern & Layout Updates

- [x] 7.1 Update `Sidebar.tsx` — `gray-3` bg, nav items to `text-small-ui` + `gray-11`, active to `primary-bg` + `gray-13`, search input recessed
- [x] 7.2 Update `_workspace.tsx` layout — topbar to `gray-3` bg, content area to `gray-4`
- [x] 7.3 Update `channels.$channelId.tsx` — message typography to spec roles, hover to `gray-6`
- [x] 7.4 Update `drive.tsx` — toolbar/breadcrumb token updates
- [x] 7.5 Update `documents.tsx` — table header/row token updates

## 8. Verification

- [ ] 8.1 Grep all frontend files for old token names — ensure no orphaned references
- [ ] 8.2 Open app in browser, navigate all modules (Messaging, Drive, Documents, Settings), verify visual consistency
- [ ] 8.3 Tab through all interactive elements to verify focus rings appear correctly
- [ ] 8.4 Check sidebar collapse/expand animation uses new timing
- [ ] 8.5 Verify no Tailwind build errors or missing utility warnings
