## ADDED Requirements

### Requirement: Dense Chat View
The messaging view SHALL display messages in a high-density list without chat bubble backgrounds or gradients, maximizing vertical information density.

#### Scenario: Dense message rendering
- **WHEN** messages are listed in a channel
- **THEN** they SHALL appear as text blocks with user avatars, without decorative wrapper bubbles

### Requirement: Thread Peek Panel
Threads SHALL open in a dedicated right-side peek panel rather than overlaying the main view or navigating to a new page, allowing simultaneous context of the main channel and thread.

#### Scenario: Opening a thread
- **WHEN** user clicks "Reply" on a message
- **THEN** a side panel SHALL slide in from the right to display the thread
