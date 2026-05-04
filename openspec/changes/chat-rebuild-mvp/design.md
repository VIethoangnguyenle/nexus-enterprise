# Design: Chat Rebuild MVP

## Design Decisions

- **2-panel split layout** cho desktop: ChatList (280px fixed) + ChatView (fluid). Consistent với pattern đã có trong AppSidebar + ListPanel.
- **Message grouping**: consecutive messages from same sender grouped — giảm visual noise, tăng readability. Cùng alignment (left) cho cả own/other messages — consistent với Lark/Teams.
- **Empty states**: mỗi zero-data scenario có illustration + CTA rõ ràng — giúp onboarding.
- **Member panel**: slide-in overlay (360px) — reuse PeekPanel pattern. Online/offline grouping.
- **Mobile**: full-screen single panel — chat list OR chat view, không split.
- **No new components needed**: tất cả đều compose từ existing primitives + patterns.

## Screen Inventory

| Screen | Purpose | Stitch ID | Layout Pattern |
|--------|---------|-----------|----------------|
| Chat View (Desktop) | Active conversation with message list | `4c21a49becc046cebef6d529b5720deb` | 2-panel: ListPanel + Content |
| Empty States + Member Panel | Reference for empty/zero-data states + member slide-in | `5f56963d9da54d0b952d0401a6d45b36` | Reference sheet |
| Mobile Chat View | 375px active chat conversation | `646dda9143c745cb9c676860f0961d55` | Full-screen single panel |

### Existing Screens (reference)

| Screen | Stitch ID | Notes |
|--------|-----------|-------|
| Nexus Chat - Final | `9fffe37618be439780ff8c84c6f756bf` | Previous iteration — reference for message styling |
| Dept Chat - Mobile | `8249a28fc46741538f1a18a778d949d6` | Previous mobile — reference for touch targets |
| Group Chat - Members | `f19efefdcf5048748e53f8d160639824` | Previous member management — reference for member list |

---

## Screen Details

### Screen 1: Chat View (Desktop)

**Stitch Reference**: `4c21a49becc046cebef6d529b5720deb`
**Layout**: 2-panel — ChatList (280px) left + ChatView (fluid) right

#### Component Mapping

| Visual Element | Codebase Component | Status |
|---------------|-------------------|--------|
| Chat list container | `<ChatList>` (patterns/ChatList.tsx) | Reuse — polish |
| Chat list item | `<ChatListItem>` (patterns/ChatListItem.tsx) | Reuse — add last message preview, online dot |
| Chat list search | `<Input>` (primitives/Input.tsx) | Reuse |
| Create channel button | `<IconButton>` (primitives/IconButton.tsx) + `<CreateChannelModal>` | Reuse |
| Section headers (PINNED/DEPARTMENTS/DM) | Custom in ChatList | Reuse — already implemented |
| Unread badge | `<Badge>` (primitives/Badge.tsx) | Reuse |
| User avatar | `<Avatar>` (primitives/Avatar.tsx) | Reuse |
| Channel header bar | Custom in `channels.$channelId.tsx` route | Reuse — polish |
| Members icon button | `<IconButton>` (primitives/IconButton.tsx) | Reuse |
| Message list | `<MessageList>` (chat/MessageList.tsx) | Reuse — add grouping logic |
| Message item | `<MessageItem>` (patterns/MessageItem.tsx) | Reuse — add grouped mode |
| Date separator | Inline in MessageList | Reuse |
| Typing indicator | Inline in route | Reuse — already implemented |
| Chat editor | `<ChatEditor>` (chat/ChatEditor.tsx) | Reuse — already good |
| Send button | `<Button>` (primitives/Button.tsx) | Reuse |

#### States

- **Empty (no messages)**: Centered icon + "No messages yet" + "Say hello! 👋"
- **Loading**: Skeleton shimmer matching message layout
- **Error**: Error icon + "Failed to load messages" + Retry button
- **Loaded**: Message groups with date separators

---

### Screen 2: Empty States + Member Panel (Reference)

**Stitch Reference**: `5f56963d9da54d0b952d0401a6d45b36`
**Layout**: Reference sheet — 3 states side by side

#### Component Mapping

| Visual Element | Codebase Component | Status |
|---------------|-------------------|--------|
| Empty state container | `<EmptyState>` (EmptyState.tsx) | Reuse |
| Empty state illustration | Lucide icon (MessageSquare, Users) | Reuse |
| CTA button | `<Button variant="primary">` | Reuse |
| Member panel container | `<PeekPanel>` (composites/PeekPanel.tsx) | Reuse |
| Member list item | `<ContactCard>` (patterns/ContactCard.tsx) | Reuse — add online dot |
| Online status dot | Inline CSS (8px circle) | Simple — no component needed |
| Add member button | `<Button variant="secondary">` | Reuse |
| Member search | `<Input>` | Reuse |
| Section headers (ONLINE/OFFLINE) | Custom — label-caps style | Simple |

#### States

- **No conversations**: EmptyState + "Create Channel" CTA
- **No messages**: EmptyState + "Say hello!" message
- **Member panel open**: PeekPanel with grouped member list

---

### Screen 3: Mobile Chat View

**Stitch Reference**: `646dda9143c745cb9c676860f0961d55`
**Layout**: Full-screen single panel at 375px

#### Component Mapping

| Visual Element | Codebase Component | Status |
|---------------|-------------------|--------|
| Mobile header | Custom in route (responsive) | Reuse — add back button |
| Back button | `<IconButton>` (ChevronLeft icon) | Reuse |
| Message list | `<MessageList>` (same as desktop, responsive) | Reuse |
| Message item | `<MessageItem>` (responsive variant) | Reuse |
| Chat editor | `<ChatEditor>` (same, responsive) | Reuse |
| Send button | `<IconButton>` | Reuse |

#### States

- Same as desktop — responsive layout handled via CSS

---

## New Components Needed

**NONE** — All visual elements map to existing primitives and patterns. Key changes are:
1. **ChatListItem** — extend to show last message preview + online dot (props addition)
2. **MessageItem** — add `grouped` mode (boolean prop for hiding avatar/name)
3. **MessageList** — add grouping logic (pure frontend computation)
4. **ChannelInfoPanel** — add online status to member list items

## Responsive Strategy

- **Mobile (375px)**: Full-screen single panel. Chat list = separate route view. Chat view = full screen with back button.
- **Tablet (768px)**: Same as desktop but narrower ListPanel (240px).
- **Desktop (1280px+)**: 2-panel split — ChatList 280px + ChatView fluid.
