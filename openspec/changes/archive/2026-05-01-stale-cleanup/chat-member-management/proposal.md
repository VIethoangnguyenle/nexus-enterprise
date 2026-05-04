# Chat Member Management — Add & Remove Members

## Evidence Summary

- **Proto**: ✅ EXISTS — `messaging.proto` has `AddChannelMember`, `RemoveChannelMember`, `ListChannelMembers` RPCs with full message types
- **Backend gRPC**: ✅ EXISTS — `grpc/server.go` implements both `AddChannelMember` and `RemoveChannelMember`
- **Backend Domain**: ✅ EXISTS — `domain/service.go` has `AddMember()` and `RemoveMember()` with NGAC policy checks
- **Backend REST**: ⚠️ PARTIAL
  - `POST /channels/:chId/members` (AddChannelMember) — ✅ registered
  - `GET /channels/:chId/members` (ListChannelMembers) — ✅ registered
  - `DELETE /channels/:chId/members/:nodeId` (RemoveChannelMember) — ❌ NOT registered
- **Frontend API** (`messaging.ts`):
  - `listMembers()` — ✅ exists
  - `addMember()` — ❌ NOT implemented
  - `removeMember()` — ❌ NOT implemented
- **Frontend Hooks** (`useMessaging.ts`):
  - `useChannelMembers()` — ✅ exists (read-only query)
  - `useAddMember()` — ❌ NOT implemented
  - `useRemoveMember()` — ❌ NOT implemented
- **Frontend UI** (`ChannelInfoPanel.tsx`):
  - `MembersTab` — ✅ exists (read-only list, no add/remove UI)
  - Add Member button/dialog — ❌ NOT implemented
  - Remove Member action — ❌ NOT implemented
- **Dependencies**:
  - Contacts/workspace members list — ✅ `useContacts()` hook exists for user picker
  - NGAC policy — ✅ Backend handles permissions

## Product Assessment

- **Size**: S (Small) — Backend 95% done, need 1 REST route + frontend UI only
- **Risk**: Low — Proto, gRPC, domain all complete. No cross-service changes needed. Existing UI patterns (ChannelInfoPanel) can be extended.
- **Target user**: Workspace Owner/Admin and Channel Creator — manage who can access a channel
- **Core action**: Add workspace members to a channel, remove members from a channel

## Scope

### In scope
1. **Backend**: Add `DELETE /channels/:chId/members/:nodeId` REST endpoint for RemoveChannelMember
2. **Frontend API**: Add `addMember()` and `removeMember()` functions to `messaging.ts`
3. **Frontend Hooks**: Add `useAddChannelMember()` and `useRemoveChannelMember()` mutations with optimistic cache updates
4. **Frontend UI**: 
   - "Add Member" button in MembersTab → opens user picker dialog (search workspace members, select, add)
   - "Remove" action on each member row (for admin/owner only) → confirm → remove
   - Optimistic updates on add/remove (instant UI feedback)

### Out of scope
- Role-based member permissions within channel (viewer/editor) — separate feature
- Bulk add/remove — not needed for MVP
- Member invitation via link — separate feature
- DM member management (DMs are fixed 1:1)

### Deferred
- Channel admin role assignment
- Member permission levels within channel

## Success Criteria
- Admin can add any workspace member to a channel via UI
- Admin can remove a member from a channel via UI
- Changes appear instantly (optimistic UI)
- Member count updates in channel header after add/remove
- Works at all 3 breakpoints (375px, 768px, 1280px)
