## REMOVED Requirements

### Requirement: Hierarchical Sidebar Navigation
**Reason**: Replaced by the LarkRail (90px fixed rail with icon + label) + ResizableListPanel architecture. The 240px sidebar with inline channel lists followed Slack patterns, not Lark patterns.
**Migration**: Use `lark-rail-navigation` spec for the new rail and `chat-list-panel` spec for the conversation list.

### Requirement: Sidebar Collapse
**Reason**: LarkRail is fixed-width and does not collapse. The resizable element is now the ListPanel, not the rail.
**Migration**: ListPanel resize (200–480px) replaces sidebar collapse behavior.

### Requirement: Sidebar Global Search
**Reason**: Search moves to the LarkRail as a `Ctrl+K` shortcut indicator, and a full search input appears at the top of the ListPanel.
**Migration**: See `lark-rail-navigation` and `chat-list-panel` specs.
