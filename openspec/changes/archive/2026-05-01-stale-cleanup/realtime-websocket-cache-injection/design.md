# Design: Platform-Wide WebSocket Cache Injection

## UX Assessment

This change is a **data-flow refactor** — it affects how data flows from WebSocket events into the UI state, NOT the visual design. There are no new screens, components, or layouts.

## Visual Impact: None

All changes are in:
- Hook logic (`useMessaging.ts`, `useDrive.ts`)
- State management (`websocket.store.ts`)

The user sees the **same UI** but with:
- Instant reaction/pin/vote/task updates (was: 200-500ms refetch delay)
- No loading flash between mutations (was: brief skeleton/loading state during refetch)
- Sub-100ms real-time updates from other users

## No Stitch Design Required

No new screens or components needed. Existing UI components already render the data correctly — we're only changing how that data enters the cache.

## Component Mapping: N/A

All affected components already exist and render correctly with current data shapes. The data contract (Message, ReactionGroup, PinnedMessage, Poll, ChatTask) is unchanged.
