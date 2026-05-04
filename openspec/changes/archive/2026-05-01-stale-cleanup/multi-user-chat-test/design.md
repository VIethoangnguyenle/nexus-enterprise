# Design — Multi-User Group Chat Test

## Architecture Context

```
Browser Tab 1 (Alice)  ──┐
Browser Tab 2 (Bob)    ──┼── Vite Proxy :5173 ──→ Workspace :8181
Browser Tab 3 (Carol)  ──┘                    ──→ Messaging :8183
                                              ──→ Drive :8185
                                              ──→ Auth :8180
                         
Each tab: independent localStorage (ngac-auth)
WebSocket: ws://localhost:5173/ws → Messaging :8081 (real-time sync)
```

## Decision 1: Invite Flow Design

**Option chosen: Username-based invite via Settings panel**

The workspace Settings area (accessible from AppRail gear icon) will get an "Invite Member" section:
- Input field for username
- Backend lookup: username → ngac_node_id → invite to workspace
- Currently the invite API requires `ngac_node_id`, so we need a backend helper or frontend-side lookup

**Problem**: Invite API takes `ngac_node_id`, but users only know usernames. 

**Solution**: Add a simple `GET /api/users/lookup?username=X` endpoint to auth service that returns `{ ngac_node_id }`. This is the only backend change needed.

**Alternative considered**: Direct SQL insert — rejected because it bypasses NGAC policy graph and would leave workspace in inconsistent state.

## Decision 2: Multi-tab Testing Strategy

Each browser tab uses separate localStorage via the `ngac-auth` persist key. We need 3 isolated sessions:

1. **Tab 1**: Normal browser → Register Alice → She creates workspace + channel
2. **Tab 2**: Open new tab → Clear localStorage → Register Bob → Alice invites Bob
3. **Tab 3**: Open new tab → Clear localStorage → Register Carol → Alice invites Carol

After invite, Bob and Carol will see Alice's workspace in their workspace list and can navigate to the channel.

## Decision 3: currentUserId Fix

Simple — read from `useAuthStore`:

```typescript
// Before (broken)
const currentUserId = '' // TODO: get from auth context

// After
const currentUserId = useAuthStore(s => s.user?.id ?? '')
```

This fixes:
- Reaction toggle (knows which reactions are "mine")
- Future: message alignment (own messages right-aligned)
- Future: edit/delete permissions on own messages

## Decision 4: UX Audit Approach

During the multi-user test, capture screenshots at each step. After test completion, produce a UX audit artifact listing:
- Visual inconsistencies
- Missing feedback (loading states, success toasts)
- Accessibility gaps
- Layout issues at different content densities
- Color/contrast problems

## File Changes Summary

| File | Change |
|------|--------|
| `frontend/src/api/workspaces.ts` | Add `inviteMember(wsId, username)` and `listMembers(wsId)` |
| `frontend/src/api/auth.ts` | Add `lookupUser(username)` |
| `frontend/src/hooks/useWorkspaces.ts` | Add `useInviteMember` mutation hook |
| `frontend/src/components/InviteMemberForm.tsx` | [NEW] Username input + invite button |
| `frontend/src/routes/_workspace.tsx` | Add invite UI to settings area |
| `frontend/src/routes/_workspace/channels.$channelId.tsx` | Fix `currentUserId` from auth store |
| `backend/services/auth/internal/rest/handler.go` | [NEW] `GET /api/users/lookup?username=X` endpoint |
