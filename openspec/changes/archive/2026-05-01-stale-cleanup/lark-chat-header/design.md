## Architecture

No new components. The existing channel header section in `channels.$channelId.tsx` is refactored from a single `<div>` to a 2-row structure.

## Detailed Design

### Current Header (1 row)
```
┌──────────────────────────────────────────────────────────┐
│  #Test   [Chat] [Pinned]         🔍 👥 📁               │
└──────────────────────────────────────────────────────────┘
```

### Target Header (2 rows — matching Lark)
```
┌──────────────────────────────────────────────────────────┐
│  [#] Test                            🔍 👥 📁 ⚙️         │  Row 1: Identity
├──────────────────────────────────────────────────────────┤
│  ● Chat    📌 Pinned    📄 Docs    📁 Files              │  Row 2: Tabs
└──────────────────────────────────────────────────────────┘
```

### Row 1 — Identity Bar
- **Left**: Channel icon (hash `#` in a 28px circle, bg-accent/10) + channel name (text-body-strong, 14px bold) + member count badge (text-micro, muted)
- **Right**: Action icon buttons — Search, Members, Files (existing HeaderButton)
- **Height**: 40px
- **Background**: bg-gray-3
- **Border**: bottom border-border/30 (subtle, since Row 2 below)

### Row 2 — Tab Bar
- Reuse existing `.pill-tab` CSS classes
- Tabs: `Chat` (active by default), `Pinned`, `Docs`, `Files`
- `Docs` and `Files` tabs open the info panel with respective tab
- **Height**: 32px
- **Background**: bg-gray-3 (same as Row 1, continuous surface)
- **Border**: bottom border-border/50

### Responsive (≤768px)
- Row 1: Channel name truncated, action icons compact (no labels already hidden via `.header-btn-text`)
- Row 2: Tabs scroll horizontally with `overflow-x: auto`, no wrapping
- Both rows reduce px from 16px to 12px

### CSS Classes
```css
.chat-header {
  background: var(--color-gray-3);
  border-bottom: 1px solid var(--color-border);
}
.chat-header__identity {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 40px;
  padding: 0 16px;
  border-bottom: 1px solid rgba(255,255,255,0.03);
}
.chat-header__tabs {
  display: flex;
  align-items: center;
  gap: 2px;
  height: 32px;
  padding: 0 16px;
  overflow-x: auto;
}
```

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Channel icon style | Hash `#` in circle | Standard messaging convention; Lark uses avatars for DMs, icons for groups |
| Tab bar vs pill bar | Reuse `.pill-tab` | Already implemented and visually consistent |
| Member count location | Inline with name | Keeps Row 1 scannable without extra UI elements |
| Settings icon | Not included | Not in current scope; can add later |
