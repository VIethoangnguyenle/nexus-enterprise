# Design: Stitch UI Parity — 3-Phase

## Reference Screens

```
design/stitch/
├── desktop/
│   ├── contacts.png           Contacts table + STATUS column
│   ├── drive-my-files.png     Drive table + filter pills
│   ├── drive-shared.png       Shared preview dialog
│   ├── chat.png               Chat with sections + file cards
│   └── dept-chat.png          Dept chat + detail panel
└── responsive/
    ├── chat-list-mobile.png   Mobile chat list + bottom nav + pinned scroll
    ├── chat-tablet.png        Tablet 2-column chat
    ├── dept-chat-mobile.png   Mobile dept chat full-screen
    ├── dept-chat-tablet.png   Tablet dept chat + detail panel
    ├── contacts-profile-popup.png  Profile popup right panel
    ├── workspace-selection-mobile.png  Mobile workspace cards
    └── workspace-selection-tablet.png  Tablet workspace cards
```

## Phase 1: Desktop Feature Gaps

### 1.1 Contacts — STATUS column + Profile Popup

**Current**: ContactCard = grid cards (centered, large avatar)
**Target**: Table rows + right-panel profile popup

```
┌─────────────────────────────────────────────────────────────────┐
│ Contacts Tree  │  Developers (48 Members)            │ Profile │
│                │                                     │ Popup   │
│ ▼ Org Contacts │  NAME          ROLE     EMAIL STATUS │         │
│   DevOps (12)  │  ┌─ avatar ─┐                      │ [Photo] │
│   Developers   │  │ S.Jenkins│ Lead..  s@..  Online  │ S.Jenk  │
│   Marketing    │  │ M.Chen   │ Sr..   m@..  Offline │ VP Eng  │
│   Design       │  │ E.Rodrig.│ Dir..  e@..  Online  │ @sarah  │
│ ▶ External     │  │ A.Rodrig.│ Eng..  a@..  Meeting │         │
│ ▶ New Contacts │  └──────────┘                      │ Email   │
│ ★ Starred      │                                     │ Phone   │
│ ⋮ My Groups    │                                     │ Loc     │
│                │                                     │ [Msg]   │
│                │                                     │ [Call]  │
└─────────────────────────────────────────────────────────────────┘
```

**Files to modify:**
- `ContactsFilterBar.tsx` — add STATUS column header
- Route `_workspace.tsx` contacts module — switch from card grid to table layout
- **NEW** `ContactsTable.tsx` — table with rows (avatar, name, role, email, status badge)
- **NEW** `ContactProfilePanel.tsx` — right-panel popup (large avatar, details, actions)

**Status badges:**
- `Online` → green badge
- `Offline` → gray badge
- `In a meeting` → blue badge

### 1.2 Drive — File Type Filter Pills

**Current**: No file type filters
**Target**: Horizontal pill tabs: All Types / Doc / Sheet / Slide / PDF

```
┌──────────────────────────────────────────────────┐
│ My Files                                         │
│ Manage and organize your personal documents.     │
│                                                  │
│ [All Types] [Doc] [Sheet] [Slide] [PDF]          │
│                                                  │
│ NAME              MODIFIED    OWNER       SIZE   │
│ ├─ Q3 Marketing   Oct 12..   Me          2.4MB  │
│ ├─ Project Nexus   Oct 10..   Sarah J.    —     │
│ └─ Budget Forecast Oct 08..   Me          845KB  │
└──────────────────────────────────────────────────┘
```

**Files to modify:**
- **NEW** `DriveFilterPills.tsx` — horizontal filter tabs component
- `DriveFileList.tsx` — integrate filter state, filter rows by file type

### 1.3 Drive — Shared Preview Dialog

**Current**: No preview dialog for shared files
**Target**: Modal overlay with image preview + metadata + Save/Download buttons

**Files to modify:**
- **NEW** `DrivePreviewDialog.tsx` — modal component
- `DriveFileRow.tsx` — add click handler to open preview dialog for shared files

### 1.4 Chat — List Sections

**Current**: Flat channel list
**Target**: Grouped: PINNED → DEPARTMENTS → DIRECT MESSAGES

**Files to modify:**
- `ListPanel.tsx` or chat list component — add section headers + grouping logic
- Grouping logic: starred channels → channels with type "department" → DM channels

### 1.5 Chat — File Attachment Card

**Current**: Basic file display
**Target**: Styled card with file icon + name + size + type + download arrow

**Files to modify:**
- `MessageContent.tsx` — enhance file attachment rendering

### 1.6 Chat — Code Blocks

**Current**: No code block rendering
**Target**: Monospace dark background + copy button

**Files to modify:**
- `MessageContent.tsx` — detect code fences in message content, render styled blocks

---

## Phase 2: Responsive Layouts

### 2.1 Chat Mobile

**Target** (from `chat-list-mobile.png`):
- Full-screen chat list (no sidebar visible)
- PINNED section = horizontal scrollable avatar circles
- RECENT section = standard list rows
- Bottom tab bar (already exists: MobileNav)
- Tap channel → full-screen message view with back arrow

### 2.2 Chat Tablet

**Target** (from `chat-tablet.png`):
- Collapsed sidebar (icon + label only)
- 2-column: chat list + message area
- No detail panel (detail panel appears only in dept chat variant)

### 2.3 Workspace-Select Responsive

**Mobile** (from `workspace-selection-mobile.png`):
- Single column, stacked cards with large touch targets
- Sections: PERSONAL / ORGANIZATIONS
- Each card: avatar + name + plan + member/project count

**Tablet** (from `workspace-selection-tablet.png`):
- Centered card layout, richer metadata
- Description line per workspace
- Tags: PERSONAL / EXTERNAL
- Two buttons: "Create New Workspace" + "Join with Code"

---

## Phase 3: Advanced Features + Polish

### 3.1 Dept Chat Detail Panel

Right panel for department channels:
- Dept avatar + name + description
- Pinned Items grid (2-col cards)
- Members list with role labels
- "View all members" link

### 3.2 Editor Toolbar Parity

Match Stitch design:
- B / I / strikethrough / code / ordered-list / unordered-list icons
- Attachment + emoji + @ mention buttons
- "Press Enter to send" hint text + blue send button

### 3.3 Workspace-Select Tags

- PERSONAL / EXTERNAL badges on cards
- "Join with Code" button (tablet+)

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Phase order | Desktop gaps → Responsive → Polish | Desktop users = primary audience now |
| Status data source | Frontend mock first | Backend status system needed separately |
| File type filtering | Client-side filter | Files already loaded, no API change needed |
| Code block rendering | Regex detect ``` blocks | Consistent with TipTap markdown support |
