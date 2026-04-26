## ADDED Requirements

### Requirement: Asset management navigation
The frontend layout SHALL include an asset management section accessible from the sidebar navigation.

#### Scenario: Asset menu in sidebar
- **WHEN** a user views the workspace layout
- **THEN** the sidebar SHALL include an "Assets" section with links to: Dashboard, My Assets, Requests, and (if admin) Type Configuration

### Requirement: Notification indicator in header
The workspace layout header SHALL include a notification bell icon with unread count.

#### Scenario: Notification bell in header
- **WHEN** a user is in any workspace view
- **THEN** the header SHALL display a bell icon. If there are unread notifications, a badge with the count SHALL be shown

### Requirement: Thread view in chat
The chat interface SHALL support viewing and replying to message threads.

#### Scenario: Open thread panel
- **WHEN** a user clicks on a message's reply indicator
- **THEN** a side panel SHALL open showing the thread with all replies and an input field for new replies

#### Scenario: Thread indicator on messages
- **WHEN** a message has replies
- **THEN** the message SHALL display a reply count indicator (e.g., "3 replies") that opens the thread panel when clicked
