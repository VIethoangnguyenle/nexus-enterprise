# Design — Lark Chat Parity

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                          Frontend (Vite SPA)                        │
│                                                                     │
│  ┌──────────┐  ┌──────────────┐  ┌──────────────────────────────┐  │
│  │ AppRail  │  │  ChatList    │  │  ChatView                    │  │
│  │ (48px)   │  │  (280px)     │  │  ┌────────────────┐ ┌──────┐│  │
│  │          │  │ ┌──────────┐ │  │  │ Channel Header │ │Right ││  │
│  │ 💬 Chat  │  │ │ Search   │ │  │  ├────────────────┤ │Panel ││  │
│  │ 📄 Docs  │  │ ├──────────┤ │  │  │                │ │      ││  │
│  │ 💾 Drive │  │ │ #general │ │  │  │  Message List  │ │ Tabs ││  │
│  │ 📦 Asset │  │ │ #random  │ │  │  │  (virtual)     │ │ Set  ││  │
│  │ ⚙️ Set   │  │ │ @john DM │ │  │  │                │ │ Mem  ││  │
│  │          │  │ │ @jane DM │ │  │  ├────────────────┤ │ Pin  ││  │
│  │          │  │ │          │ │  │  │ TipTap Editor  │ │ File ││  │
│  │          │  │ │ 🔴 2     │ │  │  │ [B I S] 😀 @ +│ │ Srch ││  │
│  │  👤      │  │ └──────────┘ │  │  └────────────────┘ └──────┘│  │
│  └──────────┘  └──────────────┘  └──────────────────────────────┘  │
└────────────────────────────────────────────────────────────────────-┘
```

## Database Schema Changes

### New Tables

```sql
-- Emoji reactions on messages
CREATE TABLE message_reactions (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id),
    emoji       TEXT NOT NULL,           -- unicode emoji e.g. "👍" "🎉"
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(message_id, user_id, emoji)   -- one reaction per emoji per user
);
CREATE INDEX idx_reactions_message ON message_reactions(message_id);

-- Pinned messages per channel
CREATE TABLE message_pins (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    channel_id  TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    pinned_by   TEXT NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(channel_id, message_id)
);
CREATE INDEX idx_pins_channel ON message_pins(channel_id);

-- Read receipt tracking per user per channel
CREATE TABLE read_receipts (
    user_id         TEXT NOT NULL REFERENCES users(id),
    channel_id      TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    last_read_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_message_id TEXT REFERENCES messages(id),
    PRIMARY KEY (user_id, channel_id)
);

