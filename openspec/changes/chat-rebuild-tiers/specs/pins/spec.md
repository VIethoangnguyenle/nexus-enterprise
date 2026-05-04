# Pins — Spec

## User Stories

### S1: Pin/Unpin Message
As a workspace member,
I want to pin an important message via hover action bar,
so that the team can find it later.

Acceptance Criteria:
- [ ] HoverActionBar shows Pin icon (unpinned) or PinOff icon (pinned)
- [ ] Clicking Pin calls API and marks message as pinned (optimistic)
- [ ] Pinned messages show a small Pin icon indicator inline
- [ ] Clicking PinOff removes the pin
- [ ] Other users see pin status update via WebSocket `pinEvent`

Proto mapping: `POST /channels/:id/pins` + `DELETE /channels/:id/pins/:messageId` (REST — EXISTS)
WS event: `pinEvent` (EXISTS)
Frontend-only: YES

### S2: View Pinned Messages
As a workspace member,
I want to view all pinned messages in a dedicated tab,
so that I can find important content quickly.

Acceptance Criteria:
- [ ] ChannelInfoPanel "Pins" tab loads pinned messages
- [ ] Each pinned item shows message content, sender, pin time
- [ ] Clicking a pinned message scrolls/navigates to it in chat
- [ ] Empty state: "No pinned messages" with description

Proto mapping: `GET /channels/:id/pins` (REST — EXISTS)
Frontend-only: YES

## Flow

### Pin Message
1. User hovers message → HoverActionBar
2. User clicks Pin icon
3. System calls `POST /channels/:id/pins` { message_id: ":msgId" }
4. Optimistic: message shows pin indicator
5. WS `pinEvent` broadcasts

### View Pins
1. User clicks channel name or info icon → ChannelInfoPanel
2. User clicks "Pins" tab
3. System calls `GET /channels/:id/pins`
4. Renders list of pinned messages

## States
- **Empty**: "No pinned messages" empty state
- **Loading**: Spinner
- **Loaded**: List of pinned messages
- **Error**: Retry
