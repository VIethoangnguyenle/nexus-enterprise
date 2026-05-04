## ADDED Requirements

### Requirement: Hierarchical Sidebar Navigation
The layout SHALL feature a left sidebar (default width 240px) replacing the legacy icon rail. It SHALL include the workspace switcher, search input, and hierarchical navigation links (Messaging, Drive, Assets).

#### Scenario: Sidebar renders in workspace
- **WHEN** user is in the workspace view
- **THEN** they SHALL see the expanded sidebar with text labels and icons

### Requirement: Sidebar Collapse
The sidebar SHALL be collapsible to an icon-only mode (width 64px) via a toggle button, maximizing the main content area.

#### Scenario: User collapses sidebar
- **WHEN** the user clicks the collapse toggle
- **THEN** the sidebar width shrinks to 64px and text labels are hidden

### Requirement: Sidebar Global Search
The sidebar SHALL contain a global search input at the top (below workspace switcher), styled with flat borders and a search icon.

#### Scenario: Search input is accessible
- **WHEN** the sidebar is expanded
- **THEN** a global search input SHALL be visible and usable
