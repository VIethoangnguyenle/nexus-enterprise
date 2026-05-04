# Design: Chat UI Stitch Parity

## Architecture Decision

Frontend-heavy change. Backend proto additions are minimal (3 new fields on Channel). No new services or RPCs needed.

## Data Model Changes

### Channel Proto (messaging.proto)

Add 3 fields to existing `Channel` message:
```protobuf
message Channel {
  // ... existing fields 1-8 ...
  string topic = 9;           // Short topic/description line
  string description = 10;    // Longer channel description
  int32 member_count = 11;    // Denormalized count
}
```

### Starred Channels

For the "PINNED" section in Stitch, implement a user-level starred channels concept.

**Option chosen**: Frontend-only localStorage approach for MVP. Channel starring doesn't need backend persistence — users can star channels locally and it persists via Zustand persist middleware.

This avoids proto changes and new DB tables. Can upgrade to server-side later if needed (e.g., sync across devices).

## Frontend Component Changes

### 1. ChatList Refactor

**Current**: Flat list with header "Chats"
**Target**: Grouped sections with header "Messages"

```
Messages                    ⊕ 🔽
─────────────────────────────────
── PINNED ──
📌 Stars Industries  10:42 AM
   EXTERNAL
   Tony: The revised Q3...

── DEPARTMENTS ──
█ Marketing Sync      Just now
  Sarah: Design assets...

🔵 Engineering        Yesterday
  Deploy scheduled for...

── DIRECT MESSAGES ──
🟢 Elena Rodriguez    Mon
  Can you review...
```

Group by:
- **PINNED**: starred channels (from localStorage/zustand)
- **DEPARTMENTS**: `channel_type === "workspace"`
- **DIRECT MESSAGES**: `channel_type === "dm"`

### 2. ChatListItem Enhancement

Current: Avatar + name only
Target: 2-row layout with preview + timestamp + badge

```
┌─ ChatListItem ────────────────────┐
│ [Avatar] Channel Name    10:42 AM │
│          EXTERNAL                 │
│          Tony: The revised Q3...  │
│                              [3]  │
└───────────────────────────────────┘
```

- Show `EXTERNAL` badge for channels where `channel_type === "private"` (cross-org)
- Show last message preview from WebSocket store `lastMessages`
- Show unread badge from `getUnreadCounts`
- Show relative timestamp

### 3. Channel Header Enhancement

Current: `# channelName` + action buttons + tabs
Target: Add member info line below channel name

```
┌─ Header ──────────────────────────────────┐
│ 🔵 Marketing Sync          🔍 👤 📁  ⋮  │
│ 👤 12 Members · Design, Content, Strategy │
├───────────────────────────────────────────┤
│ [Chat] [📌 Pinned] [📁 Files]            │
└───────────────────────────────────────────┘
```

- Member count from `member_count` field (or fallback to `listMembers` count)
- Topic from `topic` field
- Add `⋮` (MoreVertical) menu button

### 4. Message Bubble Redesign

Current: All messages left-aligned, no bubble styling
Target: Stitch-style with directional bubbles

**Incoming** (other users):
```
👤 Michael Chen · 9:05
Morning team. I've uploaded the initial
drafts for the Q5 campaign...

  ┌─────────────────────────┐
  │ 📁 Q5_Campaign_V1fg     │
  │ 4.2 MB · Figma Design   │
  └─────────────────────────┘
😃 2
```

**Outgoing** (current user):
```
                         9:12 AM Yeah!
  ┌────────────────────────────────┐ 👤
  │ Thanks Michael. Reviewing now. │
  │ The hero section looks cleaner │
  └────────────────────────────────┘
```

Key differences:
- Outgoing: right-aligned, blue background (`bg-primary`), white text, avatar on RIGHT
- Incoming: left-aligned, white/surface background, normal text, avatar on LEFT
- Timestamp above outgoing messages (right-aligned)
- File cards: styled card with colored icon, filename, size, app type

### 5. Chat Editor Enhancement

Current: Toolbar + emoji + file + send
Target: Add "Press Enter to send" hint

```
┌─ Editor ──────────────────────────────────┐
│ B I Z ⇄ ≡ □ ···                          │
│ Type a message...                         │
│                          😊 📎  Press ↵  🔵│
└───────────────────────────────────────────┘
```

- "Press Enter to send" text hint on the right side of toolbar
- Send button: circular blue (`rounded-full bg-primary w-8 h-8`)

### 6. Timestamp Dividers

Current: Simple centered text
Target: Pill-shaped badge

```
          ┌─────────────────┐
          │ TODAY, 9:00 AM  │
          └─────────────────┘
```

- `bg-primary/10 text-primary rounded-full px-3 py-1 text-micro`

## CSS Token Additions

New CSS classes needed in `index.css`:
- `.msg-bubble--outgoing`: right-aligned blue bubble
- `.msg-bubble--incoming`: left-aligned white/surface bubble
- `.chat-section-label`: section header for PINNED/DEPARTMENTS/DM groups
- `.chat-timestamp-pill`: pill-shaped timestamp divider
- `.chat-external-badge`: small EXTERNAL tag
