# chat-group-management

## Purpose
Lets the owner and managers of a group chat decide who may post, add people and mention everyone, appoint and remove
managers, hand the group over and disband it, without giving anyone who is not in the group a way in.

## Status
Implemented (phase 07 of `plans/261018-1742-mobile-ux-overhaul`). The code lives in
`backend/services/messaging/internal/domain/group_management.go`, `backend/services/messaging/internal/domain/members.go`
and `backend/ngac/channel_management.go`.

## Requirements

### Requirement: Roles in a group come from the graph and the creator column
A group (channel type `private`) SHALL have exactly one **owner** — the user recorded in `channels.created_by` — and zero
or more **managers** — the users assigned to the group's Managers UA `Ch_{channel_id}_Managers` (name from
`ngac.ChannelManagersUAName`). The Managers UA SHALL sit **beside** `Ch_{channel_id}_Members`, never inside it; every
manager keeps their own direct user → Members edge, so the roster, removal and the membership reconcile keep treating
only direct user children of Members as members. The owner is always a manager. Direct messages and the
workspace-wide channel have no owner or managers.

Manager authority SHALL be decided by "the caller is assigned to `Ch_{channel_id}_Managers`", read from the graph, and
never by a `manage` decision: workspace owners hold `manage` on every group through the Channels OA and must not gain
group powers from it.

#### Scenario: Workspace owner outside the group
- **WHEN** a workspace owner who is not in a private group tries to appoint themselves manager, change its settings or
  send `@Tất cả` in it
- **THEN** each is denied, and they still cannot read the group

#### Scenario: Manager removed from the group
- **WHEN** a manager leaves or is removed
- **THEN** both their Members and Managers edges are withdrawn in one write through the EPP path, and read, write and
  invite on the group's Content OA resolve to DENY for them

### Requirement: Every decision names who makes it and who may pass
Two kinds of decision exist. **PDP decisions** are graph reachability questions answered by the policy service (`read`,
`write`, `invite` on the group's Content OA). **Messaging-enforced rules** concern things that are not graph nodes
(messages, settings, roles); messaging SHALL enforce them by reading the graph (Managers UA membership) and
`channels.created_by`, and SHALL NOT use a `manage` decision for any of them.

| Decision | Kind | Allowed | Denied |
|---|---|---|---|
| Read the group, send a reply, react, pin, vote, call, edit or withdraw one's own message | PDP (`read`, `write`) | members | non-members, including workspace owners |
| Add people | PDP (`invite`) | members (managers only when "Ai được thêm người" is restricted) | non-members; plain members when restricted |
| Top-level post (no parent), including starting a poll, a task or an approval card | messaging | managers and owner; every member when "Ai được gửi tin" is unrestricted | plain members when restricted |
| `@Tất cả` on send, reply, edit | messaging | managers and owner; every member when unrestricted | plain members when restricted; everyone in a DM and for non-administrators in the workspace-wide channel (`chat-mention-all`) |
| Change group settings | messaging | owner, managers | members, workspace owners outside the group |
| Appoint or remove a manager | messaging | owner, managers | members, workspace owners outside the group |
| Transfer ownership | messaging | owner | managers, members |
| Disband | messaging | owner | managers, members |

#### Scenario: Reply by a person who is not in the group
- **WHEN** someone who is not a member of the group replies in one of its threads
- **THEN** the PDP denies `write` and nothing is stored

### Requirement: Existing `manage` checks stay with workspace owners and also open to the group's owner and managers
Workspace owners SHALL keep every moderation check messaging makes today with `manage` on the group's Content OA, because
that is current behaviour and an administrator's need. They gain no group-management power (settings, appointing
managers, `@Tất cả`, transfer, disband) and no read access from it. The group's owner and managers SHALL pass the same
checks through the Managers UA test.

