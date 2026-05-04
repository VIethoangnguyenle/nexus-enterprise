# Design: Approval Module Redesign v2

## BA Specs Review
**ACCEPTED** ✅ — 20 stories, ~60 AC, all browser-testable. States defined. Flows complete.

## Design Decisions
- **Content-area only**: All screens generate ONLY content area — app shell (AppSidebar, TopBar, MobileNav) already provided by `_workspace.tsx` layout
- **Tabs for filtering**: 5 horizontal tabs (Pending, History, My Requests, Department, Templates) — consistent with Lark/Feishu pattern, no secondary sidebar
- **DataTable for lists**: Reuse existing `<DataTable>` composite for ALL list views — zero custom tables
- **PeekPanel for details**: Reuse existing `<PeekPanel>` composite — zero custom side panels
- **Modal for create/edit**: Reuse existing `<Modal>` composite for template creation, request creation
- **Dynamic form renderer**: New composite `<DynamicFormRenderer>` built from existing primitives (Input, Select, Textarea)
- **Template form builder**: New composite `<FormFieldBuilder>` for admin template editor

## Stitch Project
- **Project ID**: `14852434379132121789` (configured MCP project — all NGAC designs)
- **New screens generated (v2)**:
  - "Approval Dashboard (CFO View)" — List + Detail split view
  - "Create Approval Request" — Modal with dynamic form
  - "Template Management" — Create/edit template modal

## Screen Inventory

| # | Screen | Purpose | Layout Pattern |
|---|--------|---------|----------------|
| 1 | Approval List | Main tab-based list view | Tabs + DataTable |
| 2 | Approval Detail | Right-side detail panel | PeekPanel + Timeline |
| 3 | Create Request | Modal with template selector + dynamic form | Modal |
| 4 | Template List | Admin tab showing templates | DataTable (same page, "Templates" tab) |
| 5 | Template Create/Edit | Modal with form builder | Modal (scrollable) |

## Screen Details

### Screen 1: Approval List (Main View)
**Layout**: Content area within workspace shell. Header + Tabs + DataTable.

#### Component Mapping
| Visual Element | Codebase Component | Status |
|---------------|-------------------|--------|
| Page title "Approvals" | `<Heading as="h2">` | ✅ Reuse |
| "New Request" button | `<Button variant="primary">` | ✅ Reuse |
| Tab bar (5 tabs with count badge) | `<Tabs>` | ✅ Reuse |
| Approval data rows | `<DataTable>` | ✅ Reuse — configure columns |
| Status badge | `<Badge>` | ✅ Reuse — variant prop |
| Row checkboxes | DataTable built-in selection | ✅ Reuse |
| Approve icon button | `<IconButton>` | ✅ Reuse |
| Reject icon button | `<IconButton>` | ✅ Reuse |
| Batch action bar | `<BatchActionBar>` | ✅ Exists |
| Load More button | `<Button variant="ghost">` | ✅ Reuse |
| Loading state | `<LoadingState>` | ✅ Reuse |
| Empty state | `<EmptyState>` | ✅ Reuse |
| Error state | `<ErrorState>` | ✅ Reuse |

#### States
- **Empty**: `<EmptyState icon={<ClipboardCheck>} title="No pending approvals" />`
- **Loading**: `<LoadingState />`
- **Error (not provisioned)**: `<EmptyState title="Approvals not configured" />`
- **Error (API)**: `<ErrorState onRetry={refetch} />`
- **Loaded**: DataTable with rows

### Screen 2: Approval Detail (PeekPanel)
**Layout**: Right-side PeekPanel, 400px (desktop), full-screen overlay (mobile)

#### Component Mapping
| Visual Element | Codebase Component | Status |
|---------------|-------------------|--------|
| Panel container | `<PeekPanel>` | ✅ Reuse |
| Panel title + status | `<Heading>` + `<Badge>` | ✅ Reuse |
| Form data display | **NEW: `<FormDataDisplay>`** | 🆕 Composed from Text primitives |
| Step timeline | `<Timeline>` | ✅ Reuse |
| Comment textarea | `<Textarea>` | ✅ Reuse |
| Approve button | `<Button variant="primary">` | ✅ Reuse |
| Reject button | `<Button variant="error">` | ✅ Reuse |
| Audit log entries | Simple list with `<Text>` | ✅ Reuse |
| Close button | `<IconButton>` | ✅ Reuse |

