## Why

`index.css` chứa **71 custom CSS selectors** (BEM classes: `.nexus-sidebar__*`, `.msg-bubble*`, `.chat-list-item*`, `.nexus-topbar__*`, v.v.) song song với Tailwind v4 utilities. Khi cả 2 hệ thống cùng control styling, CSS specificity conflicts xảy ra — gây bug như chữ đen trong self-message bubble (`.msg-bubble` override `color` trên children thay vì để Tailwind control). Tailwind v4 với `@theme` tokens và `@utility` directives đã đủ mạnh để thay thế 100% custom CSS. Loại bỏ custom component CSS = loại bỏ nguồn gốc specificity conflicts.

## What Changes

- **Migrate 8 component CSS blocks** từ `index.css` → inline Tailwind classes trong JSX components:
  - `.nexus-sidebar*` (13 selectors) → `AppSidebar.tsx`
  - `.nexus-topbar*` (10 selectors) → `TopBar.tsx`
  - `.chat-list-item*` (3 selectors) → `ChatListItem.tsx`
  - `.msg-bubble*` (10 selectors) → `MessageItem.tsx`
  - `.chat-header*` (5 selectors) → `channels.$channelId.tsx`
  - `.pill-tab*` (4 selectors) → inline wherever used
  - `.unread-badge` (1 selector) → `ChatListItem.tsx`
  - `.chat-external-badge` (1 selector) → `ChatListItem.tsx`
- **Migrate utility animations** (`.animate-*`, `.resize-handle`) → `@utility` directives hoặc inline
- **Keep**: `@theme` tokens, `@utility` text-* typography, `@keyframes`, scrollbar styles, TipTap editor styles (`.chat-editor-content`, `.message-html`) — những thứ Tailwind không thể inline
- **Remove**: `@media (min-width: 1024px)` PeekPanel block, `.bg-nexus-auth` → inline
- **Remove**: timestamp-pill, timestamp-divider → inline

## Capabilities

### New Capabilities
- `tailwind-component-styles`: Migration toàn bộ BEM component CSS sang inline Tailwind utilities trực tiếp trong JSX — loại bỏ custom CSS specificity layer

### Modified Capabilities
_(Không có thay đổi requirement level — đây là refactor implementation detail)_

## Impact

- **Files affected**: `index.css` (giảm ~400 lines), `AppSidebar.tsx`, `TopBar.tsx`, `ChatListItem.tsx`, `MessageItem.tsx`, `channels.$channelId.tsx`, + bất kỳ file nào reference BEM classes
- **Risk**: Medium — mỗi component migration là independent, có thể verify ngay. Nhưng sai className = UI regression
- **Dependencies**: Không thay đổi, chỉ CSS refactor
- **Breaking**: Không — visual output phải 100% identical
