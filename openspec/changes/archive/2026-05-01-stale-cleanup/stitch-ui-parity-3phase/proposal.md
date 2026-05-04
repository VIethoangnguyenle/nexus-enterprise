# Stitch UI Parity — 3-Phase Implementation

## What

Đưa toàn bộ frontend Nexus Hub đạt parity với Stitch Design System mới nhất. Bao gồm responsive layouts (mobile/tablet/desktop), style adjustments cho Contacts, Drive, Chat, và Workspace Selection.

## Why

1. **Design debt**: Stitch đã cập nhật 14+ screens bao gồm responsive variants — code chưa reflect
2. **Mobile experience**: Hiện tại mobile layouts chưa match Stitch specs (chat sections, pinned horizontal scroll, workspace cards)
3. **Feature gaps**: Contacts profile popup, Drive file filters, Chat detail panel, code blocks — tất cả đều đã design nhưng chưa implement
4. **Consistency**: Desktop screens cũng có delta (STATUS column, attachment cards, toolbar layout)

## Scope — 3 Phases

### Phase 1: Desktop Feature Gaps (High-value, visible)
- Contacts: Table với STATUS column + profile popup right panel
- Drive: File type filter pills + shared preview dialog
- Chat: Chat list sections (PINNED/DEPARTMENTS/DM) + file attachment card styling + code blocks

### Phase 2: Responsive Layouts (Mobile + Tablet)
- Chat mobile: Full-screen messages, pinned horizontal scroll, back nav
- Chat tablet: 2-column collapsed sidebar layout
- Workspace-select responsive: Mobile single-column + tablet enriched cards
- Contacts/Drive: Responsive table → card adaptations

### Phase 3: Advanced Features + Polish
- Chat dept detail panel (right panel: info, pinned items, members)
- Chat editor toolbar parity (B/I/formatting + "Press Enter to send")
- Workspace-select: PERSONAL/EXTERNAL tags, "Join with Code"
- Cross-module polish pass

## Out of Scope

- Backend API changes (tận dụng API hiện có)
- New backend features (member count, real-time status — sẽ là changes riêng)
- Approvals module UI (change riêng)
- New authentication flows
