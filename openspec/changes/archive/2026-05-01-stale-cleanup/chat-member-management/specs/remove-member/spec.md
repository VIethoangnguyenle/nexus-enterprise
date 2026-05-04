# Remove Channel Member

## Proto Mapping
- RPC: `RemoveChannelMember` — ✅ gRPC exists, ❌ REST missing
- Request: `{ channel_id, requester_ngac_node_id, target_ngac_node_id }`
- Response: `Empty`
- **Backend work needed**: Add `DELETE /channels/:chId/members/:nodeId` REST endpoint

## User Stories

### Story 1: Remove Member Action
As a channel admin/owner,
I want to remove a member from the channel,
so that I can manage channel access.

**Acceptance Criteria:**
- [ ] Each member row in MembersTab shows a remove action (X icon or "Remove" text) on hover
- [ ] Remove action is NOT shown for the current user (can't remove yourself)
- [ ] Clicking remove shows a confirmation dialog: "Remove {username} from #{channelName}?"
- [ ] Confirmation dialog has "Cancel" and "Remove" buttons
- [ ] "Remove" button uses error/destructive variant styling

### Story 2: Remove Member API Call
As a channel admin/owner,
I want the remove action to call the backend and update the UI instantly.

**Acceptance Criteria:**
- [ ] Confirming removal calls `DELETE /channels/:chId/members/:nodeId`
- [ ] Member is removed from the list immediately (optimistic)
- [ ] Member count in channel header decrements by 1
- [ ] On API failure, member reappears in list (rollback)
- [ ] On success, confirmation dialog closes
- [ ] On 403 error, toast: "You don't have permission to remove members"

### Story 3: Backend REST Endpoint
As the system,
the `DELETE /channels/:chId/members/:nodeId` REST endpoint must exist.

**Acceptance Criteria:**
- [ ] REST handler `RemoveChannelMember` registered at `DELETE /channels/:chId/members/:nodeId`
- [ ] Handler extracts requester's `ngac_node_id` from JWT claims
- [ ] Handler calls `domain.RemoveMember(ctx, channelID, targetNodeID)`
- [ ] Returns 200 `{ status: "ok" }` on success
- [ ] Returns proper error codes (403 for permission denied, 404 for not found)

**States:**
- Confirmation: Dialog open → "Are you sure?"
- Loading: Remove in progress → button shows spinner
- Error: API failure → toast, member reappears
- Success: Member removed from list

## Flows

### Flow: Remove Member
1. User opens ChannelInfoPanel → Members tab
2. User hovers over a member row → Remove icon (X) appears
3. User clicks remove icon
4. Confirmation dialog: "Remove {username} from #{channelName}?"
5. User clicks "Remove"
6. System calls `DELETE /channels/:chId/members/:nodeId`
7. Member disappears from list immediately (optimistic)
8. Dialog closes
9. Member count decrements

### Error Flow:
- If API returns 403 → Toast: "You don't have permission to remove members"
- If API returns 404 → Toast: "Member not found"
- Optimistic update rolls back on any error
