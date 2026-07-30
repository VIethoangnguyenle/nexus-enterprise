# lark-data-table-layout

## Purpose
Give list-shaped modules one consistent tabular presentation, so Drive and Assets do not each invent their own.

## Status

Verified against code 2026-07-31. **Partially implemented.**

Row-based rendering exists (`DriveFileList.tsx`, `DriveFileRow.tsx`). Breadcrumb navigation was
found only in `routes/_workspace/contacts.tsx`, not in Drive, so the breadcrumb requirement is
unmet where this spec most calls for it.

## Requirements

### Requirement: Standard Data Table
Drive and Assets modules SHALL use a common data-table layout for displaying items, replacing grid and card views. The table SHALL have a header row with column titles, and flat border separators between rows.

#### Scenario: Drive displays files as a table
- **WHEN** user views files in Drive
- **THEN** files SHALL be listed in a structured data table

### Requirement: Table Row Interactions
Data table rows SHALL NOT use box shadows or translate transforms on hover. They SHALL use a flat background color change (e.g., `bg-bg-hover`) to indicate interactivity.

#### Scenario: Hovering a file row
- **WHEN** user hovers over a table row
- **THEN** the row background SHALL darken slightly without elevation

### Requirement: Breadcrumb Navigation
Hierarchical data (like Drive folders) SHALL use breadcrumb navigation at the top of the view rather than inline "Up" buttons.

#### Scenario: Navigating folders
- **WHEN** user navigates deep into a folder structure
- **THEN** a breadcrumb trail SHALL appear allowing navigation back to parent folders

### Requirement: Peek Panel Details
Item details (e.g., file metadata, asset history) SHALL be displayed in a right-side peek panel rather than a centered modal dialog.

#### Scenario: Viewing item details
- **WHEN** user clicks to view details of an item
- **THEN** a side panel SHALL slide in from the right over the content area
