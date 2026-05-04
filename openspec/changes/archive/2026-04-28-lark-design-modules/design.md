## Context

The NGAC platform is moving to an enterprise-grade UI heavily inspired by Lark. Following the application of core design tokens (colors, borders, spacing), we need to refactor complex modules (Sidebar layout, Messaging, Drive, Assets) to use standard enterprise layout patterns like resizable sidebars, data tables, and side peek panels. The current UI is heavily grid-based and uses large, disruptive components (modals) and non-standard navigation (icon-only rail).

## Goals / Non-Goals

**Goals:**
- Implement a hierarchical, expandable sidebar (`w-64`) replacing the `w-12` rail.
- Migrate Messaging to a dense thread layout (Lark-style chat).
- Migrate Drive and Assets to data-table views instead of grid cards.
- Implement side peek panels (slide-out from right) for details instead of full-screen or centered modals.

**Non-Goals:**
- Changes to the backend API or data structures.
- Redesigning the entire policy/auth flow (these remain in their dedicated UI spaces or headless).

## Decisions

- **Sidebar Component Rewrite**: Instead of iterating on `AppRail.tsx`, we will create a new `Sidebar.tsx` component that includes the workspace switcher, hierarchical navigation, and a search input. The layout root (`_workspace.tsx`) will be adjusted to allocate space for this.
- **Data Table Standardization**: Drive and Assets will use a common `DataTable` compound component structure, allowing us to enforce a single source of truth for borders, row hover states (`bg-bg-hover`), and row heights.
- **Side Peek Panels**: We will introduce a `PeekPanel.tsx` component that slides in from the right (`animate-slide-left`) over the main content area, providing detailed views (e.g., file details, asset metadata) without losing context of the list underneath.

## Risks / Trade-offs

- **Risk: Component Bloat in Sidebar** → *Mitigation*: The sidebar will be split into logical sections (WorkspaceHeader, NavList, UserProfile) to maintain maintainability.
- **Risk: Data Table Performance** → *Mitigation*: If lists grow large, we may need virtualization, but for now standard React rendering with proper keys will suffice.
- **Trade-off**: Dropping grid views entirely in Drive/Assets. Data tables offer better information density, which aligns with our enterprise goals, but removes the visual preview aspect of large grid cards.
