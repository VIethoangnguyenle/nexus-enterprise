## MODIFIED Requirements

### Requirement: Standard Data Table
Drive and Assets modules SHALL use a common data-table layout for displaying items. The table header SHALL use `text-caption-ui` typography (12px/500 uppercase) with `gray-10` text color on `gray-3` background. Table rows SHALL be 36px height (dense mode) with `border-subtle` bottom borders. Cell padding SHALL be `0 12px`.

#### Scenario: Drive displays files as a table
- **WHEN** user views files in Drive
- **THEN** files SHALL be listed in a structured data table with 36px row height

#### Scenario: Table header typography
- **WHEN** the data table renders
- **THEN** column headers SHALL use 12px/500 uppercase text in `gray-10` color

### Requirement: Table Row Interactions
Data table rows SHALL use `gray-6` background on hover (not arbitrary bg values). Selected rows SHALL use `primary-bg` (`rgba(51,112,255,0.08)`) background. Hover transition SHALL use `--duration-instant` (50ms).

#### Scenario: Hovering a file row
- **WHEN** user hovers over a table row
- **THEN** the row background SHALL change to `gray-6` (`#21252e`)

#### Scenario: Selected row highlighting
- **WHEN** a table row is selected
- **THEN** the row background SHALL be `primary-bg` (`rgba(51,112,255,0.08)`)

### Requirement: Peek Panel Details
Item details SHALL be displayed in a right-side peek panel. The panel background SHALL be `gray-4`. The slide-in animation SHALL use `--duration-normal` (200ms) with `--ease-out`. The panel header SHALL use `text-body-strong` (14px/600) typography.

#### Scenario: Peek panel animation
- **WHEN** user clicks to view item details
- **THEN** the panel SHALL slide in from right over 200ms with ease-out easing
