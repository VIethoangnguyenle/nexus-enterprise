## MODIFIED Requirements

### Requirement: Hierarchical Sidebar Navigation
The layout SHALL feature a left sidebar (default width 240px) replacing the legacy icon rail. It SHALL include the workspace switcher, search input, and hierarchical navigation links (Messaging, Drive, Assets). Navigation items SHALL use `text-small-ui` typography role (13px/500) with `gray-11` text color. Active items SHALL use `primary-bg` background with `gray-13` text. The sidebar background SHALL be `gray-3` (`#0f1115`).

#### Scenario: Sidebar renders in workspace
- **WHEN** user is in the workspace view
- **THEN** they SHALL see the expanded sidebar with text labels at 13px/500 weight and icons at 16px

#### Scenario: Sidebar uses correct surface color
- **WHEN** the sidebar renders
- **THEN** its background SHALL be `gray-3` (`#0f1115`), visually distinct from the content area `gray-4`

### Requirement: Sidebar Collapse
The sidebar SHALL be collapsible to an icon-only mode (width 64px) via a toggle button, maximizing the main content area. The collapse transition SHALL use `--duration-normal` (200ms) with `--ease-out`.

#### Scenario: User collapses sidebar
- **WHEN** the user clicks the collapse toggle
- **THEN** the sidebar width SHALL animate to 64px over 200ms and text labels SHALL be hidden

### Requirement: Sidebar Global Search
The sidebar SHALL contain a global search input at the top (below workspace switcher). The input SHALL use recessed styling: `gray-1` background, `border-default` border, `text-small` typography.

#### Scenario: Search input styling
- **WHEN** the sidebar is expanded
- **THEN** the search input SHALL have a `gray-1` recessed background and `border-default` border
