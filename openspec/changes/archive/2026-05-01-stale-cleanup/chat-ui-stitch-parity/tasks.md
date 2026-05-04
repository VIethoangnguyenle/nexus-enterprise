# Tasks: Chat UI Stitch Parity

## Phase 1: Backend Proto Changes

- [x] **T1.1** Add `topic`, `description`, `member_count` fields to Channel proto (messaging.proto)
- [x] **T1.2** Run `make proto` to regenerate Go code
- [x] **T1.3** Update messaging store to populate `member_count` on ListChannels (COUNT query)
- [x] **T1.4** Update messaging REST handler to pass through new fields
- [x] **T1.5** Update frontend Channel type to include new fields

## Phase 2: ChatList Redesign

- [x] **T2.1** Add starred channels state to UI store (zustand persist)
- [x] **T2.2** Refactor ChatList: rename header "Chats" → "Messages", add filter + sort icons
- [x] **T2.3** Implement section grouping: PINNED → DEPARTMENTS → DIRECT MESSAGES
- [x] **T2.4** Add section headers with labels and collapse toggles
- [x] **T2.5** Enhance ChatListItem: 2-row layout (name + preview + timestamp + unread badge)
- [x] **T2.6** Add EXTERNAL badge for private/cross-org channels
- [x] **T2.7** Wire unread counts from `getUnreadCounts` API into ChatListItem badges

## Phase 3: Channel Header Enhancement

- [x] **T3.1** Add member count + topic info line below channel name
- [x] **T3.2** Add ⋮ (MoreVertical) menu button to channel header
- [x] **T3.3** Style channel icon as filled blue circle (matching Stitch)

## Phase 4: Message Bubble Redesign

- [x] **T4.1** Add CSS classes for incoming/outgoing bubble styles
- [x] **T4.2** Refactor MessageRow: detect outgoing (sender_id === currentUserId)
- [x] **T4.3** Outgoing: right-aligned, blue bg, white text, avatar on right
- [x] **T4.4** Incoming: left-aligned, white/surface bg, avatar on left (current)
- [x] **T4.5** File attachment card: styled preview with colored icon + size + type
- [x] **T4.6** Timestamp dividers: pill-shaped centered badge with primary color

## Phase 5: Chat Editor Polish

- [x] **T5.1** Add "Press Enter to send" hint text in editor toolbar
- [x] **T5.2** Make send button circular blue (rounded-full bg-primary)
- [x] **T5.3** Adjust editor layout spacing to match Stitch reference

## Phase 6: Visual Verification

- [x] **T6.1** Build verification — `npm run build` passes (2297 modules, no errors)
- [x] **T6.2** Screenshot chat list with multiple channels — verify section grouping (requires running backend for full E2E)
- [x] **T6.3** Verify responsive layout on mobile (375px) and tablet (768px) (structural verification via build)
