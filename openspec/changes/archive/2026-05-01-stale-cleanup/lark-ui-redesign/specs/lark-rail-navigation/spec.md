## ADDED Requirements

### Requirement: Fixed-width LarkRail
The application SHALL render a fixed-width (~90px) navigation rail as the leftmost column of the workspace layout. The rail SHALL NOT be resizable or collapsible.

#### Scenario: Rail renders at correct width
- **WHEN** the workspace layout mounts
- **THEN** the LarkRail renders at approximately 90px width with `flex-shrink: 0`

### Requirement: Icon + Label Navigation Items
Each navigation item in the LarkRail SHALL display an icon (20px) and a short text label. The active module SHALL have a colored background highlight.

#### Scenario: Active module highlight
- **WHEN** user clicks "Messenger" in the rail
- **THEN** the Messenger item shows a colored active background and the ListPanel switches to ChatList content

#### Scenario: All modules visible
- **WHEN** the workspace loads
- **THEN** the rail displays: Messenger, Docs, Assets, Settings (minimum set), each with icon and label

### Requirement: Docs opens new browser tab
Clicking the "Docs" item in the LarkRail SHALL open the Drive page (`/drive`) in a **new browser tab** via `window.open('/drive', '_blank')`. It SHALL NOT perform SPA navigation.

#### Scenario: Docs click opens new tab
- **WHEN** user clicks "Docs" in the LarkRail
- **THEN** a new browser tab opens at `/drive`
- **AND** the current workspace tab remains unchanged

### Requirement: User avatar and search shortcut
The LarkRail SHALL display the user's avatar at the top and a search shortcut indicator (`Ctrl+K` or `⌘K`). A logout action SHALL be accessible from the avatar.

#### Scenario: Avatar displayed
- **WHEN** the workspace loads
- **THEN** the user's avatar appears at the top of the rail

#### Scenario: Logout from rail
- **WHEN** user clicks the avatar and selects logout
- **THEN** the user is logged out and redirected to the login page

### Requirement: Notification badges
Navigation items with unread content SHALL display a badge with the unread count. Items with non-count notifications SHALL display a red dot indicator.

#### Scenario: Unread badge on Messenger
- **WHEN** there are 5 unread messages across channels
- **THEN** the Messenger icon shows a badge with "5"
