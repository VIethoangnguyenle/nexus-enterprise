---
phase: 5
title: "DM and call gate centralised: DMs, calls, event calls only between workspace-mates (F4)"
status: completed
priority: P2
effort: "4h"
dependencies: [1, 3]
---
# Phase 5: DM and call gate (F4)

## Overview
Decision 2: DMs and calls only between people who share a workspace. `FindOrCreateDM`
(`messaging/internal/domain/dms.go:57-86`) adds the client's `target_ngac_node_id` unchecked (any type, any tenant);
DMs live under PC_Global (`channels.go:170-188`), so their `write` grant survives workspace removal. Many DM write paths
check `write` on their own (polls `polls_tasks.go:105`, tasks `:265,391`, reactions/pins
`reactions_pins_receipts.go:22,33,71,91`, edit `lifecycle.go:43-48`, call token/join `calls.go:248`, StartCall early
return into an open call `calls.go:173-200`, ringing `calls_push.go:70`, eviction `mayStayInCall` `calls.go:536`).

## Requirements
- **Create** (`FindOrCreateDM`): resolve the target node server-side with `authClient.GetUserByID`
  (`messaging/internal/domain/service.go:126-135`; `UserInfo.ngac_node_id`, `proto/auth/auth.proto:58`), reuse for the
  title; client node, if sent, must match (`ErrInvalidInput`); self-DM → `ErrInvalidInput`; node type `U`;
  `policyclient.SharedWorkspaces(caller, target)` ≠ ∅ else `ErrAccessDenied`; only then find/create (no lookup oracle).
- **One choke point** in `messaging/internal/domain/authz.go`: `authorizeChannel` (`:46`) and `authorizeMessage`
  (`:65`) — when `ch.ChannelType=="dm"` and op ∈ {`write`, `invite`, `manage`}, additionally require
  `SharedWorkspaces(user, other DM member)` ≠ ∅ (other member from `channel_members`; policy error → deny). Reads stay
  graph-only (history readable, Q4).
  - Route the direct `checkAccess(..., ch.NGACOaID, OpWrite|OpManage)` call sites through a DM-aware
    `authorizeChannelOp(ctx, ch, node, op)`: `polls_tasks.go:369,391`, `calls.go` CallToken/StartCall/ticket paths,
    `calls_push.go:70` InviteToCall, `lifecycle.go` delete-by-manage path. List verified with
    `grep -n "checkAccess(ctx" messaging/internal/domain/*.go` at implementation; every write/manage on a channel OA
    goes through the helper.
  - `mayStayInCall` (`calls.go:536`) uses the same rule for DM calls (an ex-colleague is evicted on the next sweep).
  - SendMessage: the gate applies through `authorizeChannel` for every message type (not only inside the
    `msgType=="user"` block, `messages.go:81-85`).
- **Event calls** (`calls_event.go:41-66` `authorizeEvent`): add `IsWorkspaceMember(user, ref.WorkspaceID)` before the
  read check (no longer trusting the JWT tenant alone). Event guests are already members (calendar `requirePerson`);
  detach strips guest UAs.
- **Out of scope (choice 5)**: DM drive uploads (`channel_drive.go:79-82` grants `ChannelDriveOps` to the DM UA; the
  drive service has no DM-relationship context). Recorded as a follow-up with the cross-org work.
- Frontend sends both ids today; server derives the node → no frontend change.

## Architecture
```
CreateDM ─> node from user id ─> type U ─> SharedWorkspaces ≠ ∅ ─> find/create
any DM write|invite|manage (send, poll, task, react, pin, edit, call start/token/ring, stay) ─> authorizeChannel/Message ─> SharedWorkspaces ≠ ∅
event call ─> IsWorkspaceMember(event workspace) ─> read on event OA
```
Cost per DM write: `MemberWorkspaces` ×2 (2 `GetAncestors`, cached Members ids).

## Related Code Files
- Modify: `messaging/internal/domain/authz.go`, `dms.go` (FindOrCreateDM), `calls.go`, `calls_push.go`,
  `calls_event.go`, `polls_tasks.go`, `reactions_pins_receipts.go`, `lifecycle.go` (call sites only), `messages.go`,
  `rest/handler.go` (CreateDM requires `target_user_id`), `grpc/server.go` (CreateDM)
- Create: `messaging/internal/domain/dm_relationship_realpolicy_test.go` (tag `realpolicy`)
- Spec: `docs/specs/resource-pep-coverage/spec.md` ("A direct conversation needs a shared workspace for any write"),
  `docs/specs/chat-calls/spec.md` (DM and event calls need a shared workspace / membership)

## Implementation Steps
1. Deny tests first, paired allows (both active in W): create with cross-tenant target, UA target, mismatched ids,
   channel-only shared → DENY; for an ex-colleague in an existing DM: send, poll, task, reaction, pin, edit, CallToken,
   StartCall into an open call, InviteToCall → DENY; history read → ALLOW; `mayStayInCall` false. Event call by a removed
   member → DENY; by a member guest → ALLOW.
2. Implement.
3. `make test s=messaging`; `make test-realpolicy`; `make check-layering`.
4. Docker stack: two users in different orgs only → `POST /api/dms` 403.

## Success Criteria
- All listed DM write paths covered by failing-then-passing tests; `grep` shows no direct `checkAccess` of write/manage
  on a channel OA outside the helper.

## Risk Assessment
| Risk | L×I | Mitigation |
|------|-----|------------|
| Latency on every DM write | M×M | cached Members ids; measure p95 send on dev; acceptable per decision |
| Missed write path | M×H | single choke point + grep criterion |
| DM drive still writable | M×L | declared out of scope (choice 5); follow-up |
| Users with NULL `users.ngac_node` | L×L | NotFound, same as today |

## Rollback
Code-only; revert.

## Implementation Status (2026-10-17)
- [x] Deny tests first (paired allows), seen red for the right reason, then implemented
      (`dm_relationship_realpolicy_test.go`, `dm_relationship_test.go`).
- [x] `FindOrCreateDM`: target resolved from the user id (`GetUserByID`), sent node must match, no self-DM, must exist,
      shared workspace (writer) before any lookup. `store.DMPeerNodes` (new file `store/dm_peers.go`) reads the other member.
- [x] One choke point `authorizeChannelOp` (`authz.go`), used by `authorizeChannel`/`authorizeMessage`, `SendMessage`
      (all message types, not only `user`), `UpdateTask` manage/write, `InviteToCall` invitees; votes (read-level) get
      `requireDMRelationshipIn`. `mayStayInCall` follows through `authorizeChannel`.
- [x] Event calls: `IsWorkspaceMember` before the read check (`calls_event.go`).
- [x] Specs: `chat-calls`, `resource-pep-coverage`.
- [x] Review fixes: a manager's deletion notice is stored directly (no `write` needed); task status/reassignment in a DM
      needs the relationship; backing out (decline, cancel, delete own message, remove own reaction/vote) is exempt.
- Decisions: the per-write gate uses the same policy connection as the `write` check beside it. Messaging dials `POLICY_SERVICE_ADDR`
  (the writer) for both its read and write clients today, so there is no replica lag now; if it moves to `policy-read`, the
  DM gate should ask the writer.
  `handler.go`/`server.go` unchanged: the domain enforces `target_user_id` (400). Frontend unchanged: a 403 already reads as words.
- Not done (declared): DM drive uploads (choice 5).
- Follow-up (not done): `DMPeerNodes` filters `tenant_users` by `ngac_node_id`; add an index on `tenant_users.ngac_node_id` if it shows up in profiles.
