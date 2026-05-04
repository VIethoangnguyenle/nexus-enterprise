# Member Management — Spec

## User Stories

### S1: Remove Member from Channel
As a channel admin/creator,
I want to remove a member from the channel,
so that I can manage channel membership.

Acceptance Criteria:
- [ ] Each member row in ChannelInfoPanel shows a "Remove" action (icon or context menu)
- [ ] Clicking "Remove" shows a confirmation dialog
- [ ] Confirming calls DELETE API and removes member from list (optimistic)
- [ ] Removed member no longer appears in the member list
- [ ] Cannot remove self (button hidden or disabled for current user)

Proto mapping: `DELETE /channels/:id/members/:nodeId` (REST — EXISTS)
Frontend-only: YES — API endpoint exists in messaging.ts

## Flow

### Remove Member
1. User opens ChannelInfoPanel → Members tab
2. User sees member list with ONLINE/OFFLINE sections
3. User hovers member row → "Remove" icon appears
4. User clicks Remove → confirmation dialog: "Remove [Name] from #channel?"
5. User confirms → System calls `DELETE /channels/:id/members/:nodeId`
6. Member disappears from list

## States
- **Normal**: Member row with hover action
- **Confirming**: Dialog shown
- **Error**: Toast "Failed to remove member"
