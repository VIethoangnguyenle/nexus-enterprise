## ADDED Requirements

### Requirement: Context Panel Layout
The Drive UI SHALL include a right-side context panel that slides in when an item is selected. The panel SHALL NOT cause a full-page navigation.

#### Scenario: Select file in list
- **WHEN** user single-clicks a file in the main list
- **THEN** the context panel slides in from the right showing the file's preview tab

#### Scenario: Deselect
- **WHEN** user clicks empty space in the main list or presses Escape
- **THEN** the context panel slides out

### Requirement: Context Panel Tabs
The context panel SHALL support four tabs: Preview, Metadata, Permissions, and Activity.

#### Scenario: Switch between tabs
- **WHEN** user clicks the Metadata tab in the context panel
- **THEN** the panel shows file metadata (name, size, type, owner, created, modified)

#### Scenario: Permissions tab shows share list
- **WHEN** user clicks the Permissions tab for a file they own
- **THEN** the panel shows current shares with user/group names and their permission levels

### Requirement: Permission-Gated Tabs
The Permissions tab SHALL only be visible if the user has `share` permission on the selected item.

#### Scenario: No share permission
- **WHEN** user selects a file where share=false
- **THEN** the Permissions tab is not rendered in the tab bar

### Requirement: Share Dialog
The Permissions tab SHALL include an "Add" button that opens a share dialog. The dialog SHALL allow selecting a user/group and assigning read/write/comment permissions.

#### Scenario: Share a file
- **WHEN** user adds a share with write permission for another user
- **THEN** the system creates an NGAC association, emits a permission_changed event, and the share appears in the permissions list

#### Scenario: Revoke a share
- **WHEN** user removes an existing share
- **THEN** the system removes the NGAC association, emits a permission_changed event, and the share disappears from the list
