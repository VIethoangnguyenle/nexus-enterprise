# Feature Audit — NGAC Platform

> Audited: 2026-05-01 | Scope: All frontend modules

---

## 1. Module Inventory

### Frontend Metrics

| Metric | Value |
|--------|-------|
| Total TS/TSX files | 134 |
| Total LOC | 17,186 |
| Generated code | ~2,837 (proto + routeTree) |
| Application code | ~14,349 |
| Route files | ~20 |
| Components | ~39 (10 primitives + 10 composites + 19 patterns) |
| Hooks | 12 custom hooks |
| Stores | 5 Zustand stores |

### Feature Modules

| Module | Route Files | Components | Hooks | Store | Lines |
|--------|------------|------------|-------|-------|-------|
| **Chat/Messaging** | `channels.$channelId.tsx` (503), `channels.index.tsx` | 6 patterns, 1 chat/ | `useMessaging` (434L) | `websocket` (451L) | ~2,500 |
| **Drive** | `drive.tsx` (364) | 3 drive/ | `useDrive` (5.9KB) | `drive` (88L) | ~1,500 |
| **Approval** | `approval.tsx` (482) | 5 approval/ | `useApproval` (7KB) | N/A | ~1,800 |
| **Contacts** | `contacts.tsx` (254) | 5 patterns/ | `useContacts` (2KB) | N/A | ~700 |
| **Assets** | 5 route files | 0 dedicated | `useAssets` (6.2KB) | N/A | ~1,200 |
| **Auth** | 3 route files | 0 dedicated | `useAuth` (1KB) | `auth` (36L) | ~600 |
| **Workspace** | `_workspace.tsx` layout | LarkRail, TopBar, MobileNav | `useWorkspaces` (1KB) | `permission` (82L), `ui` (59L) | ~500 |

---

## 2. Per-Module Feature Analysis

### 2.1 Chat/Messaging — MOST COMPLEX

**Behavior Assessment**:
- ✅ Real-time messaging via WebSocket
- ✅ Channel creation/switching
- ✅ File attachment (via Drive integration)
- ✅ Emoji reactions
- ✅ Channel info panel with member list
- ✅ Channel drive panel (files shared in channel)
- ⚠️ Poll/Task features (backend ready, frontend partial)
- ❌ No message search
- ❌ No thread/reply support
- ❌ No message editing/deletion from UI

**Code Quality Issues**:
- `channels.$channelId.tsx` at **503 lines** — 11 React hooks in one component
- `websocket.store.ts` at **451 lines** — monolithic, handles all WS message types
- `useMessaging.ts` at **434 lines** — God hook, mixes TanStack Query + WebSocket state

**Recommendations**:
- Split channel route into `ChannelView`, `MessageList`, `MessageComposer`
- Split WebSocket store by concern (chat, typing, presence, notifications)
- Extract message rendering logic from route into MessageThread component

### 2.2 Drive — WELL STRUCTURED

**Behavior Assessment**:
- ✅ File upload with presigned URLs
- ✅ Folder creation/navigation
- ✅ Breadcrumb navigation
- ✅ File preview (images + files)
- ✅ Rename, move, copy, trash, restore, delete
- ✅ Context panel with file details
- ✅ Drag-and-drop upload
- ⚠️ No file sharing links
- ⚠️ No version history
- ❌ No search within drive

**Code Quality Issues**:
- `drive.tsx` at 364 lines — acceptable but could split sidebar
- `DriveContextPanel.tsx` at 420 lines — oversized, contains multiple sub-views
- `DriveSidebar.tsx` at 281 lines — handles tree + actions + state

**Recommendations**:
- Extract DriveContextPanel sub-views (Details, Versions, Sharing) into separate components
- Drive sidebar tree view could reuse existing `TreeView` pattern component

### 2.3 Approval — FEATURE COMPLETE

**Behavior Assessment**:
- ✅ Template CRUD with multi-step wizard
- ✅ Dynamic form builder (text, number, select, checkbox, file, date)
- ✅ Request submission with dynamic form rendering
- ✅ Multi-step approval execution
- ✅ Request status tracking
- ✅ Template versioning
- ⚠️ No bulk operations
- ⚠️ No approval delegation

**Code Quality Issues**:
- `approval.tsx` at **482 lines** — inline tab logic, two panel types inline
- 5 dedicated components well-structured in `components/approval/`
- `useApproval.ts` hook is comprehensive (7KB)

**Recommendations**:
- Extract ApprovalTabView from route (template list + request list)
- Extract TemplateDetailPanel / RequestDetailPanel already exists, but route still has inline PeekPanel logic

