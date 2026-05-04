## MODIFIED Requirements

### Requirement: Dense Chat View
The messaging view SHALL display messages in a high-density list. Sender names SHALL use `text-body-strong` (14px/600) in `gray-13` color. Message content SHALL use `text-body` (14px/400) in `gray-12`. Timestamps SHALL use `text-micro` (10px/500) in `gray-10`. The chat background SHALL be `gray-4`.

#### Scenario: Dense message rendering
- **WHEN** messages are listed in a channel
- **THEN** they SHALL render with correct typography roles: sender at 14px/600, content at 14px/400, time at 10px/500

#### Scenario: Message hover state
- **WHEN** user hovers over a message row
- **THEN** the background SHALL change to `gray-6` using `--duration-instant` (50ms) transition

### Requirement: Thread Peek Panel
Threads SHALL open in a dedicated right-side peek panel. The panel SHALL use `gray-4` background with `border-solid` left border. The slide animation SHALL use `--duration-normal` (200ms) with `--ease-out`.

#### Scenario: Opening a thread
- **WHEN** user clicks "Reply" on a message
- **THEN** the thread panel SHALL slide in from right over 200ms using the standard panel animation

#### Scenario: Thread panel header
- **WHEN** the thread panel renders
- **THEN** the header SHALL display "Thread" in `text-body-strong` (14px/600) with a close icon button
