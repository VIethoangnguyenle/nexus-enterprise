## ADDED Requirements

### Requirement: Double-ring focus indicator on buttons
All Button components SHALL display a double-ring focus indicator on `:focus-visible` state: `box-shadow: 0 0 0 2px var(--gray-4), 0 0 0 4px var(--color-accent)`.

#### Scenario: Keyboard focus on primary button
- **WHEN** user tabs to a primary Button
- **THEN** a 2px surface-gap ring followed by a 2px accent ring SHALL appear around the button

#### Scenario: Mouse click does not show focus ring
- **WHEN** user clicks a Button with mouse
- **THEN** the focus ring SHALL NOT appear (`:focus-visible` only, not `:focus`)

### Requirement: Double-ring focus indicator on inputs
All Input, Select, and Textarea components SHALL display the double-ring focus indicator on `:focus-visible`.

#### Scenario: Keyboard focus on text input
- **WHEN** user tabs to an Input field
- **THEN** the double-ring focus indicator SHALL appear and the border SHALL change to `--color-accent`

### Requirement: Focus indicator on navigation items
All clickable navigation items (sidebar links, tab items) SHALL display the double-ring focus indicator on `:focus-visible`.

#### Scenario: Keyboard navigation through sidebar
- **WHEN** user tabs through sidebar navigation items
- **THEN** each focused item SHALL show the double-ring focus indicator

### Requirement: Focus ring utility class
The system SHALL provide a `.focus-ring` utility class that any element can apply to gain the double-ring `:focus-visible` behavior.

#### Scenario: Custom interactive element uses focus-ring
- **WHEN** a developer applies `focus-ring` class to a custom interactive element
- **THEN** the element SHALL display the double-ring indicator on `:focus-visible`

### Requirement: Focus ring removes default outline
When the double-ring focus indicator is active, the browser's default `outline` SHALL be set to `none` to prevent duplicate focus indicators.

#### Scenario: No duplicate outline
- **WHEN** a Button receives keyboard focus
- **THEN** only the box-shadow-based double-ring SHALL be visible, with no browser outline
