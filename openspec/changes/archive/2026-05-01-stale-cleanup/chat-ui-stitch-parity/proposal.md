# Chat UI Stitch Design Parity

## What

Align the Chat/Messaging UI exactly to the Stitch design reference. The current implementation has correct structure but differs in several visual and functional details from the Stitch mockup.

## Why

The Stitch design defines the production standard. Current chat UI was built as a functional MVP and needs visual polish to match the design spec exactly.

## Scope

### Frontend Changes (ChatList, ChatEditor, MessageRow)

1. **ChatList header**: "Chats" → "Messages" with filter + create icons
2. **ChatList sections**: Group channels by type → PINNED / DEPARTMENTS / DIRECT MESSAGES
3. **ChatList items**: Show 2-line preview (sender + last message) + timestamp + unread badge + EXTERNAL tag
4. **Channel header**: Add member count + topic tags below channel name, add ⋮ menu
5. **Message bubbles**: Incoming = left-aligned white card, Outgoing = right-aligned blue pill with avatar on right
6. **File attachment cards**: Styled preview card with icon + filename + size + type
7. **Chat editor**: Add "Press Enter to send" hint, make send button circular blue
8. **Timestamp dividers**: "TODAY, 9:00 AM" pill-shaped centered divider

### Backend Changes (Proto + Messaging Service)

9. **Channel proto**: Add `topic`, `description`, `member_count` fields
10. **Starred channels**: New per-user starred/pinned channel concept (maps to PINNED section)

## Out of Scope

- DM creation flow (existing `CreateDM` RPC is sufficient)
- Threaded messages redesign
- Mobile-specific layouts