| Existing check | Source | Held today by | After this capability |
|---|---|---|---|
| Rename the group | `domain/channels.go:286` | workspace owners | workspace owners; owner, managers |
| Change the group picture | `domain/channel_avatar.go:88` | workspace owners | workspace owners; owner, managers |
| Delete someone else's message | `domain/lifecycle.go:125` | workspace owners | workspace owners; owner, managers |
| Close someone else's poll | `domain/polls_tasks.go:231` | workspace owners | workspace owners; owner, managers |
| Edit someone else's task | `domain/polls_tasks.go:391` | workspace owners | workspace owners; owner, managers |
| Remove a member | `domain/members.go:151,160` | the person, the creator, workspace owners | the person; owner; managers; workspace owners (never the group's owner) |
| `can_remove_members` flag on the roster | `domain/members.go:242` | the creator, workspace owners | owner, managers, workspace owners |

The creator shortcut (`isChannelCreator`) becomes the owner check and moves with a transfer.

#### Scenario: Workspace owner moderates without reading
- **WHEN** a workspace owner who is not in a group renames it or removes a member
- **THEN** it succeeds as today, and the same owner still cannot read the group or change its settings

### Requirement: Group settings decide who may post, add people and mention everyone
Each group SHALL have three settings, each "Mọi thành viên" (default) or "Chỉ chủ nhóm và quản trị nhóm":
- **Ai được gửi tin:** when restricted, a message with no parent (a top-level post) from a non-manager SHALL be refused by
  the messaging service; replies in threads, reactions, pins, polls, calls, editing and withdrawing one's own message
  stay open to every member (members keep `write`). This rule is enforced by messaging, not the PDP, because messages are
  not graph nodes.
- **Ai được thêm người:** when restricted, the Members → Content association SHALL lose `invite` (Managers keep it), so
  the PDP refuses a member's invite. The rule is stored twice, in `channels.invite_policy` and in that association, by
  two separate writes. Every save that carries the rule SHALL write the association again (an upsert), whatever the row
  already holds, so a pair that drifted apart (a row write that failed after the grant, two managers saving at once) is
  repaired by saving the rule. The rule the group reports (`GET …/management`) SHALL be read from the association
  (`ngac.InviteRule`), what the PDP enforces; the row stands in only when the association cannot be read or is missing,
  and that is logged.
- **Ai được nhắc @Tất cả:** when restricted, messaging SHALL refuse an `@Tất cả` mention from a non-manager on send,
  reply and edit (see `chat-mention-all`).
Changing a setting SHALL require manager authority and SHALL go through the EPP path when it changes the graph.

#### Scenario: Restricted posting
- **WHEN** posting is restricted and a member sends a top-level message
- **THEN** it is refused; the same member's reply in a thread, reaction and edit of their own reply succeed

Starting a poll, a task or an approval card counts as a top-level post (`checkTopLevelPost`): it is refused for a non-manager when posting is
restricted. Replies inside a thread, voting in a poll and closing one's own poll stay open to every member.

#### Scenario: Restricted posting starts a card
- **WHEN** posting is restricted and a member starts a poll, a task or an approval card in the group's main stream
- **THEN** it is refused; the same member's vote in a manager's poll and closing their own earlier poll succeed

#### Scenario: Manager or unrestricted posting
- **WHEN** a manager sends a top-level message while posting is restricted, or a member does while it is unrestricted
- **THEN** it is delivered

#### Scenario: A person sends a system line
- **WHEN** a person (not the system) sends a message of one of the system's own types (system lines) into a group
- **THEN** it is refused whatever the settings; an approval card created through the approval flow is not refused for
  its type and follows the posting rule above

#### Scenario: Restricted adding
- **WHEN** adding people is restricted and a member adds someone
- **THEN** the PDP denies `invite`; a manager adding the same person succeeds

#### Scenario: Restricted mention of everyone
- **WHEN** `@Tất cả` is restricted and a member sends it, replies with it or edits it into a message
- **THEN** each is refused; a manager sending it succeeds

#### Scenario: Drifted adding rule
- **WHEN** the row says everyone may add people but the Members → Content association lacks `invite`
- **THEN** the group reports "Chỉ chủ nhóm và quản trị nhóm" (the enforced rule), and an owner or manager saving
  "Mọi thành viên" gives the members `invite` back; a plain member's save is refused and writes nothing

#### Scenario: Demoted manager
- **WHEN** a manager is demoted while adding people is restricted
- **THEN** their `invite` resolves to DENY as soon as the write is invalidated

### Requirement: Managers are appointed and removed by the owner and managers
The owner and managers SHALL be able to appoint a member as manager and remove a manager (not the owner). Only members of
the group can be appointed.

#### Scenario: Owner or manager appoints
- **WHEN** the owner or a manager appoints a member as manager
- **THEN** the person is added to the Managers UA in one write through the EPP path

#### Scenario: Member appoints someone
- **WHEN** a member who is not a manager appoints a manager
- **THEN** it is denied and nothing is written

#### Scenario: Removing a manager
- **WHEN** the owner or a manager removes a manager, and separately a plain member tries the same
- **THEN** the first succeeds and the person stays a member; the second is denied and nothing is written

### Requirement: Ownership moves and groups end only by the owner
Only the owner SHALL transfer ownership (to another current member of the group, who becomes a manager) or disband the
group. The target's user ID is resolved from the node ID server-side (`store.UserIDByNode`). Both the new and old owner
are assigned to the Managers UA before `channels.created_by` moves, so a failure leaves the owner where they were and a
retry finishes the job. After a transfer, the former owner stays a manager but SHALL hold no owner power, including the
creator's right to remove people. The owner SHALL NOT leave the group before transferring ownership.
Transferring to oneself is refused.

#### Scenario: Transfer
- **WHEN** the owner transfers the group to a member, and separately a non-owner tries to transfer it
- **THEN** the first succeeds; the second is denied and nothing is written

#### Scenario: Disband
- **WHEN** the owner disbands the group, and separately a manager tries to
- **THEN** the first succeeds: the group's drive (the `drive_context = 'channel'` root made for it, with everything in it)
  is first moved to the trash through the drive service's `TrashItem`, as the owner (the drive checks their `write`);
  then the channel row is deleted (cascades to the members cache), and the Managers UA, the Members UA and the Content OA
  are removed from the graph through the policy writer (`cleanupChannelGraph`); a node that cannot be deleted is logged
  and left, granting nothing once its channel no longer exists. The second attempt is denied and nothing, the drive
  included, is touched

#### Scenario: Disband while the drive cannot be trashed
- **WHEN** the drive service cannot find or trash the group's drive
- **THEN** the disband is refused with 503 ("the group's drive could not be moved to the trash; try again") and the
  group, its nodes and its drive all stay; a retry finishes, and a drive already in the trash is not trashed again.
  Nothing is deleted for good: the trashed items keep their rows, their stored objects and their share of the
  workspace quota (the drive releases quota only on permanent deletion). Once the group's nodes are gone nobody holds
  `write` on the drive's OA, so restoring or purging them is an operator's job in the database; no API reaches them

#### Scenario: The group is already gone
- **WHEN** a second disband, a transfer or a settings save reaches a group whose row was deleted a moment earlier
- **THEN** the answer is 404, not 500

#### Scenario: Former owner after a transfer
- **WHEN** ownership has moved from A to B and A tries to remove B or disband the group
- **THEN** both are denied, while A can still do what any manager can

#### Scenario: Owner leaves without transferring
- **WHEN** the owner tries to leave
- **THEN** it is refused with a message asking to transfer ownership first

### Requirement: Removing a member needs the person, the owner or a manager
Removing someone from a group SHALL be allowed to the person themselves (leaving), the owner and managers (not the
owner), and workspace owners as today through `manage`. Anyone else SHALL be denied. After a transfer the former owner
has no creator shortcut. Direct messages and the workspace-wide channel keep refusing removal.

Nobody SHALL remove the group's owner, a workspace owner included: a group whose owner is gone could never be handed over
or disbanded, since both need the owner. A holder of `manage` (a workspace owner, inside the group or not) who tries is
refused with 409 "the group's owner cannot be removed; transfer ownership first", and nothing is written; anyone else gets
403. The owner leaving is the leave rule's own 409 (transfer first). Once ownership has moved, the former owner is a
manager like any other and may be removed. (Decision of 2026-10-19: refuse rather than move ownership automatically.)

#### Scenario: Owner or manager removes a member
- **WHEN** the owner, or a manager, removes a member
- **THEN** the member's Managers edge is withdrawn first, then the Members edge; a failure in between leaves the person a
  member (and still a manager), the call reports the error and repeating it finishes the removal; once both are removed,
  the person can no longer read the group

#### Scenario: Plain member removes someone
- **WHEN** a member who is not a manager removes another member
- **THEN** it is denied and nothing is written

#### Scenario: Workspace owner removes a member
- **WHEN** a workspace owner who is not in the group removes a member
- **THEN** it succeeds as today, and they still cannot read the group

#### Scenario: Workspace owner removes the group's owner
- **WHEN** a workspace owner, inside the group or not, removes the group's owner
- **THEN** it is refused with 409 asking to transfer ownership first; the owner keeps their Members and Managers edges and
  `channels.created_by` is unchanged. After the owner hands the group over, the same removal of the former owner succeeds

#### Scenario: Former owner removes a member
- **WHEN** ownership has moved from A to B and A, now a manager, removes a member
- **THEN** it succeeds only because A is a manager; with A also demoted it is denied, since the creator shortcut moved with the transfer

### Requirement: Existing groups keep working when roles arrive
Existing groups SHALL get settings "Mọi thành viên" and an owner: `created_by` if that user is currently assigned to the
group's Members UA, otherwise the earliest currently assigned member (by `channel_members.joined_at`, then
`users.created_at`, then `users.id`). A group with nobody assigned keeps its creator column and gets no owner node.

