# UX/UI Standardization — Design

## Approach

Systematic module-by-module pass using the UX Design System checklist. Each module gets audited → fixed → verified before moving to the next.

## Typography Migration Map

Replace raw Tailwind text classes with design token utilities:

| Raw Class | → Design Token | Usage |
|-----------|---------------|-------|
| `text-2xl font-bold` | `text-title` | Page headings |
| `text-lg font-semibold` | `text-section` | Section headings |
| `text-sm` / `text-[14px]` | `text-body` | Primary content |
| `text-sm font-medium` | `text-body-ui` | Interactive labels |
| `text-sm font-semibold` | `text-body-strong` | Emphasized text |
| `text-[13px]` | `text-small` | Secondary content |
| `text-xs` | `text-caption` | Metadata, timestamps |
| `text-xs font-medium` | `text-caption-ui` | Small interactive labels |
| `text-[0.65rem]` / `text-[0.7rem]` | `text-overline` | Section labels (uppercase) |
| `text-[10px]` / `text-[0.6rem]` | `text-micro` | Badges, counters |

## Spacing Standardization

Replace arbitrary spacing with scale values:

| Arbitrary | → Standard | Tailwind |
|-----------|-----------|----------|
| `p-2.5`, `px-2.5` | `p-2` or `p-3` | 8px or 12px |
| `py-1.5` | `py-1` or `py-2` | 4px or 8px |
| `gap-2.5` | `gap-2` or `gap-3` | 8px or 12px |
| `gap-1.5` | `gap-1` or `gap-2` | 4px or 8px |
| `mt-0.5` | `mt-1` | 4px |
| `style={{ height: 36 }}` | `h-9` | 36px (standard row height) |

> **Note**: `py-1.5` and `gap-1.5` map to 6px which is NOT in our scale. Decide per-case between 4px and 8px.

## Inline Style Removal

Files with inline styles to convert:

| File | Current | Fix |
|------|---------|-----|
| `documents.tsx` | `style={{ height: 36 }}` on `<tr>` | `h-9` class |
| `DataTable.tsx` | `style={{ height }}` | `h-9` class |
| `DriveFileRow.tsx` | `style={{ height }}` | `h-9` class |
| `PeekPanel.tsx` | `style={{ '--peek-w' }}` | Keep (CSS var for desktop width — intentional) |

## Component Consistency Rules

### Filter buttons (asset list, drive tabs)

Currently: arbitrary `px-2.5 py-1 text-xs rounded-[var(--radius-sm)]`

Standardize to:
```tsx
// Filter pill — consistent across all modules
className="px-2 py-1 text-caption-ui rounded-sm ..."
```

### Section labels (peek panel, sidebars)

Currently: `text-[0.65rem] font-semibold text-text-muted uppercase tracking-wider`

Standardize to:
```tsx
className="text-overline text-text-muted"
// text-overline already includes: 11px, font-weight 600, uppercase, tracking 0.5px
```

### Table headers

Currently: `text-caption-ui text-gray-10 uppercase tracking-wider` ← Already good, keep this pattern.

## Module-by-Module Plan

### 1. Primitives & Composites (foundation)
Fix the shared components first so all consumers inherit improvements.

**Files**: `Button.tsx`, `Input.tsx`, `Text.tsx`, `Avatar.tsx`, `Select.tsx`, `Textarea.tsx`, `DataTable.tsx`, `Tabs.tsx`, `Timeline.tsx`, `Breadcrumbs.tsx`, `EmptyState.tsx`

### 2. Patterns (shared layout components)
**Files**: `LarkRail.tsx`, `ListPanel.tsx`, `ChatListItem.tsx`, `MessageItem.tsx`, `FilePreviewCard.tsx`, `ChatList.tsx`

### 3. Chat module
**Files**: `channels.$channelId.tsx`, `ChatEditor.tsx`, `MessageContent.tsx`, `ReactionBar.tsx`, `ChannelInfoPanel.tsx`, `CreateChannelModal.tsx`, `InviteMemberForm.tsx`, `EmojiPicker.tsx`

### 4. Drive module
**Files**: `drive.tsx`, `DriveSidebar.tsx`, `DriveFileRow.tsx`, `DriveFileList.tsx`, `DriveTreePanel.tsx`, `DriveContextPanel.tsx`

### 5. Assets module
**Files**: `_assets.tsx`, `list.tsx`, `dashboard.tsx`, `types.tsx`, `requests.tsx`, `$assetId.tsx`, `request/new.tsx`, `NotificationBell.tsx`

### 6. Auth & Workspace shell
**Files**: `_auth.tsx`, `login.tsx`, `OtpInput.tsx`, `_workspace.tsx`, `documents.tsx`, `settings.tsx`

## Verification

After each module:
1. Run self-correction checklist (spacing → alignment → hierarchy → consistency → balance)
2. Verify at 375px and 1280px
3. `vite build` passes (no regressions)