### 2.4 Contacts — SIMPLE BUT INCONSISTENT

**Behavior Assessment**:
- ✅ Contact list with status indicators
- ✅ Contact search/filter
- ✅ Contact profile panel
- ✅ Online/offline/meeting status
- ⚠️ No contact groups
- ⚠️ No contact import/export

**Code Quality Issues**:
- Uses **raw Tailwind colors** (`bg-green-500`, `bg-orange-400`, `bg-gray-300`) — violates design system
- `ContactsTable.tsx` at 192 lines — has hardcoded status color mapping
- Contact components in `patterns/` instead of dedicated `contacts/` directory

**Recommendations**:
- Create semantic status tokens (`--color-status-online`, `--color-status-offline`, etc.)
- Move contact components to `components/contacts/`
- Extract status badge into reusable component

### 2.5 Assets — SEPARATE LAYOUT

**Behavior Assessment**:
- ✅ Asset type management (CRUD)
- ✅ Asset instance management
- ✅ Asset request submission
- ✅ Asset lifecycle tracking
- ✅ Timeline component for history
- ⚠️ Asset detail panel could be richer

**Code Quality Issues**:
- Uses completely separate layout (`_assets.tsx`) from workspace
- Good primitive/composite usage in route files
- `useAssets.ts` is well-structured (6.2KB)

**Recommendations**:
- Evaluate merging into workspace layout with a dedicated sidebar section
- Share common patterns (list + detail + action) with other modules

### 2.6 Auth — MINIMAL

**Behavior Assessment**:
- ✅ Login with username/password
- ✅ OTP verification
- ✅ Workspace selection
- ✅ User onboarding
- ⚠️ No "forgot password"
- ⚠️ No profile management

**Code Quality Issues**:
- `login.tsx` at 258 lines — contains both login and OTP forms
- No dedicated auth components
- Good use of primitives

**Recommendations**:
- Split login route into LoginForm + OTPForm components
- Add profile management page

---

## 3. Cross-Module Inconsistencies

### Data Fetching Patterns

| Module | Pattern | Caching | Pagination |
|--------|---------|---------|------------|
| Messaging | TanStack Query + WebSocket | Optimistic via WS | ❌ None |
| Drive | TanStack Query | Standard | ❌ None |
| Approval | TanStack Query | Standard | ❌ None |
| Contacts | TanStack Query | Standard | ❌ None |
| Assets | TanStack Query | Standard | ❌ None |
| Auth | TanStack Query | Token-based | N/A |

**Issue**: No module implements pagination despite backend support.

### State Management Patterns

| Pattern | Module(s) | Assessment |
|---------|-----------|-----------|
| Zustand store for WS | Chat | ⚠️ 451 lines — should split |
| Zustand store for drive state | Drive | ✅ 88 lines — appropriate |
| Zustand store for permissions | Workspace | ✅ 82 lines — appropriate |
| Zustand store for UI | Workspace | ✅ 59 lines — appropriate |
| Zustand store for auth | Auth | ✅ 36 lines — appropriate |
| No store (query only) | Approval, Contacts, Assets | ✅ Correct for CRUD-only |

### Empty State Handling

| Module | Empty State | Pattern |
|--------|-------------|---------|
| Chat | Inline text + icon | Custom |
| Drive | `EmptyState` component | ✅ Reusable |
| Approval | Inline text | Custom |
| Contacts | Inline text | Custom |
| Assets | `DataTable` emptyMessage | Via composite |

**Issue**: No unified empty state component. Drive has the best pattern.

### Error Handling

| Module | Error Display | Loading | Retry |
|--------|--------------|---------|-------|
| Chat | Toast (via store) | Spinner ✅ | No |
| Drive | Toast + inline | Spinner ✅ | No |
| Approval | Toast | Spinner ✅ | No |
| Contacts | None visible | Spinner ✅ | No |
| Assets | Inline error text | Spinner ✅ | No |

**Issue**: No error boundary. No retry mechanism. Error display is inconsistent.

---

## 4. Missing Features (System-Wide)

| Feature | Impact | Priority |
|---------|--------|----------|
| Pagination | All list views load everything | High |
| Search | No search in Drive, Chat, Assets | High |
| Error boundaries | Crash = white screen | High |
| Profile management | No user settings | Medium |
| Keyboard shortcuts | No shortcuts except ESC-close | Medium |
| Bulk operations | No multi-select in Drive, Assets | Medium |
| Undo/redo | No undo for destructive actions | Low |
| Accessibility | aria-* present but incomplete | Low |
