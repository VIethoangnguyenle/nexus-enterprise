# Design: Approval Module Frontend

## BA Specs Review
**ACCEPTED** ✅ — All 10 user stories have browser-testable criteria, all states defined, flows complete.

## Design Decisions
- **Layout**: Follows existing workspace pattern — main content fills center area within the workspace shell (TopBar + AppSidebar already provided by `_workspace.tsx` layout)
- **Tabs over sidebar**: Approval module uses horizontal tabs (Pending/History/My Requests/Department) rather than a secondary sidebar — simpler mental model, fewer clicks, consistent with how Lark handles approvals
- **PeekPanel for details**: Clicking a row opens PeekPanel on right — same pattern as Drive context panel, consistent UX
- **Inline actions**: Approve/Reject in both list (icon buttons) and detail panel (full buttons with comment) — provides quick actions for experienced users and detailed flow for new users
- **Batch approve bar**: Floating bottom bar appears only when items selected — non-intrusive, same pattern as email batch actions

## Screen Inventory
| Screen | Purpose | Stitch Project | Layout Pattern |
|--------|---------|---------------|----------------|
| Approval List | Main tab-based list view | 8118651983093871135 | Tabs + DataTable |
| Approval Detail | Side panel with timeline | 8118651983093871135 | PeekPanel + Timeline |

## Screen Details

### Screen 1: Approval List (Main View)
**Layout**: Full content area within workspace shell (TopBar + AppSidebar provided by parent layout)

#### Component Mapping
| Visual Element | Codebase Component | Status |
|---------------|-------------------|--------|
| Page title "Approvals" | `<Heading size="h2">` | ✅ Reuse |
| Tab bar (Pending/History/My Requests/Department) | `<Tabs>` | ✅ Reuse — extend Tab type to support `badge?: number` |
| Approval data rows | `<DataTable>` | ✅ Reuse — configure columns |
| Status chip (Approved/Pending/Rejected) | `<Badge>` | ✅ Reuse — `variant` prop |
| Loading state | `<LoadingState>` | ✅ Reuse |
| Empty state | `<EmptyState>` | ✅ Reuse |
| Error state | `<ErrorState>` | ✅ Reuse |
| Row checkboxes | Native `<input type="checkbox">` | ✅ Native |
| Approve icon button | `<IconButton>` | ✅ Reuse |
| Reject icon button | `<IconButton>` | ✅ Reuse |
| Batch action bar | **NEW: `<BatchActionBar>`** | 🆕 Create — floating bottom bar |
| Load More button | `<Button variant="ghost">` | ✅ Reuse |
| Pending count on tab | Extend `<Tabs>` Tab interface | ⚠️ Minor extension |

#### States
- **Empty**: `<EmptyState icon={<ClipboardCheck>} title="No pending approvals" description="Items assigned to you will appear here" />`
- **Loading**: `<LoadingState />` centered in content area
- **Error**: `<ErrorState title="Failed to load approvals" onRetry={refetch} />`
- **Loaded**: DataTable with approval rows

### Screen 2: Approval Detail (PeekPanel)
**Layout**: Right-side PeekPanel, 400px width (desktop), full-screen (mobile)

#### Component Mapping
| Visual Element | Codebase Component | Status |
|---------------|-------------------|--------|
| Side panel container | `<PeekPanel>` | ✅ Reuse |
| Panel title + status badge | `<Heading>` + `<Badge>` | ✅ Reuse |
| Detail field rows | Custom layout (label/value pairs) | ✅ Simple div layout |
| Step timeline | `<Timeline>` | ✅ Reuse — map assignments to TimelineItem[] |
| Comment field | `<Textarea>` | ✅ Reuse |
| Approve button | `<Button variant="primary">` | ✅ Reuse — green color via CSS |
| Reject button | `<Button variant="error">` | ✅ Reuse |
| Confirm dialog | `<ConfirmDialog>` | ✅ Reuse |

#### States
- **Loading**: Skeleton lines in panel body
- **Loaded**: Full detail with timeline
- **Error**: Inline error message with retry

## New Components Needed

### 1. `<BatchActionBar>` (Composite)
**Purpose**: Floating bottom bar for batch operations
**Composed from**: `<Button>`, native layout
**Props**: `selectedCount: number`, `onBatchApprove: () => void`, `onClear: () => void`
**Responsive**: Full-width bar pinned to bottom, above MobileNav on mobile

### 2. `<ApprovalStatusBadge>` (tiny helper)
**Purpose**: Map approval status string to Badge variant with correct color
**Composed from**: `<Badge>`
**Logic**: `pending → amber, approved → green, rejected → red`

## Tabs Extension
The existing `<Tabs>` component needs a minor extension to support badge counts:
```typescript
interface Tab {
  id: string
  label: string
  icon?: ReactNode
  badge?: number  // NEW — show count after label
}
```

## Responsive Strategy
- **Mobile (375px)**: 
  - Full-width single column
  - Tabs scroll horizontally if needed
  - DataTable shows only: Request, Status columns (hide Requester, Step, Date)
  - PeekPanel opens full-screen overlay
  - BatchActionBar above MobileNav (bottom: 56px + 16px)
  - Approve/Reject as swipe actions or in detail panel only (no inline icons)
  
- **Tablet (768px)**:
  - Tabs visible fully
  - DataTable shows: Request, Requester, Status, Date (hide Step)
  - PeekPanel opens as overlay (60% width)

- **Desktop (1280px)**:
  - Full DataTable with all columns
  - PeekPanel as inline side panel (400px)
  - Batch action bar full-width with counters

## Color Mapping
| Status | Badge Color | Token |
|--------|------------|-------|
| pending | Amber | `bg-amber-500/10 text-amber-400` |
| approved | Green | `bg-green-500/10 text-green-400` |
| rejected | Red | `bg-red-500/10 text-red-400` |
| cancelled | Gray | `bg-gray-500/10 text-gray-400` |
