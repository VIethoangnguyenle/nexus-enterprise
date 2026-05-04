# Chat Module — Stitch Compliance Spec

## Stitch Reference Screens
- `05-chat-final.png` — Main chat view with sidebar + messages
- `06-chat-messaging.png` — Messaging view with external partner indicator
- `07-chat-empty-states.png` — Empty states & member panel

## Stitch Design Patterns (Source of Truth)

### Chat Sidebar (from Stitch screens 05, 06)
- **App header**: "Nexus Enterprise" with workspace avatar, top-left
- **Search bar**: Full-width search input "Search conversations..."
- **Primary CTA**: Blue button "✚ New Project" — full-width
- **Section labels**: "PINNED", "DEPARTMENTS", "DIRECT MESSAGES" — uppercase, label-caps typography (11px, bold, tracking-widest)
- **Channel items**: Avatar + channel name + preview text + timestamp, right-aligned unread badge
- **Active state**: Blue background highlight on selected channel

### Message Area (from Stitch screen 05)
- **Channel header**: Channel icon + name + member count + topic tags
- **Date separator**: "TODAY, 9:00 AM" centered divider
- **Messages**: Avatar + author name + timestamp, left-aligned for others, right-aligned bubble for self
- **File attachments**: Card with file icon + filename + size
- **Reactions**: Emoji badges below messages
- **Message composer**: Toolbar (B, I, list, grid, table icons) + text input + "Press Enter to send" + send button

### Sidebar Navigation (from Stitch screen 05)
- Items: "Chat", "Drive", "Contacts", "Workplace" — icon + label
- Active state: Blue background pill

---

## User Stories

### US-4: Chat sidebar section headers use consistent typography
As a workspace user,
I want the Chat sidebar section headers (PINNED, DEPARTMENTS, DIRECT MESSAGES) to use the Stitch `label-caps` typography pattern,
so that section labels are consistent across modules.

**Acceptance Criteria:**
- [ ] "PINNED" section header uses `label-caps` style: 11px, font-weight 700, uppercase, letter-spacing 0.05em
- [ ] "DEPARTMENTS" section header uses same `label-caps` style
- [ ] "DIRECT MESSAGES" section header uses same `label-caps` style
- [ ] Section headers match Drive sidebar section headers in styling
- [ ] No raw inline styling for section labels

**Proto mapping:** Frontend-only
**Files affected:** Chat sidebar component (workspace route layout)

### US-5: Chat channel list matches Stitch layout
As a workspace user,
I want the Chat channel list items to follow the Stitch design,
so that channel browsing is visually consistent.

**Acceptance Criteria:**
- [ ] Each channel item shows: avatar + name + preview text + timestamp
- [ ] Active channel has blue/primary background highlight
- [ ] Unread badge appears on right side of channel item
- [ ] Channel items have consistent hover state

**Proto mapping:** Frontend-only
**Files affected:** Channel list components in workspace route

---

## UI States

| State | Description |
|-------|-------------|
| Empty | No channels — "No conversations yet" with CTA |
| Loading | Skeleton while loading channels |
| Loaded | Channel list with messages |
| Error | API failure state |
| No channel selected | Empty message area with prompt |
