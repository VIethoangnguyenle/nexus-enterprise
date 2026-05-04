# Design: Admin Organization Management

## Design Decisions
- Layout follows existing workspace module pattern: admin sub-sidebar (240px) + content area + detail panel
- Tree view for departments matches the Lark/Feishu organization structure experience
- Data table for users follows the same density pattern as existing Drive/Assets list views
- Permission matrix uses checkbox grid — most intuitive for enterprise role management
- Admin sub-sidebar is separate from AppSidebar — it's the content area's own navigation
- Detail panels use PeekPanel pattern (slide-over from right) consistent with rest of app

## Stitch Project
- Project: Nexus Hub (`14852434379132121789`)
- Design System: Serene Enterprise (Manrope/Inter, Primary #2563EB, Light mode)

## Screen Inventory

| Screen | Purpose | Stitch ID | Layout Pattern |
|--------|---------|-----------|----------------|
| Admin — Organization | Department tree + detail panel | `8de9f83a14db4fb9865fc7c5a7b6160c` | Admin sub-nav left + tree center + PeekPanel right |
| Admin — Users | Member list + user detail | `50b5ea4299ff494d884b96da51211d7a` | Admin sub-nav left + DataTable center + PeekPanel right |
| Admin — Roles | Role cards + permission matrix | `fa17519cac3944198522f0f03c9340e9` | Admin sub-nav left + card list center + PeekPanel right |

---

## Screen Details

### Screen 1: Admin — Organization (Department Tree)

**Stitch Reference**: `projects/14852434379132121789/screens/8de9f83a14db4fb9865fc7c5a7b6160c`
**Layout**: Admin sub-nav (240px) + Tree view (fluid) + Department detail PeekPanel (320px)

#### Component Mapping

| Visual Element | Codebase Component | Status |
|---------------|-------------------|--------|
| Admin sub-sidebar | **NEW: `<AdminSidebar>`** | Simple nav list — 3 items |
| Section header "ADMIN" | Text with `label-caps` token | Inline styling |
| Nav item (active/inactive) | Styled link — similar to existing sidebar items | Inline |
| Department tree | ✅ `<TreeView>` from `patterns/` | Reuse — needs dept data adapter |
| Tree expand/collapse chevron | Part of `<TreeView>` | Existing |
| Member count badge | ✅ `<Badge>` from `primitives/` | Reuse |
| "New Department" button | ✅ `<Button variant="primary">` | Reuse |
| Department detail panel | ✅ `<PeekPanel>` from `composites/` | Reuse |
| Member avatar list in panel | ✅ `<Avatar>` from `primitives/` | Reuse |
| "Edit Name" action | ✅ `<Button variant="ghost">` | Reuse |
| Create department modal | ✅ `<Modal>` from `composites/` | Reuse |
| Parent department dropdown | **NEW: `<DepartmentPicker>`** | Tree-based selector in modal |
| Delete confirmation | ✅ `<ConfirmDialog>` from `composites/` | Reuse |

#### States
- **Empty**: Centered icon + "No departments yet. Create your first department." + CTA button
- **Loading**: Skeleton tree rows (3 levels, 5 rows)
- **Error**: "Failed to load organization structure" + Retry button
- **Loaded**: Expandable tree with departments, member counts, and interactive selection

---

### Screen 2: Admin — Users (Member Management)

**Stitch Reference**: `projects/14852434379132121789/screens/50b5ea4299ff494d884b96da51211d7a`
**Layout**: Admin sub-nav (240px) + DataTable (fluid) + User detail PeekPanel (360px)

#### Component Mapping

| Visual Element | Codebase Component | Status |
|---------------|-------------------|--------|
| Admin sub-sidebar | `<AdminSidebar>` | Shared with all admin screens |
| Search input | ✅ `<Input>` from `primitives/` | Reuse |
| Department filter dropdown | ✅ `<Select>` from `primitives/` | Reuse |
| Role filter dropdown | ✅ `<Select>` from `primitives/` | Reuse |
| User data table | ✅ `<DataTable>` from `composites/` | Reuse |
| Avatar in table | ✅ `<Avatar>` from `primitives/` | Reuse |
| Role badge chips | ✅ `<Badge>` from `primitives/` | Reuse |
| Status dot indicator | ✅ `<Badge variant="dot">` | Reuse |
| User detail panel | ✅ `<PeekPanel>` from `composites/` | Reuse |
| "Change Department" button | ✅ `<Button>` | Opens DepartmentPicker |
| "Edit Roles" button | ✅ `<Button>` | Opens role checkbox modal |
| Department breadcrumb | Inline text | `Parent → Child` format |
| User count header | Inline text | `"26 users"` |

#### States
- **Empty**: "No users in this organization"
- **Loading**: Skeleton table rows (6 rows, 5 columns)
- **Error**: "Failed to load users" + Retry button
- **Loaded**: Data table with search/filter, clickable rows

---

### Screen 3: Admin — Roles & Permissions

**Stitch Reference**: `projects/14852434379132121789/screens/fa17519cac3944198522f0f03c9340e9`
**Layout**: Admin sub-nav (240px) + Role card list (fluid) + Role detail PeekPanel (380px)

#### Component Mapping

| Visual Element | Codebase Component | Status |
|---------------|-------------------|--------|
| Admin sub-sidebar | `<AdminSidebar>` | Shared |
| Role card | ✅ Card pattern using `<div>` with border styling | Inline |
| "System" badge | ✅ `<Badge variant="secondary">` | Reuse |
| Permission chips | ✅ `<Badge>` from `primitives/` | Reuse |
| "New Role" button | ✅ `<Button variant="primary">` | Reuse |
| Role detail panel | ✅ `<PeekPanel>` from `composites/` | Reuse |
| Permission matrix | **NEW: `<PermissionMatrix>`** | Checkbox grid table |
| Matrix checkbox | ✅ Checkbox from `primitives/` (or native) | Reuse/extend |
| Member list in panel | ✅ `<Avatar>` + text | Reuse |
| "Edit Permissions" button | ✅ `<Button variant="primary">` | Reuse |
| Delete role action | ✅ `<Button variant="ghost">` + `<ConfirmDialog>` | Reuse |
| Create role modal | ✅ `<Modal>` + `<Input>` | Reuse |

#### States
- **Empty**: "No custom roles. Create your first role." + CTA button (system roles always shown)
- **Loading**: Skeleton cards (4 cards)
- **Error**: "Failed to load roles" + Retry button
- **Loaded**: Card list with system + custom roles, clickable for details

---

## New Components Needed

### 1. `<AdminSidebar>` (Simple)
- Vertical nav list with 3 items
- Uses existing routing via TanStack Router `<Link>`
- Active state: primary tint background
- Composed from: `<Link>` + icons + inline styling
- **Minimal effort — pure composition, no new primitive**

### 2. `<DepartmentPicker>` (Medium)
- Modal with searchable tree selector
- Composes: `<Modal>` + `<TreeView>` + `<Input>` (search)
- Used in: User detail (change department), Create department (select parent)
- **Composition of existing components**

### 3. `<PermissionMatrix>` (Medium)
- Table with checkboxes for resource × operation grid
- Read-only mode + edit mode
- Uses: HTML `<table>` + `<Checkbox>` primitive
- **Only truly new visual pattern — checkbox grid**

---

## Responsive Strategy

### Desktop (≥1280px)
- Full layout: admin sub-nav + content + detail panel
- Detail panel overlays or pushes content

### Tablet (768px–1279px)
- Admin sub-nav collapses to icons only (64px)
- Detail panel becomes overlay modal

### Mobile (≤767px)
- Admin sub-nav becomes horizontal tabs at top
- Full-width content, detail panel is full-screen overlay
- Tree view: full width with touch-friendly 44px row height
