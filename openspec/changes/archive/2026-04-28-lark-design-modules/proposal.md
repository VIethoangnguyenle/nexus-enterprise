## Why

The NGAC platform requires a transition from its legacy consumer-grade UI to a high-density, enterprise-grade interface inspired by Lark. Following the completion of Phase 1 (Design Foundation), the platform now has the correct color tokens, spacing primitives, and border radii. However, the core layout (sidebar) and individual modules (Messaging, Drive, Assets) still rely on legacy layouts that lack information density and professional structuring. This change implements the remaining phases of the Lark UI migration.

## What Changes

- **Phase 2: Sidebar Migration**: Replace the 48px rail with a resizable/collapsible ~200px sidebar featuring search, hierarchical navigation, and user context.
- **Phase 3: Messaging Module**: Migrate the chat view to a dense layout, introducing thread replies in a right-hand panel, and removing chat bubble gradients.
- **Phase 4: Drive Module**: Redesign the Drive module with a data-table view for files, breadcrumb navigation, and flat, border-based visual boundaries instead of large shadows.
- **Phase 5: Assets Module**: Standardize the Assets module using a data-table view, flat filter bars, and side peek panels for details instead of centered modal popups.

## Capabilities

### New Capabilities
- `lark-sidebar-layout`: Define requirements for the expanded, hierarchical sidebar layout and navigation.
- `lark-messaging-layout`: Define requirements for the dense messaging view and side-panel thread layout.
- `lark-data-table-layout`: Define requirements for the standard enterprise data-table view used in Drive and Assets.

### Modified Capabilities
- `lark-design-tokens`: Extension of the design system foundation to cover complex component layouts like data tables and side peek panels.

## Impact

- **Frontend Navigation (`_workspace.tsx`, `AppRail.tsx`)**: Significant architectural change from Rail to full Sidebar.
- **Messaging Module (`routes/_workspace/channels.$channelId.tsx`)**: Layout rewrite to support side-panel threading and dense message lists.
- **Drive & Assets Modules**: Transition from grid/card layouts to structured data tables.
- **State Management**: Zustand stores may require updates to handle sidebar collapse state and side peek panel state.
