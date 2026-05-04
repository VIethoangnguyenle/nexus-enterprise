# Tasks — Lark Chat Parity

## Phase 1: Database & Proto Foundation
> New tables, proto updates, migration script

- [x] **T1.1** Create migration SQL: `message_reactions`, `message_pins`, `read_receipts` tables
- [x] **T1.2** Create migration SQL: `polls`, `poll_options`, `poll_votes` tables
- [x] **T1.3** Create migration SQL: `chat_tasks` table
- [x] **T1.4** ALTER `messages`: add `search_vector` (tsvector), `content_format`, `mentions` columns
- [x] **T1.5** Update `messaging.proto`: add Reaction, Pin, ReadReceipt, Poll, Task RPCs + messages
- [x] **T1.6** Update `ws.proto`: add ReactionEvent, PinEvent, ReadEvent, PollVoteEvent, TaskUpdateEvent
- [x] **T1.7** Run `make proto` to regenerate Go + TS protobuf
- [x] **T1.8** Apply migration to database, drop old message data

---

## Phase 2: NGAC & Auth Backend
> Auto-create workspace, fix NGAC naming

- [x] **T2.1** Refactor `ngac/` package: all naming functions use workspace ID instead of name
- [x] **T2.2** Update `workspace/internal/grpc/server.go` CreateWorkspace: pass ID to ngac naming
- [x] **T2.3** Add `messaging.CreateChannel` gRPC call support to auth service (new gRPC client)
- [x] **T2.4** Update `auth/internal/domain/service.go` Register: after user creation → gRPC create workspace → gRPC create #general channel
- [x] **T2.5** Update `auth/cmd/main.go`: wire workspace + messaging gRPC clients
- [x] **T2.6** Update `auth/Dockerfile`: add proto dependencies for workspace + messaging
- [x] **T2.7** Verify: register → auto workspace + #general channel created

---

## Phase 3: Messaging Backend — Reactions, Pins, Mentions
> Store + Domain + REST + WebSocket for core chat features

- [x] **T3.1** Create `messaging/internal/store/reactions.go`: InsertReaction, DeleteReaction, ListByMessage
- [x] **T3.2** Create `messaging/internal/store/pins.go`: InsertPin, DeletePin, ListByChannel
- [x] **T3.3** Create `messaging/internal/store/read_receipts.go`: Upsert, GetUnreadCount, GetLastRead
- [x] **T3.4** Update `messaging/internal/domain/service.go`: AddReaction, RemoveReaction, TogglePin, MarkRead, GetUnreadCounts
- [x] **T3.5** Add mention parsing in domain: extract `@username` from markdown → populate `mentions` column → create notifications
- [x] **T3.6** Update `messaging/internal/rest/handler.go`: reaction, pin, read receipt, search endpoints
- [x] **T3.7** Add full-text search: `SearchMessages(channelID, query, limit)` in store using `search_vector`
- [x] **T3.8** Update WebSocket hub: broadcast ReactionEvent, PinEvent, ReadEvent on mutations
- [x] **T3.9** Update `GET /api/channels/:id/messages` response: include reactions[] and pins for each message
- [x] **T3.10** Add `GET /api/channels/:id/members` REST endpoint (delegate to existing gRPC)
- [x] **T3.11** Verify: reaction CRUD, pin CRUD, read receipt, search all working via curl

---

## Phase 4: Messaging Backend — Polls & Tasks
> New subsystems embedded in chat

- [x] **T4.1** Create `messaging/internal/store/polls.go`: InsertPoll, InsertOption, InsertVote, DeleteVote, GetPollResults
- [x] **T4.2** Create `messaging/internal/store/tasks.go`: InsertTask, UpdateTask, ListByChannel, ListByAssignee
- [x] **T4.3** Update `messaging/internal/domain/service.go`: CreatePoll, VotePoll, CreateTask, UpdateTaskStatus
- [x] **T4.4** Update `messaging/internal/rest/handler.go`: poll + task REST endpoints
- [x] **T4.5** Update WebSocket hub: broadcast PollVoteEvent, TaskUpdateEvent
- [x] **T4.6** Verify: create poll, vote, create task, update status all working

---

## Phase 5: Frontend — TipTap Rich Text Editor
> Replace textarea with contentEditable editor

- [x] **T5.1** Install TipTap: `@tiptap/react`, `@tiptap/starter-kit`, `@tiptap/extension-mention`, `@tiptap/extension-placeholder`
- [x] **T5.2** Install emoji-mart: `@emoji-mart/react`, `@emoji-mart/data`
- [x] **T5.3** Install markdown renderer: `react-markdown`, `remark-gfm`
- [x] **T5.4** Create `components/chat/ChatEditor.tsx`: TipTap editor with markdown serialization
- [x] **T5.5** Create `components/chat/EditorToolbar.tsx`: format buttons (B, I, S, code, quote, list)
- [x] **T5.6** Create `components/chat/EmojiPicker.tsx`: emoji-mart popup, insert into editor
- [x] **T5.7** Create `components/chat/MentionList.tsx`: @mention autocomplete dropdown
- [x] **T5.8** Create `components/chat/AttachMenu.tsx`: attachment popup (Image, File, Poll, Task)
- [x] **T5.9** Integrate ChatEditor into channel chat view, replace ChatInput
- [x] **T5.10** Verify: rich text editing, emoji insert, @mention, file attach all working in browser

