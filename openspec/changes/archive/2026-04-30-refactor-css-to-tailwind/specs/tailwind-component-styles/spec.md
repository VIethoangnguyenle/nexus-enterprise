## ADDED Requirements

### Requirement: All component styles inline in JSX
Every UI component SHALL define its visual styles exclusively through Tailwind v4 utility classes applied directly in JSX. No component SHALL rely on custom CSS class selectors defined in `index.css` for layout, color, spacing, typography, or interactive states.

#### Scenario: Component renders without custom CSS classes
- **WHEN** any BEM-style CSS class (`.nexus-sidebar__*`, `.msg-bubble*`, `.chat-list-item*`, `.nexus-topbar__*`, `.pill-tab*`, `.chat-header__*`, `.unread-badge`, `.chat-external-badge`, `.timestamp-*`, `.resize-handle*`, `.bg-nexus-auth`) is removed from `index.css`
- **THEN** the corresponding component SHALL render identically using only Tailwind utility classes in JSX

#### Scenario: No specificity conflict between CSS and Tailwind
- **WHEN** a Tailwind utility class and a custom CSS class both target the same CSS property on the same element
- **THEN** this situation SHALL NOT exist after migration — only Tailwind utilities SHALL control styling

### Requirement: index.css contains only tokens, utilities, and editor styles
After migration, `index.css` SHALL contain only:
1. `@theme` design token declarations
2. `@utility` custom utility definitions (typography roles, focus-ring, scrollbar-none, animation helpers)
3. `@keyframes` animation definitions
4. Scrollbar pseudo-element styles
5. TipTap editor styles (`.chat-editor-content`, `.message-html`) — required for third-party HTML rendering

#### Scenario: No BEM component classes remain
- **WHEN** `index.css` is searched for BEM-style selectors (`.nexus-*`, `.msg-*`, `.chat-list-*`, `.pill-*`, `.unread-*`, `.timestamp-*`, `.resize-*`, `.bg-nexus-*`)
- **THEN** zero matches SHALL be found

#### Scenario: File size reduction
- **WHEN** `index.css` is measured after migration
- **THEN** it SHALL contain fewer than 400 lines (down from ~820)

### Requirement: Animation helpers as Tailwind utilities
All animation helper classes (`.animate-reaction-pop`, `.animate-msg-slide-in`, `.animate-panel-slide`, `.animate-slide-in-right`) SHALL be converted to `@utility` directives so they can be used as Tailwind classes without custom CSS selectors.

#### Scenario: Animation utility usage
- **WHEN** a component needs the message slide-in animation
- **THEN** it SHALL use `animate-msg-slide-in` as a Tailwind utility class backed by an `@utility` directive in `index.css`

### Requirement: Visual output identical before and after
Every migrated component SHALL produce pixel-identical visual output at all three verification breakpoints (375px, 768px, 1280px) for all interactive states (default, hover, active, focus-visible).

#### Scenario: Side-by-side comparison
- **WHEN** screenshots are captured before and after migrating a component
- **THEN** the visual output SHALL be identical at 375px, 768px, and 1280px viewports

### Requirement: Interactive states fully migrated
Every interactive state defined in custom CSS (`:hover`, `:focus`, `:active`, `.--active` modifier, `.--unread` modifier) SHALL have an equivalent Tailwind modifier in JSX.

#### Scenario: Hover state preservation
- **WHEN** a user hovers over a sidebar nav item
- **THEN** the background and text color SHALL change identically to the pre-migration behavior using Tailwind `hover:` modifiers
