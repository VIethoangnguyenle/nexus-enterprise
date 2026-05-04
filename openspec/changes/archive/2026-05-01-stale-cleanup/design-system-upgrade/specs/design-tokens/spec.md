## ADDED Requirements

### Requirement: 14-shade grayscale surface scale
The system SHALL define CSS custom properties `--gray-1` through `--gray-14` with cool blue undertone hex values as specified in DESIGN.md. Each shade SHALL serve a distinct surface/text role.

#### Scenario: All gray tokens are defined
- **WHEN** the CSS theme loads
- **THEN** properties `--gray-1` through `--gray-14` SHALL all resolve to valid hex colors

#### Scenario: Surface shades produce visible distinction
- **WHEN** adjacent layout regions use consecutive gray values (e.g., Rail `gray-2` next to Sidebar `gray-3`)
- **THEN** the boundary SHALL be visually distinguishable without an explicit border

### Requirement: Semantic background aliases
The system SHALL define Tailwind-compatible aliases that map semantic names to gray scale values: `--color-bg-primary` → `var(--gray-4)`, `--color-bg-secondary` → `var(--gray-3)`, `--color-bg-rail` → `var(--gray-2)`, `--color-bg-hover` → `var(--gray-6)`, `--color-bg-active` → `var(--gray-7)`.

#### Scenario: Existing bg-bg-primary class resolves correctly
- **WHEN** an element uses Tailwind class `bg-bg-primary`
- **THEN** it SHALL render with background color `#141720` (`gray-4`)

#### Scenario: Backward compatibility
- **WHEN** existing components reference old token names (`bg-bg-primary`, `text-text-primary`)
- **THEN** they SHALL continue to render correctly via alias mapping

### Requirement: Functional color pairs
The system SHALL define functional color pairs (foreground + 8%-opacity background) for: primary/action (`#3370FF`), success (`#22C55E`), warning (`#F59E0B`), error (`#EF4444`), info (`#06B6D4`).

#### Scenario: Error color pair
- **WHEN** a component needs error styling
- **THEN** it SHALL use `--color-danger` (`#EF4444`) for text/border and `--color-danger-bg` (`rgba(239,68,68,0.08)`) for background

### Requirement: Border scale
The system SHALL define three border opacity levels: `--border-subtle` (0.05 opacity), `--border-default` (0.08), `--border-strong` (0.12), plus `--border-solid` (`#23252a`) for structural dividers.

#### Scenario: Table row dividers use subtle border
- **WHEN** a DataTable renders row boundaries
- **THEN** dividers SHALL use `border-subtle` (`rgba(255,255,255,0.05)`)

### Requirement: Typography utility classes
The system SHALL define `@utility` directives for 11 named typography roles matching DESIGN.md: `text-title` (18px/600), `text-section` (15px/600), `text-body` (14px/400), `text-body-ui` (14px/500), `text-body-strong` (14px/600), `text-small` (13px/400), `text-small-ui` (13px/500), `text-caption` (12px/400), `text-caption-ui` (12px/500), `text-overline` (11px/600/uppercase/0.5px tracking), `text-micro` (10px/500).

#### Scenario: text-title utility applies complete style
- **WHEN** an element uses class `text-title`
- **THEN** it SHALL render at 18px, weight 600, line-height 1.35, letter-spacing -0.3px

#### Scenario: text-overline utility includes uppercase
- **WHEN** an element uses class `text-overline`
- **THEN** it SHALL render at 11px, weight 600, line-height 1.35, letter-spacing 0.5px, text-transform uppercase

### Requirement: Motion tokens
The system SHALL define CSS custom properties for animation durations (`--duration-instant: 50ms`, `--duration-fast: 100ms`, `--duration-normal: 200ms`, `--duration-slow: 300ms`) and easing functions (`--ease-out`, `--ease-in`, `--ease-in-out`, `--ease-spring`).

#### Scenario: Panel slide animation uses normal duration
- **WHEN** a PeekPanel slides in
- **THEN** the transition SHALL use `--duration-normal` (200ms) with `--ease-out`

### Requirement: Depth/elevation presets
The system SHALL define 6 named elevation levels as CSS classes or custom properties: Recessed (darkest bg), Base (standard surfaces), Surface (cards), Raised (dropdowns), Floating (toasts), Overlay (modals).

#### Scenario: Modal uses overlay elevation
- **WHEN** a modal dialog renders
- **THEN** it SHALL use Overlay level shadow (`0 16px 48px rgba(0,0,0,0.5)`)

### Requirement: Inter Variable font features
The system SHALL set `font-feature-settings: "cv01", "ss03"` on the root element, applying to all text globally.

#### Scenario: Font features are active
- **WHEN** any text renders in the application
- **THEN** Inter Variable's cv01 (alt. 1) and ss03 (alt. 3) OpenType features SHALL be active