---

## Phase 6: Frontend — Message Redesign
> MessageItem with timestamps, reactions, hover actions, markdown render

- [x] **T6.1** Create `components/chat/MessageContent.tsx`: render markdown content with react-markdown + mention highlighting
- [x] **T6.2** Create `components/chat/ReactionBar.tsx`: display reaction chips [👍 3], add reaction button
- [x] **T6.3** Create `components/chat/HoverActionBar.tsx`: reply, react, pin, more actions (visible on hover)
- [x] **T6.4** Create `components/chat/PollCard.tsx`: poll display with vote buttons, results bar
- [x] **T6.5** Create `components/chat/TaskCard.tsx`: task display with status toggle, assignee
- [x] **T6.6** Redesign `MessageItem.tsx`: compose MessageContent + ReactionBar + HoverActionBar + timestamp
- [x] **T6.7** Create `hooks/useReactions.ts`: TanStack Query hooks for reaction CRUD
- [x] **T6.8** Create `hooks/usePolls.ts`: TanStack Query hooks for poll/vote
- [x] **T6.9** Create `hooks/useTasks.ts`: TanStack Query hooks for task CRUD
- [x] **T6.10** Update WebSocket store: handle reaction, pin, poll, task events → invalidate queries
- [x] **T6.11** Verify: messages render markdown, reactions work, hover actions work, polls/tasks display

---

## Phase 7: Frontend — Channel Header & Right Panel
> Lark-style header + tabbed info panel

- [x] **T7.1** Redesign channel header: channel name, member count, search icon, pin icon, settings icon
- [x] **T7.2** Create `components/chat/ChannelInfoPanel.tsx`: tabbed container with close button
- [x] **T7.3** Create `components/chat/tabs/SettingsTab.tsx`: channel name, description, notification prefs
- [x] **T7.4** Create `components/chat/tabs/MembersTab.tsx`: member list with roles, online status
- [x] **T7.5** Create `components/chat/tabs/PinnedTab.tsx`: list of pinned messages
- [x] **T7.6** Create `components/chat/tabs/FilesTab.tsx`: refactor existing ChannelDrivePanel
- [x] **T7.7** Create `components/chat/tabs/SearchTab.tsx`: search input + message results
- [x] **T7.8** Wire header buttons to toggle right panel tabs
- [x] **T7.9** Thread panel becomes overlay within right panel
- [x] **T7.10** Verify: all 5 tabs render, header buttons toggle correctly

---

## Phase 8: Frontend — Left Panel & Read Receipts
> Unified chat list, unread badges, DM support

- [x] **T8.1** Redesign `ListPanel` channel list: add search bar, last message preview, timestamp
- [x] **T8.2** Sort channels by last message time (most recent first)
- [x] **T8.3** Create `hooks/useReadReceipts.ts`: unread count per channel
- [x] **T8.4** Add unread badge (red dot + count) to channel list items
- [x] **T8.5** Auto mark-as-read when user views channel (debounced POST)
- [x] **T8.6** Add DM conversations to chat list (alongside channels)
- [x] **T8.7** Update WebSocket store: handle ReadEvent → update unread counts
- [x] **T8.8** Verify: unread badges appear, auto-clear on view, DMs in list

---

## Phase 9: Auto-Create Workspace & Onboarding
> Seamless post-registration experience

- [x] **T9.1** Remove `CreateWorkspaceCard` from workspace layout
- [x] **T9.2** Update register flow: after success → redirect directly to workspace/channels
- [x] **T9.3** Add welcome message in #general channel (system message from backend)
- [x] **T9.4** Verify: fresh register → land in #general with welcome message, no manual workspace creation

---

## Phase 10: Integration & Polish
> End-to-end testing, animations, responsive

- [x] **T10.1** Full smoke test: register → auto workspace → #general → send rich message → react → @mention → pin → search → poll → task
- [x] **T10.2** WebSocket reconnection: ensure all new events survive reconnect
- [x] **T10.3** Add micro-animations: reaction pop, message slide-in, panel transitions
- [x] **T10.4** Mobile responsive: editor toolbar collapses, panels overlay
- [x] **T10.5** Performance: virtualize message list for channels with many messages
- [x] **T10.6** Final UI review against Lark screenshots

---

## Summary

| Phase | Tasks | Focus |
|-------|-------|-------|
| 1 | 8 | DB + Proto |
| 2 | 7 | Auth + NGAC |
| 3 | 11 | Reactions, Pins, Search, Read Receipts |
| 4 | 6 | Polls, Tasks |
| 5 | 10 | TipTap Editor |
| 6 | 11 | Message Redesign |
| 7 | 10 | Header + Right Panel |
| 8 | 8 | Left Panel + Unread |
| 9 | 4 | Auto Workspace |
| 10 | 6 | Polish |
| **Total** | **81** | |
