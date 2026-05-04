## MODIFIED Requirements

### Requirement: Standard Data Table
The Drive file table SHALL be rendered within the standalone Drive page (`/_drive` route), NOT embedded in the workspace chat layout. The table SHALL have columns: Name (with colored file-type icon), Modified date, and Created date. Row height SHALL be approximately 36–40px. A "..." actions menu SHALL appear on row hover.

#### Scenario: Drive displays files as a table
- **WHEN** user views files in the standalone Drive page
- **THEN** files SHALL be listed in a structured data table with Name, Modified, and Created columns
- **AND** each row has a colored file-type icon (blue=doc, red=PDF, green=sheet, orange=slides)

### Requirement: Table Row Interactions
Data table rows SHALL use a flat background color change on hover. Rows SHALL show a "..." actions menu on the right side on hover. Row height SHALL be compact (36–40px).

#### Scenario: Hovering a file row
- **WHEN** user hovers over a table row
- **THEN** the row background darkens slightly and a "..." menu appears on the right

### Requirement: Breadcrumb Navigation
Hierarchical folder navigation in the Drive page SHALL use breadcrumb navigation at the top of the content area.

#### Scenario: Navigating folders
- **WHEN** user navigates deep into a folder structure in the Drive page
- **THEN** a breadcrumb trail appears allowing navigation back to parent folders

### Requirement: Peek Panel Details
Item details (file metadata, permissions) SHALL be displayed in a right-side peek panel within the Drive page layout.

#### Scenario: Viewing item details
- **WHEN** user clicks to view details of a file in the Drive page
- **THEN** a side panel slides in from the right showing file metadata
