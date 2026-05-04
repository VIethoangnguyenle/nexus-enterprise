# Tasks: Stitch UI Parity — 3-Phase

## Phase 1: Desktop Feature Gaps

### Contacts
- [x] **P1-C1: ContactsTable component** — New table layout replacing card grid: avatar + name/username + role + email + STATUS badge (Online/Offline/Meeting)
- [x] **P1-C2: ContactProfilePanel** — Right-panel profile popup: large avatar, name, title, dept, email/phone/location fields, Message + Call action buttons
- [x] **P1-C3: Integrate into contacts route** — Replace existing card grid with table + profile panel, click row → show profile panel

### Drive
- [x] **P1-D1: DriveFilterPills component** — Horizontal filter tabs: All Types / Doc / Sheet / Slide / PDF, client-side filtering
- [x] **P1-D2: DrivePreviewDialog** — Modal for shared file preview: image preview + title + shared-by + "Save to My Drive" + "Download" buttons
- [x] **P1-D3: Integrate filters into DriveFileList** — Wire filter state, filter displayed rows by selected type

### Chat
- [x] **P1-M1: Chat list sections** — Group channels: PINNED (starred) → DEPARTMENTS (type=department) → DIRECT MESSAGES (type=dm), with section headers
- [x] **P1-M2: File attachment card** — Styled card in MessageContent: file icon + filename + size + type label + download arrow
- [x] **P1-M3: Code block rendering** — Detect ``` fenced blocks in message content, render monospace with dark bg + copy button

## Phase 2: Responsive Layouts

### Chat Responsive
- [x] **P2-M1: Chat mobile list** — Full-screen chat list (no sidebar), PINNED = horizontal avatar scroll, RECENT = standard rows
- [x] **P2-M2: Chat mobile message view** — Full-screen messages, back arrow header, channel name + member count
- [x] **P2-M3: Chat tablet layout** — 2-column (collapsed sidebar + chat list + messages), hide detail panel

### Workspace-Select Responsive
- [x] **P2-W1: Workspace mobile layout** — Single column stacked cards, large touch targets, PERSONAL/ORGANIZATIONS sections
- [x] **P2-W2: Workspace tablet layout** — Centered cards with description + metadata, PERSONAL/EXTERNAL tags, "Join with Code" button

### Contacts/Drive Responsive
- [x] **P2-CD1: Contacts mobile** — Table → stacked cards on mobile, profile panel → full-screen overlay
- [x] **P2-CD2: Drive mobile** — Table → simplified list on mobile, context panel → full-screen overlay

## Phase 3: Advanced Features + Polish

- [x] **P3-1: Dept chat detail panel** — Right panel: dept avatar/name/description, pinned items grid, member list with roles
- [x] **P3-2: Chat editor toolbar parity** — Icon-based B/I/strikethrough/code/lists + @mention + attachment/emoji buttons + "Press Enter to send" hint
- [x] **P3-3: Workspace-select enrichment** — PERSONAL/EXTERNAL badge tags, plan/member count from API when available
- [x] **P3-4: Cross-module polish pass** — Verify all modules against Stitch screens, fix spacing/alignment/color deltas

## Verification

- [x] **V1: Build passes** — `npx vite build` succeeds after each phase
- [ ] **V2: Desktop parity** — Screenshot compare against Stitch desktop screens
- [x] **V3: Mobile test** — Verify at 375px width (iPhone SE)
- [ ] **V4: Tablet test** — Verify at 768px width (iPad portrait)
