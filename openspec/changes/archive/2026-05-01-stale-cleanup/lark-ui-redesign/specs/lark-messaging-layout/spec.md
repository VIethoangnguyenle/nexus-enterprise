## MODIFIED Requirements

### Requirement: Dense Chat View
The messaging view SHALL display messages in a high-density list with subtle flat bubble backgrounds (dark navy/tinted row per message). Messages from the same sender SHALL be grouped — consecutive messages SHALL NOT repeat the avatar. Hover on a message SHALL reveal an inline action bar (Reply, React, Pin, More) floating above the message.

#### Scenario: Dense message rendering
- **WHEN** messages are listed in a channel
- **THEN** they SHALL appear with subtle tinted backgrounds, sender avatar + name on the first message of a group, and no repeated avatar for consecutive messages from the same sender

#### Scenario: Hover action bar
- **WHEN** user hovers over a message
- **THEN** a floating action bar appears above the message with Reply, React, Pin, and More actions

## ADDED Requirements

### Requirement: Pill-style Chat Header Tabs
The chat header SHALL display tabs as pill-shaped chips: "Chat" (active, filled), "Pinned" (outline with pin icon), and "+" (add tab). Right side SHALL show Search, Members, Settings, and More icons.

#### Scenario: Tab rendering
- **WHEN** user views a channel
- **THEN** the header shows pill-style tabs with "Chat" filled as active

#### Scenario: Switch to Pinned tab
- **WHEN** user clicks the "Pinned" pill tab
- **THEN** the content area shows pinned messages for the channel

### Requirement: Minimal Input Bar
The chat input bar SHALL have a minimal design with placeholder text "Message [channel name]", and right-aligned tool icons: formatting (Aa), emoji, attachment, and send button. No heavy border or prominent container styling.

#### Scenario: Input bar renders
- **WHEN** user views a channel
- **THEN** the input bar appears at the bottom with minimal styling and right-aligned tool icons

### Requirement: Timestamp Dividers
Message timestamps SHALL appear as centered dividers in the message flow when there is a significant time gap between messages (e.g., different days or >30 minute gap).

#### Scenario: Time gap divider
- **WHEN** two consecutive messages are more than 30 minutes apart
- **THEN** a centered timestamp divider appears between them
