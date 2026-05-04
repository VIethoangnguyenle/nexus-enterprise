# Design: Chat Member Management

## Design Decisions

- **No new screens needed** — feature extends the existing `ChannelInfoPanel` MembersTab
- **Modal pattern** for Add Member dialog — consistent with other modals in the system (ConfirmDialog, Modal composites)
- **Inline actions** on member rows for Remove — hover-reveal icon pattern (consistent with Drive actions)
- **ConfirmDialog** for remove confirmation — existing composite, destructive variant
- **Search-as-you-type** for member picker — filters workspace contacts

## Screen Inventory

| Screen | Purpose | Layout Pattern |
|--------|---------|---------------|
| MembersTab (enhanced) | Show members with add/remove actions | List inside ChannelInfoPanel right panel |
| AddMemberModal | Search and select workspace members to add | Modal overlay with search input + results list |
| RemoveConfirmDialog | Confirm member removal | ConfirmDialog composite (destructive) |

## Screen Details

### Screen 1: MembersTab (Enhanced)
**Layout**: Extends existing `MembersTab` inside `ChannelInfoPanel` (340px right panel)

#### Component Mapping
| Element | Component | Status |
|---------|-----------|--------|
| "Add Member" button | `<Button variant="ghost" size="sm">` + UserPlus icon | **Reuse** existing primitive |
| Member count header | Existing text element | **Reuse** |
| Member row | Existing flex layout (avatar + username) | **Reuse** |
| Remove icon on hover | `<IconButton>` with X icon | **Reuse** existing primitive |
| Avatar circle | Existing initial-letter div | **Reuse** |

#### Layout
```
┌──────────────────────────┐
│  3 members    [+ Add]    │  ← header with count + add button
├──────────────────────────┤
│  🔵 viethoangnguyenle  ✕ │  ← member row with hover remove
│  🟣 testuser2.nexus     ✕ │
│  🟢 testuser3.nexus     ✕ │
└──────────────────────────┘
```

### Screen 2: AddMemberModal
**Layout**: `<Modal>` composite (max-w-sm centered overlay)

#### Component Mapping
| Element | Component | Status |
|---------|-----------|--------|
| Modal shell | `<Modal>` | **Reuse** existing composite |
| Title | "Add Member to #channelName" | Built into Modal |
| Search input | `<Input>` with Search icon | **Reuse** existing primitive |
| Results list | Custom list (flex layout matching MembersTab) | **Compose** from existing patterns |
| Member result row | Avatar + username + "Add" button | Compose: Avatar div + `<Button size="sm">` |
| "Already added" badge | `<Badge variant="outline">` | **Reuse** existing primitive |
| Loading state | `<Spinner size="sm">` | **Reuse** existing primitive |
| Empty state | Text centered "No matching members" | Simple text |

#### Layout
```
┌─────────────────────────────────┐
│  Add Member to #general      ✕  │
├─────────────────────────────────┤
│  🔍 Search workspace members... │  ← input with search icon
├─────────────────────────────────┤
│  🔵 user4.nexus        [Add]   │  ← available member
│  🟣 user5.nexus        [Add]   │
│  🟢 testuser2.nexus  ✓ Added   │  ← already in channel (disabled)
└─────────────────────────────────┘
```

#### States
- **Empty**: No search results → centered "No matching members" text
- **Loading**: Contacts loading → `<Spinner>` centered
- **Error**: API failure → toast error via existing toast system
- **Loaded**: List of workspace members with add/disabled states

### Screen 3: RemoveConfirmDialog
**Layout**: `<ConfirmDialog>` composite (destructive variant)

#### Component Mapping
| Element | Component | Status |
|---------|-----------|--------|
| Dialog shell | `<ConfirmDialog>` | **Reuse** existing composite |
| Title | "Remove Member" | Built into ConfirmDialog |
| Message | "Remove {username} from #{channelName}?" | Built into ConfirmDialog |
| Cancel button | ConfirmDialog built-in | **Reuse** |
| Remove button | `variant="error"` | **Reuse** existing |

## New Components Needed

**None.** All elements compose from existing primitives and composites:
- `Modal`, `ConfirmDialog`, `Button`, `IconButton`, `Input`, `Badge`, `Spinner`, `Avatar`

Only changes needed are within `ChannelInfoPanel.tsx` (enhanced MembersTab) and new hooks/API functions.

## Responsive Strategy

- **Mobile (375px)**: ChannelInfoPanel is full-screen overlay (already handled). Modal is full-width with padding. Same layout works.
- **Tablet (768px)**: Panel width maintains. Modal centered with max-width.
- **Desktop (1280px)**: Panel is 340px right-side. Modal centered at ~400px width.

No responsive-specific changes needed — existing Modal and ChannelInfoPanel patterns already handle all breakpoints.
