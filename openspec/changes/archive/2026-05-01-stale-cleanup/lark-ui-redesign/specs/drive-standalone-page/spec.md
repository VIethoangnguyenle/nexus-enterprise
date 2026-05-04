## ADDED Requirements

### Requirement: Standalone Drive Route
The Drive module SHALL be accessible at the `/_drive` route group with its own layout component. It SHALL NOT share the workspace layout (no LarkRail visible). The page title SHALL be "Lark Docs" or equivalent branding.

#### Scenario: Drive page loads independently
- **WHEN** user navigates to `/drive` (via new tab)
- **THEN** the Drive page renders with its own layout: sidebar + content area
- **AND** no LarkRail is visible

### Requirement: Drive Sidebar with Folder Tree
The Drive page SHALL have a left sidebar (~180px) containing: search input, "Home" nav item, "Drive" nav item with expandable folder tree (My Folders, Shared Folders), and "Wiki" placeholder. Clicking "Drive" SHALL expand the folder tree showing My Folders and Shared Folders sections.

#### Scenario: Folder tree navigation
- **WHEN** user clicks "Drive" in the Drive sidebar
- **THEN** the sidebar expands to show "My Folders" (with sub-folders) and "Shared Folders" (with sub-folders)

#### Scenario: Folder selection
- **WHEN** user clicks a folder in the tree
- **THEN** the main content area shows the contents of that folder

### Requirement: Drive Action Bar
The Drive content area SHALL have an action bar at the top with: "New" (create document), "Upload" (upload files), and "Templates" (template gallery) buttons.

#### Scenario: Upload file
- **WHEN** user clicks "Upload" in the action bar
- **THEN** a file picker dialog opens for selecting files to upload

#### Scenario: Create folder
- **WHEN** user clicks "New" and selects "Folder"
- **THEN** a new folder is created in the current directory

### Requirement: Drive File Table
The Drive content area SHALL display files in a dense table view by default with columns: Name (with file-type icon), Modified date, and Created date. Each row SHALL have a "..." actions menu on hover. Row height SHALL be approximately 36–40px.

#### Scenario: List view renders
- **WHEN** the Drive page loads
- **THEN** files are displayed in a table with Name, Modified, and Created columns
- **AND** each file has a colored icon indicating its type (blue=doc, red=PDF, green=sheet, orange=slides)

#### Scenario: Row hover actions
- **WHEN** user hovers over a file row
- **THEN** a "..." menu appears on the right side of the row

### Requirement: Drive Tab Filters
The Drive content area SHALL have tab filters: "My Folders" and "Shared Folders" (when in Drive view), or "Recent", "Owned by Me", "Shared With Me", "Favorites" (when in Home view).

#### Scenario: Tab switching
- **WHEN** user clicks "Shared Folders" tab
- **THEN** the file table shows only shared folder contents

### Requirement: Sort and Display Controls
The Drive file table SHALL have Sort and Display Settings controls. Display Settings SHALL include a toggle between list view (table) and grid view (tiles).

#### Scenario: Toggle to grid view
- **WHEN** user clicks the grid view icon in Display Settings
- **THEN** files display as square tiles with preview thumbnail and name below

#### Scenario: Sort by modified date
- **WHEN** user clicks the "Modified" column header
- **THEN** files are sorted by modified date (toggle ascending/descending)
