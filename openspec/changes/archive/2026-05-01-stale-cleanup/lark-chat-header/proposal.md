## Why

The current chat header is a single thin row (`#ChannelName [Chat] [Pinned] | Search Members Files`) — functional but visually flat and not matching the Lark enterprise messaging pattern. Lark uses a 2-row header with a prominent identity section (avatar + name + subtitle) plus a tab bar below. This change brings the chat header in line with Lark's design to improve visual hierarchy and navigation clarity.

## What Changes

- **MODIFY**: Channel chat header in `channels.$channelId.tsx` — split into 2 rows:
  - **Row 1 (Identity)**: Channel avatar/icon + channel name (larger) + member count + action icons (Search, Members, Files, Settings)
  - **Row 2 (Tabs)**: Pill tab bar (Chat, Docs, Files, +) matching Lark's tab-style navigation
- **MODIFY**: `index.css` — add new CSS classes for the 2-row header layout
- **No backend changes** — pure frontend layout/UX refactor

## Capabilities

### Modified Capabilities
- `chat-header-layout`: Redesign from 1-row flat bar to 2-row Lark-style header with identity section + tab bar

## Impact

- **Components**: `channels.$channelId.tsx` — header section refactored
- **CSS**: New `.chat-header-identity` and `.chat-header-tabs` styles
- **No route changes, no store changes, no API changes**
