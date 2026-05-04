# Conversation — TIER 1 Spec

## Proto Mapping

| Feature | RPC | Status |
|---------|-----|--------|
| Create group channel | `CreateChannel` | ✅ EXISTS |
| Create DM (1-1) | `CreateDM` | ✅ EXISTS |
| List channels | `ListChannels` | ✅ EXISTS |
| List DMs | `ListDMs` | ✅ EXISTS |
| Get channel | `GetChannel` | ✅ EXISTS |
| Rename channel | — | ❌ MISSING (needs new RPC) |

## Backend Gaps

### Rename Channel
- Need: `UpdateChannel` RPC in proto
- Need: REST `PATCH /channels/{channelId}` endpoint
- DB: `channels` table has `name` column — update-in-place
- WS: broadcast channel update to subscribers

> **BA Decision**: Include rename in TIER 1. It's a simple single-column update. No migration needed. ~30min backend work.

---

## User Stories

### US-5: Create Group Conversation

As a **workspace member**,
I want to **create a new group channel and name it**,
so that **I can collaborate with a team on a topic**.

Acceptance Criteria:
- [ ] "New Channel" button visible in chat list header
- [ ] Modal opens with: channel name input + channel type selector
- [ ] Name is required (validation)
- [ ] After creation, user is navigated to the new channel
- [ ] New channel appears in chat list for all workspace members with access
- [ ] Channel shows in list with # prefix for group, person icon for DM

Proto: `CreateChannel` (existing)
Type: Frontend polish (modal UX improvement)

### US-6: Create DM (1-1 Conversation)

As a **workspace member**,
I want to **start a direct message with another user**,
so that **I can have a private conversation**.

Acceptance Criteria:
- [ ] User can initiate DM from contacts list or user picker
- [ ] System finds existing DM if one exists (no duplicates)
- [ ] New DM appears in chat list with the other user's name
- [ ] DM is only visible to the two participants
- [ ] DM channel shows person avatar, not # prefix

Proto: `CreateDM` (existing — FindOrCreate logic already in domain service)
Type: Frontend UX

### US-7: Conversation List with Preview

As a **workspace member**,
I want to **see all my conversations with last message preview and timestamps**,
so that **I can quickly find active conversations**.

Acceptance Criteria:
- [ ] Chat list shows all channels user has access to
- [ ] Each item shows: channel name/avatar, last message preview (truncated), timestamp
- [ ] Last message preview updates in realtime (WS)
- [ ] Channels with unread messages appear bold / have badge
- [ ] Active channel is highlighted
- [ ] DMs show the other user's name, NOT "DM_xxx_yyy"
- [ ] List is sorted by most recent activity (last message time)
- [ ] Empty state: "No conversations yet" with "Create Channel" CTA

Proto: `ListChannels` + `ListDMs` + WS `lastMessages` cache (existing in store)
Type: Frontend polish

### US-8: Rename Conversation

As a **channel creator/admin**,
I want to **rename a channel**,
so that **the name reflects the current topic**.

Acceptance Criteria:
- [ ] Channel name is editable in channel header or settings
- [ ] Inline edit (click name → input field) or modal
- [ ] Name change persists after refresh
- [ ] Name change broadcasts to all members via WS
- [ ] Empty name rejected (validation)

Proto: NEW `UpdateChannel` RPC needed
Type: Backend + Frontend

---

## Flows

### Flow: Create Group Channel
1. User clicks "+" or "New Channel" in chat list
2. Modal opens: name input + type dropdown (Group/Private)
3. User enters name, clicks Create
4. System: `POST /workspaces/{wsId}/channels` with name + type
5. System: navigates to `/channels/{newChannelId}`
6. Channel appears in chat list

### Flow: Start DM
1. User goes to Contacts or clicks user profile
2. User clicks "Message" on a contact
3. System: `POST /channels/dm` with target user
4. System: either creates new DM or returns existing one
5. System: navigates to the DM channel

---

## States

| Screen | Empty | Loading | Loaded | Error |
|--------|-------|---------|--------|-------|
| Conversation list | "No conversations yet" + CTA | Skeleton items | Channel items | "Failed to load" + retry |
| Create channel modal | Form ready | Submit spinner | Success → navigate | Error message below form |
