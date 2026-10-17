---
phase: 8
title: "Messaging low items: channel RemoveMember, gRPC message type, DM listing (F8, F9, P1)"
status: completed
priority: P3
effort: "2h"
dependencies: [3, 5]
---
# Phase 8: Messaging low items (F8, F9, P1)

## Overview
- F8: `RemoveMember` (`messaging/internal/domain/members.go:86-104`) — any member (holds `invite`) removes anyone,
  incl. the creator; no DM guard (AddMember has one, `:35`); `channel_members` cache not updated, so
  `FindDMByMembers` (`store.go:133`) keeps returning a DM the removed party cannot read. Dev DB: 6 cache rows with
  no graph assignment.
- F9: gRPC `SendMessage` passes `req.MessageType` (`grpc/server.go:104`); `domain/messages.go:81-85` checks write
  only for `"user"`. No internal gRPC caller of `SendMessage` exists; system lines are produced in-process
  (`polls_tasks.go:136,295`, `calls.go:204`, `lifecycle.go:152`).
- P1: `ListDMs` (`dms.go:19-25`) loads every DM (`store.go:106` `ListAllDMs`) then batch-checks them all.

## Requirements
- F8 `RemoveMember(ctx, channelID, requester, target)`:
  1. channel must not be `dm` or `workspace` (`#general` membership follows workspace membership, Phase 02) →
     `ErrInvalidInput`.
  2. target must be a `U` node directly assigned to the channel UA (else `ErrNotFound`).
  3. allowed when requester == target (leave), or requester == `ch.CreatedBy`'s node, or requester holds `manage`
     on the content OA (owners via Channels OA after Phase 02). Otherwise `ErrAccessDenied`. (Q2 in plan.md.)
     Creator cannot be removed by others except a `manage` holder.
  4. after `RemoveAssignment`: delete the cache row (`store.DeleteChannelMember`, new, beside
     `InsertChannelMember` `store.go:153`); cache delete failure → logged + returned (graph already correct;
     retry idempotent).
  5. Existing stale cache rows are deleted by Phase 03's `061` (it owns the cleanup); nothing to migrate here.
- F9: gRPC server forces `MessageType: "user"` (drop the field from the domain input mapping); proto field left in
  place (removing it is a cross-side proto change for no gain).
- P1: `ListDMs` uses `store.ListDMsOf(ctx, userNodeID)` (`store.go:119`, already tested
  `store/channels_test.go:181`), keeping `filterAccessible` as the authority (cache may be stale; graph decides).
  `ListAllDMs` deleted if no other caller (`grep` first).

## Architecture
```
RemoveMember: not DM ─> target is U in Ch UA ─> self | creator | manage ─> RemoveAssignment (EPP) ─> delete cache ─> revoke subs ─> event
SendMessage(gRPC): type := "user" ─> write check always
ListDMs: channel_members(node) ─> BatchCheck read
```

## Related Code Files
- Modify: `backend/services/messaging/internal/domain/members.go` (RemoveMember), `messaging/internal/store/store.go`
  (+DeleteChannelMember, −ListAllDMs), `memberCache` interface (`messaging/internal/domain/service.go:22-25`,
  +`DeleteChannelMember`), `messaging/internal/domain/dms.go` (ListDMs), `messaging/internal/grpc/server.go` (SendMessage)
- Create: `messaging/internal/domain/remove_member_realpolicy_test.go` (tag `realpolicy`),
  `messaging/internal/grpc/send_message_type_test.go`
- Spec: `docs/specs/tenant-ngac-init/spec.md` (channel removal rules),
  `docs/specs/resource-pep-coverage/spec.md` (system messages only from the service itself). P1: no spec (no
  behaviour change).

## Implementation Steps
1. Deny tests first: plain member removes another member → DENY; anyone removes from `#general` → `ErrInvalidInput`; member removes creator → DENY; DM participant
   removes the other → `ErrInvalidInput`; target = a UA → `ErrNotFound`; gRPC `SendMessage` with
   `message_type:"system"` by a non-member → DENY. Allow: self-leave, creator removes member, owner (manage)
   removes member; cache row gone afterwards; `FindDMByMembers` no longer returns a removed DM.
   P1: `ListDMs` returns only the caller's DMs (store-level test exists; add domain test with 2 users).
2. Implement.
3. `make test s=messaging`; `make test-realpolicy`; `make check-layering`.

## Success Criteria
- Tests green; `grep -n "ListAllDMs" backend/services/messaging` → 0 (or justified remaining caller).

## Risk Assessment
| Risk | L×I | Mitigation |
|------|-----|------------|
| Members used "remove" to clean up groups | M×L | creator/owner can; self-leave allowed |
| Stale cache hides a DM in ListDMs | L×M | cache written atomically with channel (`InsertChannelWithMembers`); DM membership never changes after F8 guard |

## Rollback
Code-only; revert.

## Status (implemented, not committed)
- [x] Deny tests first, paired allows (`remove_member_realpolicy_test.go`, `send_message_type_test.go`, store `TestDeleteChannelMember_DropsOnlyThatRow`, fake-based tests in `member_cache_test.go`), seen red before the change.
- [x] F8: `RemoveMember` = inside-or-manager gate, DM/`#general` refused (`ErrInvalidInput`), self | creator | manage, target a `U` directly in the group (`ErrNotFound`), then `RemoveAssignment`, `DeleteChannelMember` (new), revoke, announce; a cache failure is returned after the announce.
- [x] F9: gRPC `SendMessage` no longer maps `message_type`.
- [x] P1: `ListDMs` reads `ListDMsOf`; `ListAllDMs` deleted (`grep` = 0).
- [x] Specs: `tenant-ngac-init` (removal rule), `resource-pep-coverage` (message type ignored over gRPC).
- Deviations: (1) a DM cannot be removed from, so "`FindDMByMembers` no longer returns a removed DM" is tested as "a refused removal leaves the DM found". (2) Owners hold `manage`, not `invite`/`read`, so the gate is read-or-manage and the old `authorizeChannel(invite)` is not used. (3) Fixtures: `mockPolicyWriteClient` gained `GetNode`/`IsAssigned`; two older RemoveMember tests now use a private group and script the edge.

### Review fixes (same worktree, uncommitted)
- [x] M2: members list returns `can_remove_members` (creator or `manage`; false in DM and `#general`); `MembersPanel` shows "Xoá khỏi nhóm" only by it; optimistic add/remove keep the flag; 400 on removal reads in words (`explainRemoval`). vitest + backend deny/allow tests.
- Follow-up (not done on purpose): **Leave group action needs a DESIGN.md/mockup entry first.** The API already lets a member leave; the screen has no action for it.
