# NGAC Platform — Comprehensive QC Test Plan

> **Purpose**: Reusable test document for full platform verification.
> **Pre-requisite**: App running at `http://localhost:5173`, all backend services up.

---

## Test Accounts

| # | Email | Role | OTP Code |
|---|-------|------|----------|
| U1 | `viethoangnguyenle@gmail.com` | Owner (Admin) | `999999` |
| U2 | `testuser2@nexus.dev` | Manager | `999999` |
| U3 | `testuser3@nexus.dev` | Employee | `999999` |

**Login Flow**: Enter email → Click Continue → Enter OTP `999999` → Select/Create workspace

---

## Test Modules

### Module 1: Authentication & Workspace Setup

| # | Test Case | Steps | Expected | Status |
|---|-----------|-------|----------|--------|
| 1.1 | Login U1 | Enter U1 email → OTP → workspace-select | Dashboard loads | |
| 1.2 | Create workspace | U1: + New Project → "NGAC Test Workspace" | Workspace created, sidebar shows | |
| 1.3 | Login U2 (new tab) | Open incognito → Enter U2 email → OTP | New user created | |
| 1.4 | Login U3 (new tab) | Open another incognito → Enter U3 email → OTP | New user created | |
| 1.5 | Invite U2 to workspace | U1: Settings → Invite → enter U2 username | U2 added as member | |
| 1.6 | Invite U3 to workspace | U1: Settings → Invite → enter U3 username | U3 added as member | |

### Module 2: Chat — Multi-user Real-time

| # | Test Case | Steps | Expected | Status |
|---|-----------|-------|----------|--------|
| 2.1 | Create channel | U1: Chat → New Channel → "general" | Channel appears in list | |
| 2.2 | Send message (U1) | U1: Type "Hello from Admin" → Send | Message appears instantly (optimistic) | |
| 2.3 | Receive message (U2) | U2: Open #general channel | U1's message visible via WS | |
| 2.4 | Send message (U2) | U2: Type "Hello from Manager" → Send | Message appears for U1 + U2 in real-time | |
| 2.5 | Send message (U3) | U3: Type "Hello from Employee" → Send | All 3 users see all messages | |
| 2.6 | Rapid chat | All users send 5+ messages quickly | No duplicates, correct order | |
| 2.7 | Add reaction | U1: React 👍 to U2's message | Emoji appears instantly for all users | |
| 2.8 | Remove reaction | U1: Click 👍 again to remove | Emoji disappears for all | |
| 2.9 | Pin message | U1: Pin U2's message | Pin indicator appears, pins list updated | |
| 2.10 | Unpin message | U1: Unpin the message | Pin indicator removed | |
| 2.11 | Create thread reply | U1: Reply to U2's message in thread | Thread count increments | |
| 2.12 | Add member to channel | U1: Channel settings → Add U3 | U3 can now see + send in channel | |
| 2.13 | Remove member from channel | U1: Channel settings → Remove U3 | U3 loses access to channel | |
| 2.14 | Typing indicator | U2 starts typing | U1 sees "U2 is typing..." | |
| 2.15 | Create DM channel | U1: New DM → Select U2 | Private channel created | |

### Module 3: Drive — File & Folder Management

| # | Test Case | Steps | Expected | Status |
|---|-----------|-------|----------|--------|
| 3.1 | Create folder | U1: Drive → New Folder → "Project Docs" | Folder appears in list | |
| 3.2 | Create subfolder | U1: Open "Project Docs" → New Folder → "Designs" | Subfolder created | |
| 3.3 | Upload file | U1: Upload a file to "Project Docs" | File appears in folder | |
| 3.4 | Upload file (U2) | U2: Upload a file to root | File visible to U2 | |
| 3.5 | Move file to folder | U1: Move uploaded file → "Designs" subfolder | File moves, optimistic removal from source | |
| 3.6 | Rename file | U1: Rename file → "Updated Name" | Name updates in list | |
| 3.7 | Share folder with U2 | U1: Share "Project Docs" → Add U2 (read) | U2 can see shared folder | |
| 3.8 | Share with U3 (write) | U1: Share "Project Docs" → Add U3 (write) | U3 can upload to folder | |
| 3.9 | Delete file (optimistic) | U1: Delete a file | File disappears instantly, no refetch | |
| 3.10 | Delete folder | U1: Delete "Designs" subfolder | Folder removed with contents | |
| 3.11 | Trash item | U1: Trash a file | File moves to trash instantly | |
| 3.12 | Drive quota check | U1: Check drive quota | Quota reflects used/available | |

### Module 4: Approval Workflow — NGAC Permission Flow

