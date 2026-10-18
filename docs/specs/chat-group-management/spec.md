# chat-group-management

## Purpose
Lets the owner and managers of a group chat decide who may post, add people and mention everyone, appoint and remove
managers, hand the group over and disband it, without giving anyone who is not in the group a way in.

## Status
Specified ahead of code (2026-10-19, plan `plans/261018-1742-mobile-ux-overhaul`, phase 07). Until it lands, the rules in
`tenant-ngac-init` ("Any member of the group may add people"; "Only the person, the creator or a holder of `manage`
removes someone") are what the code does.

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
| Remove a member | `domain/members.go:151,160` | the person, the creator, workspace owners | the person; owner; managers; workspace owners |
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
  the PDP refuses a member's invite.
- **Ai được nhắc @Tất cả:** when restricted, messaging SHALL refuse an `@Tất cả` mention from a non-manager on send,
  reply and edit (see `chat-mention-all`).
Changing a setting SHALL require manager authority and SHALL go through the EPP path when it changes the graph.

#### Scenario: Restricted posting
- **WHEN** posting is restricted and a member sends a top-level message
- **THEN** it is refused; the same member's reply in a thread, reaction and edit of their own reply succeed

Starting a poll, a task or an approval card counts as a top-level post: it is refused for a non-manager when posting is
restricted. Voting in a poll and closing one's own poll stay open to every member.

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
Only the owner SHALL transfer ownership (to another member, who becomes a manager) or disband the group. After a
transfer, the former owner stays a manager but SHALL hold no owner power, including the creator's right to remove
people. The owner SHALL NOT leave the group before transferring ownership.

#### Scenario: Transfer
- **WHEN** the owner transfers the group to a member, and separately a non-owner tries to transfer it
- **THEN** the first succeeds; the second is denied and nothing is written

#### Scenario: Disband
- **WHEN** the owner disbands the group, and separately a manager tries to
- **THEN** the first succeeds; the second is denied and the group stays

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

#### Scenario: Owner or manager removes a member
- **WHEN** the owner, or a manager, removes a member
- **THEN** the member's Members edge and any Managers edge are withdrawn in one write and they can no longer read the group

#### Scenario: Plain member removes someone
- **WHEN** a member who is not a manager removes another member
- **THEN** it is denied and nothing is written

#### Scenario: Workspace owner removes a member
- **WHEN** a workspace owner who is not in the group removes a member
- **THEN** it succeeds as today, and they still cannot read the group

#### Scenario: Former owner removes a member
- **WHEN** ownership has moved from A to B and A, now a manager, removes a member
- **THEN** it succeeds only because A is a manager; with A also demoted it is denied, since the creator shortcut moved with the transfer

### Requirement: Existing groups keep working when roles arrive
Existing groups SHALL get settings "Mọi thành viên" and an owner: `created_by` if that user is currently assigned to the
group's Members UA, otherwise the earliest currently assigned member. The backfill SHALL write through the policy writer
so decisions are invalidated.

#### Scenario: Creator who left before the backfill
- **WHEN** a group's creator had left before roles arrived
- **THEN** another current member becomes owner and the creator still cannot read the group

### Requirement: Clients learn about role changes at once
A change of roles or settings SHALL publish a realtime event that makes clients refresh their cached permissions for that
group; an action refused because the client's permission was stale SHALL read "Bạn không còn quyền làm việc này".