The backfill is the standalone command `backfill-managers` (`backend/services/messaging/cmd/backfill-managers/`), run
once per environment as a deploy step (`docs/deployment.md`, "One-off: group managers backfill"). It lists every private
channel oldest first, applies the owner rule, creates the Managers UA (`Ch_{id}_Managers`) beside Members under the
workspace's policy class when it is missing, and then, on every run whether the UA was just made or found, writes the
UA's assignment under the policy class, the `ChannelManagerOps` (invite) grant on the group's Content OA, the Members →
Content grant from the group's `invite_policy` (`ChannelMemberOps`, or `ChannelMemberOpsRestricted` when restricted), and
the owner's assignment to the Managers UA. Each write is an upsert, so a run that failed part-way (the UA made, a later
edge not) is finished by the next. Graph writes go through the policy writer so decisions are invalidated. A group whose
creator left before the backfill gets a different owner. Failed groups are listed and make the command exit non-zero.
The command supports `-dry-run` to print what would change without applying.

The command has no user. It dials the policy as the service identity `backfill-managers`, which the policy admits on
exactly the writer's `FindNodeByName`, `CreateNode`, `CreateAssignment` and `CreateAssociation`
(`backend/services/policy/internal/grpc/authpolicy.go`); every other RPC, and those four from any other service
identity, stay refused.

