## ADDED Requirements

### Requirement: Dense Chat List
The ListPanel SHALL render a ChatList component when the active module is "Messenger". Each chat item SHALL display: circle avatar (~36px), chat name (bold, truncated), last message preview (gray, one line), timestamp (right-aligned), and unread badge (red circle with count).

#### Scenario: Chat list renders channels
- **WHEN** the Messenger module is active
- **THEN** the ListPanel shows all workspace channels as dense chat items with avatar, name, preview, timestamp

#### Scenario: Active chat highlight
- **WHEN** user clicks a chat item
- **THEN** that item gets a subtle lighter background
- **AND** the ContentPanel loads the corresponding channel chat view

### Requirement: Chat item height
Each chat item SHALL have a height of approximately 56–60px with compact vertical spacing (no gaps between items).

#### Scenario: Dense layout
- **WHEN** the chat list renders 20 channels
- **THEN** all 20 items are visible without excessive whitespace between them

### Requirement: Unread badge
Chat items with unread messages SHALL display a red badge with the count on the right side. Items without unread messages SHALL show no badge.

#### Scenario: Unread count display
- **WHEN** a channel has 3 unread messages
- **THEN** the chat item shows a red badge with "3" aligned to the right

### Requirement: Resizable ListPanel
The ListPanel SHALL be resizable by dragging its right border. The width SHALL be constrained between 200px (minimum) and 480px (maximum) with a default of 280px.

#### Scenario: Drag to resize
- **WHEN** user hovers near the right edge of the ListPanel
- **THEN** the cursor changes to `col-resize`
- **AND** user can drag to resize the panel width

#### Scenario: Width persistence
- **WHEN** user resizes the ListPanel to 350px and refreshes the page
- **THEN** the ListPanel renders at 350px (persisted to localStorage)

#### Scenario: Min/max constraints
- **WHEN** user tries to drag the ListPanel narrower than 200px
- **THEN** the width stops at 200px
- **AND** when user tries to drag wider than 480px, the width stops at 480px

#### Scenario: Double-click reset
- **WHEN** user double-clicks the resize handle
- **THEN** the ListPanel width resets to the default 280px

### Requirement: Search bar
The ListPanel SHALL include a search input at the top for filtering conversations.

#### Scenario: Filter by name
- **WHEN** user types "general" in the search bar
- **THEN** only channels with names matching "general" are shown in the list
