## Context

`index.css` hiện có 809 dòng, chia thành:
1. **Tailwind v4 tokens** (`@theme`, `@utility`, `@keyframes`) — 270 lines → **KEEP**
2. **Component CSS** (BEM classes) — ~400 lines → **MIGRATE to JSX**
3. **Editor/rich-text styles** (`.chat-editor-content`, `.message-html`) — ~60 lines → **KEEP** (TipTap cần global CSS)
4. **Scrollbar, animation utilities** — ~80 lines → **KEEP or convert to `@utility`**

Vấn đề cốt lõi: BEM component classes (`.nexus-sidebar__item`, `.msg-bubble`, `.chat-list-item`) set `color`, `background`, `padding` trực tiếp trên elements — **conflict specificity** với Tailwind utilities trên cùng element. Tailwind v4 generate utilities với specificity bình thường, nhưng khi component CSS cũng target cùng element qua class selector, thứ tự trong CSS output quyết định winner — không predictable.

## Goals / Non-Goals

**Goals:**
- Loại bỏ 100% BEM component CSS classes khỏi `index.css`
- Mỗi component self-contained: mọi visual styling inline trong JSX bằng Tailwind utilities
- Giữ `index.css` chỉ chứa: `@theme` tokens, `@utility` text roles, `@keyframes`, scrollbar, TipTap editor styles
- Visual output 100% identical trước và sau refactor

**Non-Goals:**
- Không redesign components — chỉ migrate CSS sang Tailwind
- Không refactor component structure hay props
- Không thêm dark mode — chỉ light mode hiện tại
- Không thay đổi responsive breakpoints

## Decisions

### 1. Migration strategy: Component-by-component, verify sau mỗi component

**Quyết định**: Migrate từng component CSS block riêng lẻ, build + visual verify sau mỗi block.

**Rationale**: Mỗi component là independent scope. Nếu sai 1 class = UI regression chỉ ở component đó, dễ rollback. Batch migrate sẽ khó debug.

**Alternatives considered**:
- Batch migrate tất cả cùng lúc → Rủi ro cao, khó debug
- Extract thành CSS modules per component → Vẫn giữ specificity problem, thêm complexity

### 2. Migration order: Outside-in (layout → content → micro)

1. **AppSidebar** (`.nexus-sidebar*`) — 13 selectors
2. **TopBar** (`.nexus-topbar*`) — 10 selectors
3. **ChatListItem** (`.chat-list-item*`, `.chat-external-badge`, `.unread-badge`) — 5 selectors
4. **MessageItem** (`.msg-bubble*`) — 10 selectors
5. **ChatHeader** (`.chat-header*`) — 5 selectors
6. **Pill tabs** (`.pill-tab*`) — 4 selectors
7. **Timestamp** (`.timestamp-*`) — 4 selectors
8. **Resize handle** (`.resize-handle*`) — 3 selectors
9. **Auth background** (`.bg-nexus-auth`) — 1 selector
10. **Animation classes** (`.animate-*`) → convert sang `@utility`

### 3. Hover/active/focus states: Tailwind modifiers

CSS hover rules (`.nexus-sidebar__item:hover`) → Tailwind `hover:bg-surface-container` directly in JSX.

### 4. Giữ `@theme`, `@utility`, editor styles

Những phần sau KEEP trong `index.css`:
- `@theme` block — design token source of truth
- `@utility text-*` — typography roles
- `@utility focus-ring`, `scrollbar-none` — genuinely reusable utilities
- `@keyframes` — animation definitions
- `.chat-editor-content`, `.message-html` — TipTap cần global CSS selectors
- Scrollbar styles — pseudo-element styling

### 5. Convert `.animate-*` helpers → `@utility`

`.animate-reaction-pop`, `.animate-msg-slide-in`, `.animate-panel-slide`, `.animate-slide-in-right` → `@utility animate-*` blocks (Tailwind v4 custom utilities).

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Tailwind class strings rất dài trong JSX | Tách thành const string variables hoặc multi-line template literals. Đây là trade-off chấp nhận được — readability thua nhưng safety thắng |
| Visual regression khi migrate | Verify bằng screenshot comparison sau mỗi component. Build phải pass |
| Miss một state (hover, focus, active) | Checklist per component: normal → hover → active → focus-visible → disabled |
| TipTap editor styles cần global CSS | KEEP `.chat-editor-content` và `.message-html` — TipTap renders HTML bên trong, không control được class names |