#### Scenario: Creator who left before the backfill
- **WHEN** a group's creator had left before roles arrived
- **THEN** another current member becomes owner and the creator still cannot read the group

#### Scenario: Backfill rerun
- **WHEN** the backfill runs again on a group that already has its Managers UA
- **THEN** the existing UA is reused, the owner's assignment is idempotent, and no second copy is created

#### Scenario: Backfill rerun after a partial failure
- **WHEN** a run created a group's Managers UA and then failed before its PC edge or its grant was written
- **THEN** the next run finds the UA by name and writes the PC edge, the managers' grant, the members' grant and the
  owner's edge; the owner may add people again and a plain member of a restricted group still may not

#### Scenario: Another service identity runs the backfill
- **WHEN** the backfill dials as any service identity other than `backfill-managers`
- **THEN** the policy answers Unauthenticated and nothing is written

### Requirement: Clients learn about role changes at once
A change of roles or settings SHALL publish a realtime event (`management_changed`) that makes clients refresh their
cached permissions for that group; an action refused because the client's permission was stale SHALL read "Bạn không còn
quyền làm việc này".

### Requirement: Appoint, remove-manager and transfer take user IDs resolved server-side
The `appoint-manager`, `remove-manager` and `transfer` endpoints accept user IDs (the IDs the roster gives each member).
The server resolves these to NGAC node IDs through `store.UserNode` (for the owner check) and `store.UserIDByNode` (for
the transfer target), so the client never needs to know node IDs.
