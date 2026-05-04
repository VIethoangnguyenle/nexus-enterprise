# Add Channel Member

## Proto Mapping
- RPC: `AddChannelMember` — ✅ exists (gRPC + REST)
- Request: `{ channel_id, requester_ngac_node_id, target_ngac_node_id }`
- Response: `Empty` (200 OK with `{ status: "ok" }`)

## User Stories

### Story 1: Add Member Button
As a channel admin/owner,
I want to see an "Add Member" button in the Members tab,
so that I can invite workspace members to the channel.

**Acceptance Criteria:**
- [ ] MembersTab displays an "Add Member" button (icon + text or icon-only)
- [ ] Button is visible only for channel owner/admin (anyone for now — NGAC will enforce server-side)
- [ ] Button uses consistent design token styling (primary variant)

### Story 2: Member Picker Dialog
As a channel admin/owner,
I want to search and select workspace members from a dialog,
so that I can add them to the channel.

**Acceptance Criteria:**
- [ ] Clicking "Add Member" opens a modal/dialog
- [ ] Dialog shows a search input field
- [ ] Typing filters workspace members by username (minimum 1 character)
- [ ] Each result shows avatar initial + username
- [ ] Members already in the channel are either hidden or shown as "Already added" (disabled)
- [ ] Clicking a member triggers the `addMember` API call
- [ ] Dialog closes after successful add
- [ ] On API error, toast error message is shown, dialog stays open

### Story 3: Optimistic Cache Update
As a user,
I want the member list to update instantly after adding a member,
so that the UI feels responsive.

**Acceptance Criteria:**
- [ ] New member appears in the list immediately (before API response)
- [ ] Member count in channel header increments by 1
- [ ] On API failure, member is removed from list (rollback)
- [ ] `channelMembers` query cache is updated optimistically

**States:**
- Empty: No workspace members match search → "No matching members"
- Loading: Member list loading → spinner/skeleton
- Error: API failure → toast with retry option
- Success: Member added → member appears in list, dialog closes

## Flows

### Flow: Add Member
1. User opens ChannelInfoPanel → Members tab
2. User clicks "Add Member" button
3. Modal opens with search input focused
4. User types username to search
5. Filtered workspace members appear (excluding existing channel members)
6. User clicks on a member to add
7. System calls `POST /channels/:chId/members` with `{ ngac_node_id }`
8. Member appears in list immediately (optimistic)
9. Modal closes
10. Member count in header updates

### Error Flow:
- If API returns 403 (permission denied) → Toast: "You don't have permission to add members"
- If API returns 500 → Toast: "Failed to add member. Please try again."
- Optimistic update rolls back on any error
