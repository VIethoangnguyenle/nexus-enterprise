## ADDED Requirements

### Requirement: Lark-accurate background color tokens
The design system SHALL define background color tokens matching Lark dark mode:
- `bg-primary`: #0f1115 (app background)
- `bg-secondary`: #161a20 (panel background)
- `bg-tertiary`: #1e2228 (elevated surface)
- `bg-rail`: #0d1017 (sidebar rail)
- `bg-hover`: #1f242b (hover overlay, solid)
- `bg-active`: rgba(51,112,255,0.12) (selected state)

#### Scenario: Background colors render correctly
- **WHEN** any panel or surface renders in the app
- **THEN** it SHALL use the corresponding background token from the design system

### Requirement: Lark-accurate text color tokens
The design system SHALL define text color tokens:
- `text-primary`: #e6eaf0
- `text-secondary`: #9aa4b2
- `text-muted`: #6b7480

#### Scenario: Text hierarchy is visually distinct
- **WHEN** primary, secondary, and muted text appear on the same surface
- **THEN** each level SHALL be visually distinguishable with decreasing brightness

### Requirement: Muted blue accent color
The accent color SHALL be #3370ff (muted blue) instead of #6366f1 (indigo). Hover variant SHALL be #4a85ff.

#### Scenario: Active navigation item uses muted blue
- **WHEN** a navigation item is in active state
- **THEN** it SHALL use the muted blue accent for highlighting

### Requirement: Flat button primitives
All Button variants SHALL be flat — no gradients, no glow shadows, no translate transforms. Primary variant SHALL be `bg-accent text-white` with `hover:bg-accent-hover`. No `box-shadow` on any button variant.

#### Scenario: Primary button renders flat
- **WHEN** a primary Button is rendered
- **THEN** it SHALL have a solid blue background with no gradient or shadow

#### Scenario: Button hover has no elevation effect
- **WHEN** user hovers over any button variant
- **THEN** the button SHALL NOT translate vertically or increase shadow

### Requirement: Tight border radius
Border radius tokens SHALL be: sm=4px, md=6px, lg=8px. No element SHALL have border-radius greater than 8px except avatars.

#### Scenario: Buttons use tight radius
- **WHEN** a Button primitive renders
- **THEN** it SHALL use radius-sm (4px) for sm/md sizes

### Requirement: Compact spacing
Topbar height SHALL be 44px. List item padding SHALL be py-1.5 (6px vertical). Panel header padding SHALL be py-2 (8px vertical).

#### Scenario: Topbar is compact
- **WHEN** the workspace topbar renders
- **THEN** its height SHALL be 44px

### Requirement: Invisible scrollbar
Scrollbars SHALL be invisible by default. Scrollbar thumb SHALL appear only when the user hovers over the scrollable container. Width SHALL be 4px maximum.

#### Scenario: Scrollbar hidden at rest
- **WHEN** a scrollable container is not being hovered
- **THEN** no scrollbar thumb SHALL be visible

#### Scenario: Scrollbar appears on hover
- **WHEN** user hovers over a scrollable container
- **THEN** a 4px semi-transparent scrollbar thumb SHALL appear

### Requirement: No decorative CSS effects
The design system SHALL NOT use: `backdrop-blur`, `background-image: linear-gradient()` for UI chrome (buttons, headers), `box-shadow` with color spread (glow effects), or `transform: translateY` on hover states.

#### Scenario: Workspace header has no blur
- **WHEN** the workspace topbar renders
- **THEN** it SHALL NOT apply `backdrop-blur` or `backdrop-filter`
