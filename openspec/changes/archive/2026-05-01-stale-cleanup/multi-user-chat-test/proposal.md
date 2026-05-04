# Multi-User Group Chat — E2E UI Test & UX Audit

## Problem

The NGAC messaging system has never been tested with multiple real users interacting simultaneously. Current state:

1. **No "Invite to Workspace" UI** — backend API exists (`POST /api/workspaces/:id/invite`) but frontend has no way to invite users, making multi-user testing impossible through UI alone.
2. **`currentUserId` is hardcoded to `''`** — reactions, read receipts, and "is own message" logic are broken because the channel page never reads the authenticated user's ID.
3. **No multi-user conversation has been visually verified** — avatar color differentiation, message grouping across users, file attachment rendering with different senders, and real-time WebSocket updates between sessions are untested.

## Goal

Build the minimum UI needed to enable multi-user workspace collaboration, then execute a comprehensive 3-user chat test scenario entirely through the browser — acting simultaneously as QA engineer and UX designer to identify both functional bugs and design improvements.

## Scope

### In Scope
- **Frontend**: Invite Member modal/form (username-based invite)
- **Frontend**: Fix `currentUserId` from auth store
- **Frontend**: Fix ThreadPanel close button (remaining `✕` text → Lucide `X` icon)
- **Test**: Full 3-user UI test scenario (register → invite → chat → upload → react → pin)
- **Audit**: UX/UI design review during test, capturing issues for follow-up

### Out of Scope
- Backend changes (all APIs already exist)
- Permission/role management UI
- Channel-level member management (workspace-level invite is sufficient)
- Fixing UX issues found during audit (captured as separate future change)

## Success Criteria

1. Three users (Alice, Bob, Carol) can register, join the same workspace, and see the same channel — all through UI
2. Messages from all 3 users render correctly with distinct avatars and proper ordering
3. File uploads from different users display correctly with FilePreviewCard
4. Reactions toggle correctly per-user (each user sees their own reaction state)
5. UX audit document produced with screenshots of issues found
