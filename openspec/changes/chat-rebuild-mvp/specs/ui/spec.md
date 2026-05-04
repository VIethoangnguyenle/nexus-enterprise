# UI Rebuild — TIER 1 Spec

## Design System Reference

Source of truth: `.stitch/DESIGN.md`
- Font: Manrope, light-mode M3 tokens, `#2563EB` primary
- Layout: 4-column workspace (Rail + ListPanel + Content + PeekPanel)

## Existing Components to Redesign

| Component | Path | Issue |
|-----------|------|-------|
| ChatList | `components/patterns/ChatList.tsx` | 13KB — oversized, mixed concerns |
| ChatListItem | `components/patterns/ChatListItem.tsx` | 3.5KB — needs last message preview |
| MessageItem | `components/patterns/MessageItem.tsx` | 5.6KB — needs grouping, hover actions |
| ChatEditor | `components/patterns/ChatInput.tsx` | 5KB — rename to ChatEditor, polish |
| ListPanel | `components/patterns/ListPanel.tsx` | 3.8KB — generic, needs chat-specific config |

## Primitives Available

| Component | Path | Reuse? |
|-----------|------|--------|
| Avatar | `primitives/Avatar.tsx` | ✅ Use for user avatars |
| Badge | `primitives/Badge.tsx` | ✅ Use for unread counts |
| Button | `primitives/Button.tsx` | ✅ Use for actions |
| IconButton | `primitives/IconButton.tsx` | ✅ Use for toolbar |
| Input | `primitives/Input.tsx` | ✅ Use for search/rename |
| Spinner | `primitives/Spinner.tsx` | ✅ Use for loading |
| Text | `primitives/Text.tsx` | ✅ Use for labels |
| Heading | `primitives/Heading.tsx` | ✅ Use for panel titles |

---

## User Stories

### US-13: Chat List Redesign

As a **workspace member**,
I want to **see a clean, Lark-quality conversation list**,
so that **I can quickly scan and select conversations**.

Acceptance Criteria:
- [ ] Each item: avatar (group icon or user), name, last message preview, timestamp
- [ ] DM items show other user's name + avatar
- [ ] Group items show channel name + member count
- [ ] Active conversation highlighted with primary color
- [ ] Unread conversations: bold name + unread badge
- [ ] Timestamps: "Just now", "5m", "1h", "Yesterday", date
- [ ] Hover: subtle background change
- [ ] Sorted by last activity (most recent first)
- [ ] Create channel FAB/button at top

### US-14: Message Display Redesign

As a **workspace member**,
I want to **see messages in a clean, grouped layout**,
so that **conversations are easy to read**.

Acceptance Criteria:
- [ ] Consecutive messages from same sender: grouped (no repeat avatar/name)
- [ ] First message in group: shows avatar + name + timestamp
- [ ] Subsequent messages: content only, smaller margin
- [ ] Date separators between different days
- [ ] System messages (join/leave): centered, muted style
- [ ] Long messages: word-wrap, no horizontal scroll
- [ ] Hover: show message actions (reply, more)
- [ ] Own messages: same alignment (left), different subtle background
- [ ] Timestamps: relative for recent, absolute for older

### US-15: Empty States

As a **workspace member**,
I want to **see helpful empty states when there's no data**,
so that **I'm guided on what to do next**.

Acceptance Criteria:
- [ ] No conversations: illustration + "Start a conversation" CTA
- [ ] No messages in channel: "No messages yet. Say hello! 👋"
- [ ] No search results: "No results found" (future, but design now)
- [ ] Error state: icon + message + retry button
- [ ] Loading state: skeleton UI matching final layout

### US-16: Mobile Responsive

As a **mobile user**,
I want to **use chat on my phone**,
so that **I can communicate on the go**.

Acceptance Criteria:
- [ ] 375px: chat list takes full width, hides content panel
- [ ] Clicking a channel: navigates to full-screen chat view
- [ ] Back button: returns to chat list
- [ ] 768px+: side-by-side layout
- [ ] Touch-friendly: min 44px tap targets
- [ ] Chat editor: keyboard-aware, stays above virtual keyboard

---

## Component Architecture (for DEV reference)

```
ChatLayout (route)
├── ChatSidebar
│   ├── ChatListHeader (search + create)
│   └── ChatList
│       └── ChatListItem[] (avatar, name, preview, badge, time)
├── ChatView (content area)
│   ├── ChatHeader (name, members count, actions)
│   ├── MessageList (virtualized)
│   │   ├── DateSeparator
│   │   ├── MessageGroup (avatar + name header)
│   │   │   └── MessageBubble[] (content only)
│   │   └── SystemMessage (centered)
│   ├── TypingIndicator
│   └── ChatEditor (input + send button)
└── InfoPanel (slide-in, conditional)
    ├── MemberList
    └── MemberItem (avatar, name, status)
```