-- Polls embedded in messages
CREATE TABLE polls (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    question    TEXT NOT NULL,
    is_multi    BOOLEAN DEFAULT FALSE,   -- allow multiple choices
    is_anonymous BOOLEAN DEFAULT FALSE,
    ends_at     TIMESTAMPTZ,             -- optional deadline
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE poll_options (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    poll_id     TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    text        TEXT NOT NULL,
    position    INT NOT NULL DEFAULT 0
);
CREATE INDEX idx_poll_options_poll ON poll_options(poll_id);

CREATE TABLE poll_votes (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    poll_id     TEXT NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    option_id   TEXT NOT NULL REFERENCES poll_options(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(poll_id, option_id, user_id)
);

-- Tasks embedded in messages
CREATE TABLE chat_tasks (
    id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    channel_id  TEXT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    assignee_id TEXT REFERENCES users(id),
    status      TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'done')),
    due_date    DATE,
    created_by  TEXT NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_tasks_channel ON chat_tasks(channel_id, status);
CREATE INDEX idx_tasks_assignee ON chat_tasks(assignee_id, status);
```

### Existing Table Changes

```sql
-- Add full-text search vector to messages
ALTER TABLE messages ADD COLUMN search_vector tsvector
    GENERATED ALWAYS AS (to_tsvector('english', content)) STORED;
CREATE INDEX idx_messages_search ON messages USING GIN(search_vector);

-- Add markdown content type marker
ALTER TABLE messages ADD COLUMN content_format TEXT DEFAULT 'markdown'
    CHECK (content_format IN ('plain', 'markdown'));

-- Add mention metadata
ALTER TABLE messages ADD COLUMN mentions TEXT[] DEFAULT '{}';
```

## Messaging Service API Design

### New REST Endpoints

```
# Reactions
POST   /api/channels/:id/messages/:msgId/reactions     { emoji: "👍" }
DELETE /api/channels/:id/messages/:msgId/reactions/:emoji

# Pins
POST   /api/channels/:id/pins                          { message_id: "..." }
DELETE /api/channels/:id/pins/:messageId
GET    /api/channels/:id/pins

# Read Receipts
POST   /api/channels/:id/read                          { message_id: "..." }
GET    /api/channels/:id/unread-count

# Search
GET    /api/channels/:id/messages/search?q=keyword&limit=20

# Polls
POST   /api/channels/:id/polls    { question, options[], is_multi, ends_at }
POST   /api/polls/:id/vote        { option_id }
DELETE /api/polls/:id/vote/:optionId

# Tasks
POST   /api/channels/:id/tasks    { title, assignee_id, due_date }
PATCH  /api/tasks/:id             { status, assignee_id }
GET    /api/channels/:id/tasks

# Members
GET    /api/channels/:id/members
```

### WebSocket Events (New)

```protobuf
// Add to ws.proto ServerEnvelope.payload:
ReactionEvent reaction_event = 10;
PinEvent      pin_event = 11;
ReadEvent     read_event = 12;
PollVoteEvent poll_vote_event = 13;
TaskUpdateEvent task_update_event = 14;

message ReactionEvent {
    string message_id = 1;
    string user_id = 2;
    string username = 3;
    string emoji = 4;
    string action = 5;  // "add" | "remove"
}

message PinEvent {
    string channel_id = 1;
    string message_id = 2;
    string action = 3;  // "pin" | "unpin"
}

message ReadEvent {
    string channel_id = 1;
    string user_id = 2;
    string last_message_id = 3;
}

message PollVoteEvent {
    string poll_id = 1;
    string option_id = 2;
    int32 vote_count = 3;
}

message TaskUpdateEvent {
    string task_id = 1;
    string status = 2;
    string assignee_id = 3;
}
```

## Frontend Component Architecture

### Rich Text Editor (TipTap)

```
ChatEditor (replaces ChatInput)
├── TipTap Editor (contentEditable)
│   ├── StarterKit (bold, italic, strike, code, blockquote, lists)
│   ├── Mention extension (@user autocomplete)
│   ├── Emoji extension (inline emoji via emoji-mart)
│   └── Placeholder extension
├── Toolbar
│   ├── Format buttons [Aa] [B] [I] [S] [</>]
│   ├── Emoji picker button [😀] → emoji-mart popup
│   ├── Mention button [@] → triggers autocomplete
│   ├── Attach menu [+] → Image, File, Poll, Task popup
│   └── Send button [➤]
└── Attachment preview row (before send)
```

### MessageItem Redesign

```
MessageItem
├── Avatar (grouped logic stays)
├── Header: sender name + timestamp
├── Content: rendered markdown (react-markdown)
│   ├── @mention spans (highlighted, clickable)
│   ├── Inline code / code blocks
│   └── Links with preview
├── Attachments
│   ├── FilePreviewCard (existing)
│   ├── PollCard (new)
│   └── TaskCard (new)
├── ReactionBar
│   ├── Reaction chips [👍 3] [🎉 1] 
│   └── Add reaction button [+😀]
└── HoverActionBar (visible on hover)
    ├── 😀 React
    ├── 💬 Reply
    ├── 📌 Pin
    └── ⋯ More (edit, delete, forward)
```

### Right Panel (Tabbed)

```
ChannelInfoPanel (replaces ThreadPanel position)
├── Header: Channel name + close button
├── Tab bar: [Settings] [Members] [Pinned] [Files] [Search]
├── Tab content:
│   ├── SettingsTab: name, description, notification prefs
│   ├── MembersTab: member list with roles, invite button
│   ├── PinnedTab: list of pinned messages
│   ├── FilesTab: existing ChannelDrivePanel content
│   └── SearchTab: search input + results list
└── Thread sub-panel (overlays when viewing thread)
```

### Auto-Create Workspace Flow

```
Current:  Register → Empty → CreateWorkspaceCard → Manual create
New:      Register → Backend auto-creates workspace + #general → Redirect to #general

Auth.Register() {
  1. Create user in DB
  2. Create NGAC user node
  3. gRPC → Workspace.CreateWorkspace(name: "<username>'s Workspace")
  4. gRPC → Messaging.CreateChannel(workspace_id, name: "general")
  5. Return JWT token
}
```

### NGAC Naming Refactor

```go
// BEFORE (collision risk):
func PCName(wsName string) string { return fmt.Sprintf("PC_%s", wsName) }

// AFTER (UUID-based, safe):
func PCName(wsID string) string { return fmt.Sprintf("PC_%s", wsID) }
// All downstream: OwnersUAName, MembersUAName, etc. use wsID
```

## Key Libraries

| Library | Purpose | Size |
|---------|---------|------|
| `@tiptap/react` | Rich text editor | ~50KB |
| `@tiptap/starter-kit` | Bold, italic, etc. | ~30KB |
| `@tiptap/extension-mention` | @mention autocomplete | ~5KB |
| `@emoji-mart/react` | Emoji picker | ~40KB |
| `react-markdown` | Render markdown in messages | ~15KB |
| `remark-gfm` | GitHub-flavored markdown | ~5KB |
