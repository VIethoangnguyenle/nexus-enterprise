# Drive Module — Stitch Compliance Spec

## Stitch Reference Screens
- `08-drive-my-files.png` — Main file list view
- `09-drive-my-files-v2.png` — File preview dialog
- `10-drive-folder-mgmt.png` — Folder management view
- `11-drive-shared.png` — Shared & interactions view
- `12-drive-file-mgmt.png` — File management with context menu & detail panel

## Stitch Design Patterns (Source of Truth)

### Sidebar (from Stitch screens 08, 12)
- **Header**: "Global Drive" with workspace name subtitle, left-aligned
- **Primary CTA**: Blue button "⬆ Upload File" — full-width, primary variant
- **Navigation items**: "All Files", "Recent", "Shared", "Starred", "Trash" — icon + label, left-aligned
- **Section labels**: "DEPARTMENTS" header (label-caps style, 11px uppercase tracking)
- **Department items**: indented list items below section header
- **Footer**: "Storage Usage" + "Help" links at bottom

### Content Area (from Stitch screen 08)
- **Breadcrumbs**: "All Files > Finance Dept > Q3 Reports" — clickable path segments
- **Header row**: Folder title + item count ("3 items · 12.4 MB"), right-aligned view toggles (grid/list icons)
- **View toggles**: IconButton primitives, NOT raw buttons
- **Filter pills**: "All Types", "Doc", "Sheet", "Slide", "PDF" — horizontal row of filter chips
- **Data table**: columns NAME, MODIFIED DATE, OWNER (with avatar), SIZE
- **Row actions**: Three-dot menu on each row (context menu pattern)

### Context Menu (from Stitch screen 12)
- Items: "Download", "Move to...", "Share", "Delete" (delete in red)
- Uses standard dropdown menu pattern

### Detail Panel (from Stitch screen 12)
- "File Details" header
- File preview thumbnail
- File info: name, type, size
- "INFORMATION" section: Created, Modified, Owner
- "ACCESS" section: shared users with role badges

---

## User Stories

### US-1: Drive sidebar uses Stitch primitives
As a workspace user,
I want the Drive sidebar to match the Stitch design exactly,
so that the UI is consistent with the design system.

**Acceptance Criteria:**
- [ ] Sidebar header shows workspace icon + "Global Drive" text (not "NAVIGATION")
- [ ] Primary CTA is a `<Button>` primitive with variant="primary" (not raw `<button>`)
- [ ] Upload button is full-width primary button "⬆ Upload File"
- [ ] Navigation items use consistent icon + label pattern
- [ ] Section headers use `label-caps` typography (11px, uppercase, tracking-widest)
- [ ] All raw `<button>` elements in DriveSidebar.tsx replaced with primitives

**Proto mapping:** Frontend-only — no backend changes
**Files affected:** `DriveSidebar.tsx`

### US-2: Drive toolbar uses Stitch button primitives
As a workspace user,
I want the Drive toolbar actions (Upload, New Folder, view toggles) to use proper Button/IconButton primitives,
so that buttons look consistent with the rest of the platform.

**Acceptance Criteria:**
- [ ] "Upload" action uses `<Button>` primitive with variant="primary"
- [ ] "New Folder" action uses `<Button>` primitive with variant="outline"
- [ ] View toggle (list/grid) uses `<IconButton>` primitive
- [ ] Filter pills use consistent chip/pill pattern
- [ ] No raw `<button>` elements in drive toolbar area

**Proto mapping:** Frontend-only
**Files affected:** `DriveContextPanel.tsx`, `DriveFilterPills.tsx`

### US-3: Drive file row context menu uses consistent pattern
As a workspace user,
I want file row actions to match the Stitch context menu design,
so that right-click/more menus are consistent.

**Acceptance Criteria:**
- [ ] Three-dot menu button uses `<IconButton>` primitive
- [ ] Context menu items use standard dropdown pattern
- [ ] "Delete" action styled in error/red color
- [ ] All raw `<button>` elements in context menus replaced

**Proto mapping:** Frontend-only
**Files affected:** `DriveFileRow.tsx`

---

## UI States

| State | Description |
|-------|-------------|
| Empty | No files/folders — show empty state message |
| Loading | Skeleton/spinner while fetching |
| Loaded | File list with data |
| Error | API failure — show error state |
| Permission denied | User lacks drive access |

---

## Flow: Drive File Browse

1. User navigates to `/drive`
2. Sidebar shows navigation + folder tree
3. Content area shows file list for current folder
4. User clicks view toggle → switches between list/grid
5. User clicks filter pill → filters by file type
6. User clicks three-dot menu → context menu appears
7. User selects action → action executes
