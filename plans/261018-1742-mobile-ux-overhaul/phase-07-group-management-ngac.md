---
phase: 7
title: "Group management"
status: completed
priority: P1
effort: "3.5d"
dependencies: [6]
---

# Phase 07: Group management

## Overview
Group owners and group managers decide who may post at the top level, add people and mention everyone; managers
are appointed and removed; ownership can be transferred and a group disbanded. Graph changes go through the PDP
with spec + deny test vectors in the same commit; message-level rules are enforced in messaging and stated as such
in the spec.

## Requirements
- Functional: roles Chủ nhóm (exactly one = `channels.created_by`), Quản trị nhóm (0..n), Thành viên; settings
  "Ai được gửi tin", "Ai được thêm người", "Ai được nhắc @Tất cả", each Mọi thành viên | Chỉ chủ nhóm và quản trị
  nhóm; when posting is restricted members still post replies in threads (Zane 2026-10-19) and keep reactions,
  pins, polls, calls, editing and withdrawing their own messages; add/remove manager; transfer ownership; disband
  (owner only); last owner cannot leave without transferring.
- @Tất cả is a **new capability** (no @all exists today): one mention that notifies every member, checked on send,
  reply **and** edit (an edit that adds @all is refused when restricted). Only workspace administrators may use it
  in the workspace-wide channel; DMs refuse it (`chat-mention-all`).
- Starting a poll, task or approval card counts as a top-level post (refused for non-managers when posting is
  restricted); voting and closing one's own poll stay open. Only the system's own message types (system lines) are
  refused from a person; approval cards made through the approval flow follow the posting rule.
- Every existing `manage` check in messaging (rename `channels.go:286`, picture `channel_avatar.go:88`, delete
  others' message `lifecycle.go:125`, close others' poll `polls_tasks.go:231`, edit others' task `polls_tasks.go:391`,
  remove member `members.go:151,160`, `can_remove_members` `members.go:242`) stays for workspace owners and also
  opens to the group owner and managers via the Managers-UA test (the spec lists them).
- Non-functional: workspace owners keep their current `manage` on Channels (those moderation checks, without reading)
  and get no group-manager power from it (settings, appoint, @Tất cả, transfer, disband); DMs have no management; demotion/removal takes effect at once (EPP invalidation +
  a realtime event that clears the client's cached permissions).

## Architecture
- New per-group UA `Ch_<id>_Managers` (helper `ngac.ChannelManagersUAName(ngac.ChannelID)`), a **sibling** of
  `Ch_<id>_Members`; managers keep their direct user → Members edge so roster, removal and `061` reconcile rules
  hold. Managers UA → Content OA association `{invite}` only (members' rights come from Members as today).
- Manager authority (appoint, settings, remove members, @all when restricted) = "caller is assigned to
  `Ch_<id>_Managers`" read from the graph by messaging, **never** a `manage` decision (red team #3). Owner checks use
  `created_by`; the existing `isChannelCreator` shortcut becomes the owner check and moves with a transfer.
- "Ai được thêm người = quản trị": Members → Content association loses `invite` (Managers keeps it).
- "Ai được gửi tin = quản trị": members keep `write`; messaging refuses a message with no parent from a non-manager
  when the setting is on (polls, tasks and approval cards count as top-level posts), and refuses the system's own
  message types (system lines) sent by a person into a group (closes the system-line spoof).
- Remove/leave/demote/transfer removes **all** of the person's group edges in one policy write through EPP.
- Settings in reserved migration `069_channel_management.sql` (columns on `channels`, idempotent). Backfill runs in
  Go through the policy writer (invalidating): owner stays `created_by` only if that user is currently assigned to
  Members in `ngac_assignments`; otherwise the earliest currently assigned member becomes owner.

## Related Code Files
- Modify: `backend/ngac/ngac_ops.go` (+ name helper, op-set helper), `backend/ngac/channel_ops_test.go`, `backend/services/messaging/internal/domain/{members.go,messages.go,lifecycle.go,channels.go}`, messaging store/rest, `frontend/src/components/spaces/MembersPanel.tsx`, `frontend/src/hooks/usePermissions.ts` (invalidate on event), `frontend/vite.config.js`, `deploy/nginx/default.conf` if a new prefix appears
- Create: `data/migrations/069_channel_management.sql`, backfill command, `backend/services/policy/internal/ngac/group_management_test.go` (vectors), `frontend/src/components/spaces/GroupManagement.tsx`, tests
- Specs: `chat-group-management`, `chat-mention-all` (new), `tenant-ngac-init` (Managers UA, rows at `:160`, `:176`, `:211`, marked superseded), `chat-threads`

## Implementation Steps
1. Specs (phase 01) with the allow/deny table and which rules messaging enforces vs the PDP; run the `ngac-policy-change` skill checklist.
2. Tests first, deny first: restricted member posts top-level → DENY, replies in thread → ALLOW, reacts/pins/votes/edits own → ALLOW; system-line type from a person → DENY; restricted member invites → DENY; restricted member sends or edits in @all → DENY; workspace owner not in group appoints self manager → DENY, sends @all → DENY, reads → DENY, but renames and removes a member → ALLOW as today; plain member removes someone → DENY, owner/manager removes → ALLOW; former owner after transfer who is also demoted removes someone → DENY; restricted member starts a poll, task or approval card → DENY, votes and closes own poll → ALLOW; plain member appoints/removes a manager, transfers or disbands → DENY, owner transfers/disbands → ALLOW, manager disbands → DENY; non-administrator sends @all in the workspace-wide channel or anyone in a DM → DENY, administrator in the workspace-wide channel → ALLOW; reply by a non-member → DENY; demoted manager invite → DENY; removed or demoted manager → read/invite DENY after invalidation; creator who left before migration → read DENY after backfill.
3. Back up DB; `069` migration; `make db-migrate` twice; run the backfill; verify on the dev DB.
4. Domain + EPP writes + realtime permission event; REST; Quản lý nhóm UI and member roles.
5. Full per-path deny regression matrix across every messaging route touched.

## Success Criteria
- [ ] Spec + policy test vectors in the same commit as the graph change; every decision has a deny test
- [ ] Strict `make test s=policy`, `s=messaging`, `s=drive` green; `make check-ngac` green
- [ ] Demoted manager loses Quản lý nhóm in the open client without reload (e2e)

## Risk Assessment
Backfill on production data → dry-run mode printing the owner chosen per group; run on a DB copy first.

## Rollback
Revert the release; before it, reset every group's settings to Mọi thành viên (restores Members `invite`) with the
admin command, so the old version, which cannot show these settings, is not left with restricted groups. The
Managers UAs are harmless to the old version (no association grants it anything it lacked).
