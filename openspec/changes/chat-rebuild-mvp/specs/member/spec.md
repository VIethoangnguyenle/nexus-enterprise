# Member — TIER 1 Spec

## Proto Mapping

| Feature | RPC | Status |
|---------|-----|--------|
| List members | `ListChannelMembers` | ✅ EXISTS |
| Add member | `AddChannelMember` | ✅ EXISTS |
| Remove member | `RemoveChannelMember` | ✅ EXISTS (defer UI to TIER 3) |

## Backend Status

All member RPCs exist and work:
- `ListChannelMembers` returns `ChannelMember{user_id, username, ngac_node_id}`
- `AddChannelMember` creates NGAC assignment + channel_members row
- REST endpoints: `GET/POST/DELETE /channels/{channelId}/members`
- Frontend hooks: `useChannelMembers`, `useAddChannelMember`, `useRemoveChannelMember`

No backend gaps for TIER 1 member scope.

---

## User Stories

### US-11: View Channel Members

As a **workspace member**,
I want to **see who is in a channel**,
so that **I know who I'm chatting with**.

Acceptance Criteria:
- [ ] Member list accessible from channel header (Members button/icon)
- [ ] Members displayed with: avatar, display name, online status
- [ ] Member count shown in channel header
- [ ] No UUID displayed — only display names
- [ ] Empty state: should not happen (creator is always a member)
- [ ] Members sorted: online first, then alphabetical

Proto: `ListChannelMembers` (existing)
Type: Frontend UX improvement

### US-12: Add Member to Channel

As a **workspace member**,
I want to **add another user to a channel**,
so that **they can participate in the conversation**.

Acceptance Criteria:
- [ ] "Add Member" button in member panel
- [ ] User picker: search/select from workspace contacts
- [ ] User picker shows display names, NOT node IDs or UUIDs
- [ ] After adding: member appears in list immediately (optimistic)
- [ ] Added member can immediately see channel in their chat list
- [ ] Added member can see message history
- [ ] Cannot add user already in channel (validation)
- [ ] Error: if add fails, show error message and rollback

Proto: `AddChannelMember` (existing) — NGAC assignment + DB tracking
Type: Frontend UX (user picker improvement)

---

## Flows

### Flow: View Members
1. User clicks Members icon in channel header
2. Side panel opens with member list
3. Members load from `GET /channels/{channelId}/members`
4. Each member shows: avatar, name, online dot

### Flow: Add Member
1. User clicks "Add Member" in member panel
2. User picker opens: shows workspace users not already in channel
3. User searches/selects a contact
4. System: `POST /channels/{channelId}/members` with ngac_node_id
5. Optimistic: member appears in list immediately
6. New member: channel appears in their chat list on next load/WS refresh

---

## States

| Screen | Empty | Loading | Loaded | Error |
|--------|-------|---------|--------|-------|
| Member list | "No members" (shouldn't happen) | Skeleton items | Member cards | "Failed to load" + retry |
| User picker | "No contacts available" | Loading spinner | Contact list with search | "Failed to load contacts" |