| # | Test Case | Steps | Expected | Status |
|---|-----------|-------|----------|--------|
| 4.1 | Create template (U1/Admin) | U1: Approvals → Templates → Create → "Leave Request" with steps: Manager → Admin | Template created | |
| 4.2 | Submit request (U3/Employee) | U3: Approvals → + New Request → Select "Leave Request" → Fill form → Submit | Request appears in U3's "My Requests" (optimistic insert) | |
| 4.3 | Pending shows for U2 | U2: Approvals → Pending tab | U3's request visible (U2 is Manager step) | |
| 4.4 | Approve step 1 (U2/Manager) | U2: Click Approve on U3's request | Request removed from U2 pending (optimistic), moves to next step | |
| 4.5 | Pending shows for U1 | U1: Approvals → Pending tab | Request now at Admin step | |
| 4.6 | Approve step 2 (U1/Admin) | U1: Click Approve | Request fully approved, removed from pending (optimistic) | |
| 4.7 | Check history | U1 + U2: History tab | Approved request visible | |
| 4.8 | Reject flow | U3: Submit another request → U2: Reject with comment | Request rejected, U3 sees updated status | |
| 4.9 | Batch approve | U2: Select multiple pending → Batch Approve | All selected removed from pending (optimistic) | |
| 4.10 | Audit log | U1: Click request → View audit trail | Shows all approval steps, actors, timestamps | |

### Module 5: Assets — Lifecycle + Requests

| # | Test Case | Steps | Expected | Status |
|---|-----------|-------|----------|--------|
| 5.1 | Create asset type (U1) | U1: Assets → Types → Create "Laptop" | Type appears | |
| 5.2 | Create asset (U1) | U1: Assets → Create "MacBook Pro 16" type=Laptop | Asset created with state | |
| 5.3 | Transition asset | U1: Asset detail → Transition (e.g., assign) | State updates, history logged | |
| 5.4 | Create asset request (U3) | U3: Assets → Request → "Laptop" | Request created | |
| 5.5 | Approve request (U1) | U1: Asset Requests → Approve U3's request | Status updates to "approved" (optimistic) | |
| 5.6 | Reject request | U1: Reject a request with reason | Status updates to "rejected" (optimistic) | |

### Module 6: Documents

| # | Test Case | Steps | Expected | Status |
|---|-----------|-------|----------|--------|
| 6.1 | Upload document (U1) | U1: Documents → Upload → Select file | Document appears in list | |
| 6.2 | Approve document | U1: Click approve on document | Status changes to "approved" (optimistic) | |
| 6.3 | Delete document | U1: Delete a document | Removed from list instantly (optimistic) | |

### Module 7: NGAC Permission Verification

| # | Test Case | Steps | Expected | Status |
|---|-----------|-------|----------|--------|
| 7.1 | U3 cannot create template | U3: Approvals → Templates tab | No "Create Template" button or action denied | |
| 7.2 | U3 cannot delete others' files | U3: Try to delete U1's file in Drive | Permission denied or action hidden | |
| 7.3 | U2 sees scoped requests | U2: Approval → Department tab | Only department-scoped requests | |
| 7.4 | Channel privacy | U3 removed from channel → cannot read messages | Channel not in list or access denied | |

---

## Responsive Breakpoint Checks

For critical screens, verify at:

| Breakpoint | Width | Device |
|------------|-------|--------|
| Mobile | 375px | iPhone SE |
| Tablet | 768px | iPad portrait |
| Desktop | 1280px | Laptop |

**Screens to check**: Chat, Drive, Approvals, Assets

---

## How to Run This Test Plan

### Pre-setup
```bash
# 1. Start infrastructure
cd /path/to/ngac && docker compose up -d

# 2. Start backend services 
cd backend && make run

# 3. Start frontend
cd frontend && npm run dev
```

### Browser Setup for 3 Users
1. **U1**: Normal browser window → http://localhost:5173
2. **U2**: Incognito window → http://localhost:5173
3. **U3**: Different browser (Firefox) or second incognito profile → http://localhost:5173

### Login Each User
1. Enter email → Click Continue
2. Enter OTP: `999999`
3. Select workspace (or create new for U1)

### Execution Order
1. Module 1 first (setup accounts + workspace)
2. Module 4 (Approval) — creates templates + permission structure
3. Module 2 (Chat) — multi-user real-time
4. Module 3 (Drive) — file operations
5. Module 5 (Assets) — lifecycle
6. Module 6 (Documents)
7. Module 7 (NGAC permissions)

---

## WebSocket Events to Monitor (DevTools Console)

Enable debug mode by setting in browser console:
```js
localStorage.setItem('WS_DEBUG', 'true')
```

Key events to watch:
- `chatMessage` — real-time message delivery
- `reactionEvent` — emoji add/remove
- `pinEvent` — pin/unpin
- `typingEvent` — typing indicator
- `driveObject` — file/folder changes
- `notification` — cross-module notifications
- `unreadCount` — badge updates

---

## Regression Checklist (After Any Change)

- [ ] Login flow works (OTP)
- [ ] Chat sends + receives in real-time
- [ ] Reactions update for all users
- [ ] Drive upload + delete works
- [ ] Approval create + approve/reject works
- [ ] No console errors
- [ ] No stale data after mutations
- [ ] Mobile layout not broken