### Screen 3: Create Request (Modal)
**Layout**: Centered Modal (640px wide)

#### Component Mapping
| Visual Element | Codebase Component | Status |
|---------------|-------------------|--------|
| Modal shell | `<Modal>` | ✅ Reuse |
| Template selector | `<Select>` | ✅ Reuse |
| Dynamic form fields | **NEW: `<DynamicFormRenderer>`** | 🆕 See below |
| Step preview | `<Timeline>` (horizontal) | ✅ Reuse (adapt layout) |
| Cancel button | `<Button variant="ghost">` | ✅ Reuse |
| Submit button | `<Button variant="primary">` | ✅ Reuse |
| Confirm dialog | `<ConfirmDialog>` | ✅ Reuse |

### Screen 4: Template List (Admin Tab)
Same DataTable as Screen 1, filtered by "Templates" tab.

#### Component Mapping
| Visual Element | Codebase Component | Status |
|---------------|-------------------|--------|
| Template rows | `<DataTable>` | ✅ Reuse — different columns |
| Active/Inactive badge | `<Badge>` | ✅ Reuse |
| "Create Template" button | `<Button variant="primary">` | ✅ Reuse |

### Screen 5: Template Create/Edit (Modal)
**Layout**: Large Modal (720px, scrollable)

#### Component Mapping
| Visual Element | Codebase Component | Status |
|---------------|-------------------|--------|
| Modal shell | `<Modal>` | ✅ Reuse |
| Template name input | `<Input>` | ✅ Reuse |
| Entity type select | `<Select>` | ✅ Reuse |
| Active toggle | Native toggle (extend primitives if needed) | ⚠️ May need primitive |
| Form field builder | **NEW: `<FormFieldBuilder>`** | 🆕 See below |
| Step builder | **NEW: `<StepBuilder>`** | 🆕 See below |
| Save/Cancel buttons | `<Button>` | ✅ Reuse |

## New Components Needed

### 1. `<DynamicFormRenderer>` (Composite)
**Purpose**: Renders form fields dynamically based on template field definitions
**Composed from**: `<Input>`, `<Select>`, `<Textarea>`, `<Text>` (labels)
**Props**: `fields: FormFieldDefinition[]`, `values: Record<string, any>`, `onChange`, `errors`
**Responsive**: Full-width on all breakpoints

### 2. `<FormDataDisplay>` (Composite)
**Purpose**: Displays submitted form data as label/value pairs (read-only)
**Composed from**: `<Text>` primitives
**Props**: `fields: FormFieldDefinition[]`, `data: Record<string, any>`
**Responsive**: Stacked label/value on mobile

### 3. `<FormFieldBuilder>` (Composite)
**Purpose**: Admin form builder — add/remove/reorder form field definitions
**Composed from**: `<Input>`, `<Select>`, `<IconButton>`, `<Button>`, `<Card>`
**Props**: `fields: FormFieldDefinition[]`, `onChange`
**Responsive**: Full-width cards

### 4. `<StepBuilder>` (Composite)
**Purpose**: Admin step builder — add/remove/reorder approval steps
**Composed from**: `<Input>`, `<Select>`, `<Badge>`, `<IconButton>`, `<Card>`
**Props**: `steps: ApprovalStep[]`, `onChange`
**Responsive**: Full-width cards

## Responsive Strategy
- **Mobile (375px)**:
  - Single column, full-width
  - Tabs scroll horizontally
  - DataTable shows: Request + Status only (hide Requester, Step, Date)
  - PeekPanel opens as full-screen overlay
  - Modal opens full-screen
  - BatchActionBar above MobileNav
  
- **Tablet (768px)**:
  - Tabs fully visible
  - DataTable shows: Request, Status, Date
  - PeekPanel as overlay (60% width)
  - Modal centered (640px)

- **Desktop (1280px)**:
  - Full DataTable with all columns
  - PeekPanel inline (400px)
  - Modal centered (640-720px)

## Color Mapping
| Status | Badge Variant | Visual |
|--------|--------------|--------|
| pending | amber | `bg-amber-500/10 text-amber-600` |
| approved | green | `bg-green-500/10 text-green-600` |
| rejected | red | `bg-red-500/10 text-red-600` |
| cancelled | gray | `bg-gray-500/10 text-gray-500` |
