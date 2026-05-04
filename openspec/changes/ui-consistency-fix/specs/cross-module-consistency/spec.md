# Cross-Module Consistency Spec

## Stitch Reference
- Design System: "Nexus Hub" — Serene Enterprise theme
- Design tokens from Stitch designMd (Manrope font, M3 color tokens)

## Cross-Module Patterns (from Stitch)

### Sidebar Section Labels
All sidebars MUST use identical `label-caps` pattern:
- Font: Manrope 11px, font-weight 700
- Transform: uppercase
- Letter-spacing: 0.05em
- Color: `on-surface-variant`

**Modules**: Chat ("PINNED", "DEPARTMENTS", "DIRECT MESSAGES"), Drive ("DEPARTMENTS"), Approval (no sidebar sections)

### Page Headers
From Stitch designs, each module header follows:
- Module name as page title (h2 style: Manrope 24px, font-weight 600)
- Subtitle/description below in body-sm
- Action buttons right-aligned in header row

**Pattern**: `<Heading level={2}>` primitive for all module page titles

### Action Buttons
Primary actions MUST use `<Button>` primitive:
- **Approval**: "✚ New Request" — variant="primary"
- **Chat**: "✚ New Project" (or "✚ New Message") — variant="primary"
- **Drive**: "⬆ Upload File" — variant="primary"
- Secondary actions (New Folder, filters) — variant="outline" or `<IconButton>`

### Navigation Sidebar Items
All sidebar nav items follow consistent pattern:
- Icon (16-20px) + label text
- Active state: primary-color background pill/highlight
- Hover state: surface-container background

---

## User Stories

### US-6: All modules use consistent sidebar section label styling
As a platform user,
I want all module sidebars to use the same section label typography,
so that the platform feels unified.

**Acceptance Criteria:**
- [ ] Drive sidebar section labels use `label-caps` class/style
- [ ] Chat sidebar section labels use `label-caps` class/style
- [ ] Both modules use identical font-size, weight, letter-spacing, text-transform
- [ ] Visual comparison confirms parity at 1280px

**Proto mapping:** Frontend-only
**Files affected:** DriveSidebar.tsx, workspace chat layout

### US-7: All modules use Button/IconButton primitives for actions
As a platform user,
I want all action buttons across all modules to use the same Button/IconButton components,
so that buttons look and behave consistently.

**Acceptance Criteria:**
- [ ] Zero raw `<button>` elements used for user-facing actions in approval, chat, drive modules
- [ ] All primary actions use `<Button variant="primary">`
- [ ] All icon-only actions use `<IconButton>`
- [ ] Tree expand/collapse chevrons may remain as raw buttons (structural, not user-facing actions)
- [ ] Build passes with no regressions

**Proto mapping:** Frontend-only
**Files affected:** All drive components, approval toolbar, chat toolbar

### US-8: All modules use Heading primitive for page titles
As a platform user,
I want all module page titles to use the `<Heading>` primitive,
so that typography hierarchy is consistent.

**Acceptance Criteria:**
- [ ] Drive page title uses `<Heading level={2}>`
- [ ] Approval page title uses `<Heading level={2}>`
- [ ] Chat page title uses `<Heading level={2}>`
- [ ] All headings render with Manrope 24px font-weight 600

**Proto mapping:** Frontend-only
**Files affected:** Drive route page, Approval route page, Chat route page

---

## Flow: Cross-Module Visual Audit

1. Navigate to `/approval` — verify header + button patterns
2. Navigate to `/channels` — verify sidebar labels + header + button patterns
3. Navigate to `/drive` — verify sidebar labels + header + button patterns
4. Compare all 3 side-by-side at 1280px — confirm visual parity
