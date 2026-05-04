## ADDED Requirements

### Requirement: Folder Tree Sidebar
The Drive UI SHALL render a folder tree in the ListPanel area (column 2) showing the workspace's folder hierarchy. The tree SHALL lazy-load children on expand.

#### Scenario: Initial tree load
- **WHEN** user navigates to Drive
- **THEN** the tree shows root-level folders only (children not loaded)

#### Scenario: Expand folder
- **WHEN** user clicks the expand arrow on a folder node
- **THEN** children folders are fetched and rendered under the parent with indentation

#### Scenario: Collapse folder
- **WHEN** user clicks the collapse arrow on an expanded folder
- **THEN** children are hidden (but cached, not refetched on re-expand)

### Requirement: Tree State Persistence
The folder tree SHALL persist expand/collapse state in the Zustand store, surviving route transitions within the workspace session.

#### Scenario: Navigate away and back
- **WHEN** user navigates from Drive to Messaging and back to Drive
- **THEN** previously expanded folders remain expanded

### Requirement: Active Path Highlighting
The tree SHALL highlight the currently selected folder and all ancestor folders in the path.

#### Scenario: Deep folder selected
- **WHEN** user navigates to /Projects/Q4/Reports
- **THEN** "Projects", "Q4", and "Reports" tree nodes are visually highlighted

### Requirement: Permission-Aware Tree
The folder tree SHALL NOT display folders the user cannot read. For workspace members, all workspace folders are readable by default.

#### Scenario: Shared folder not in workspace
- **WHEN** a user has been shared a specific subfolder but not its parent
- **THEN** only the shared folder appears in the tree (not the inaccessible parent hierarchy)
