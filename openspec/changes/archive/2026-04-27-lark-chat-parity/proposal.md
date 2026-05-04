# Lark Chat Parity — Full Messaging Redesign

## Problem

The current NGAC messaging UI is a minimal MVP:
- Plain textarea input (no emoji, no @mention, no formatting)
- No emoji reactions on messages
- No message pins, no read receipts, no unread badges
- No message search
- Right panel limited to Thread or Files only
- Workspace requires manual creation after registration
- NGAC node naming uses workspace name (collision risk)
- No polls or tasks in chat
- Overall UI feels sparse and unfinished compared to Lark

## Goal

Rebuild the entire messaging experience to **Lark parity**: a production-grade chat platform with rich text editing, emoji reactions, @mentions, file sharing, polls, tasks, read receipts, message search, and a complete channel info panel.

## Scope

### In Scope

1. **Rich Text Editor** — Replace textarea with TipTap (contentEditable), markdown storage
2. **Emoji Picker** — emoji-mart library, insert into editor
3. **Emoji Reactions** — Backend CRUD + WebSocket events + UI reaction chips
4. **@Mention** — Autocomplete popup, parse to notification, highlight in messages
5. **Message Pins** — Pin/unpin API + pinned messages panel
6. **Read Receipts** — Per-user-per-channel last-read tracking + unread count + badges
7. **Message Search** — PostgreSQL `tsvector` full-text search + search UI
8. **Polls** — Create poll in chat, vote, see results inline
9. **Tasks** — Create task in chat, assign, mark done inline
10. **Right Panel Redesign** — Tabbed: Settings, Members, Pinned, Files, Search
11. **Channel Header Redesign** — Member count, search, pin, settings buttons
12. **Message Item Redesign** — Timestamps, hover action bar, reaction chips, mention highlight
13. **Left Panel Redesign** — Unified chat list (channels + DMs), unread badges, last message preview
14. **Auto-Create Workspace** — On register, create `<username>'s Workspace` + #general channel
15. **NGAC Naming** — Use workspace ID instead of name for policy node naming
16. **Data Migration** — Drop old message data, start fresh with markdown format

### Out of Scope

- Video/audio calls
- Message forwarding to other channels
- Bot framework
- Message scheduling
- Custom emoji upload

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Rich text storage | Markdown | Lightweight, renderable, diff-friendly |
| Editor library | TipTap | Best contentEditable lib for React, extensible |
| Emoji library | emoji-mart | Industry standard, lightweight, customizable |
| Old data | Drop & rebuild | Clean slate, no migration complexity |
| Workspace naming | ID-based NGAC nodes | Prevents name collision, allows rename |

## Phases

### Phase 1: Backend Foundation
- DB schema: `message_reactions`, `message_pins`, `read_receipts`, `polls`, `poll_votes`, `tasks`
- Messaging service: reactions, pins, mentions, read receipt APIs
- Auth service: auto-create workspace on register
- NGAC naming refactor

### Phase 2: Frontend Chat Core
- TipTap rich text editor with emoji-mart + @mention
- MessageItem redesign (timestamps, hover actions, reactions, markdown render)
- Channel header redesign
- File attachment menu (Image, File from Lark screenshot)

### Phase 3: Panels & Search
- Right panel with 5 tabs (Settings, Members, Pinned, Files, Search)
- Left panel with unified chat list + unread badges
- Full-text message search

### Phase 4: Polls & Tasks
- Poll creation + voting UI
- Task creation + assignment + status UI
- WebSocket events for real-time updates

### Phase 5: Read Receipts & Polish
- Read receipt tracking + display
- Unread count badges
- Typing indicators enhancement
- Overall UI polish and animation

## Risk

| Risk | Mitigation |
|------|-----------|
| TipTap bundle size | Tree-shake, lazy load editor |
| Read receipt write volume | Debounce, batch upserts |
| Full-text search perf | GIN index on tsvector, LIMIT results |
| Breaking existing data | Explicit decision: drop old data |
| NGAC node rename | One-time migration script |
