# Authz / over-privilege audit (vượt quyền) — 2026-10-17

Scope: read-only review of people/org data and actions on other users across workspace, auth,
messaging (incl. calls, WS hub, presence), approval (session-tenant paths), policy write/PDP
plumbing. No code changed. Method: end-to-end code reading + read-only SQL against the dev DB
(`nexus-enterprise-postgres-1`, db `ngac`) to confirm graph shape. No new tests were written; the
existing messaging authz tests use allow-all / deny-all policy mocks, so they cannot demonstrate
any of the graph-shape findings below.

Severity is by real exploitability: who can do it, with what they already have.

## Summary table

| # | Finding | Severity |
|---|---------|----------|
| F1 | Every workspace member reads/writes every group ("nhóm") in the workspace: messages, files, live feed, calls. Channel membership does not limit access | High (Critical if groups are meant to be private) |
| F2 | Any channel member can bring anyone into the workspace (and reverse a removal) through `AddChannelMember`. No invite right needed, no check that the target belongs to the workspace, and the target can be a whole UA | High |
| F3 | Removed member keeps a tenant-scoped session forever (refresh does not re-check membership) → can still approve requests as a named approver, create requests, read templates, get presence | High |
| F4 | `CreateDM`: anyone can DM, and ring, any user or any UA in the system. `target_user_id` and `target_ngac_node_id` are not checked against each other | Medium |
| F5 | Google `hd` sign-in re-adds a removed member to a domain-claimed org | Medium |
| F6 | Invite-only holders can remove members who hold manage (privilege inversion) | Medium |
| F7 | Contacts / tenant_users can drift from NGAC (best-effort deletes and assigns). Contacts shows every member's email to every member | Medium |
| F8 | Channel `RemoveMember`: any member kicks anyone, DM participants can be removed, and the `channel_members` cache is left stale | Low |
| F9 | Messaging gRPC `SendMessage` trusts `req.MessageType`; a non-"user" type skips the write check | Low (internal-only) |
| F10 | `DeleteFolder` (gRPC) lets a manage holder delete core OAs (Mgmt/Channels/Documents) | Low (internal-only) |
| P1 | `ListDMs` loads every DM in the system per request | Perf (informational) |

---

## F1 — Group membership does not limit access: all workspace members read and write every group (High)

**Where**
- `backend/services/workspace/internal/domain/service.go:210`: `{membersUA, channelsOA, ngac.MemberChannelOps()}` = read, write, create_channel on the workspace Channels OA.
- `backend/services/messaging/internal/domain/channels.go:158`: each channel's Content OA is assigned **under** the Channels OA.
- `backend/services/drive/internal/domain/channel_drive.go:~76`: each channel drive OA is assigned under the channel Content OA.
- PDP `backend/services/policy/internal/ngac/pdp_access.go` (`findMatchingAssociation`): an association on an OA covers all of the OA's descendants (standard NGAC).

**Missing check.** The channel's own Members UA (`ChannelMemberOps`) is meant to be "the audience, not the workspace" (comment at `channels.go:119-121`). But the workspace-level association already grants read+write on every Content OA and every channel drive. `authorizeChannel(ch.NGACOaID, read|write)` therefore ALLOWs any workspace member, in the channel or not.

**Verified on live data.** The Content OA parent of each `general` channel is `<ws>_Channels`, with `<ws>_Members {read,write,create_channel}`. Several channels have workspace members who are not assigned to the channel UA, and those members still satisfy the association. A previous review noted the same thing (TenantMember UA reads every channel).

**Exploit.** Member M of workspace W, not added to group G ("Kế toán", made through "Tạo nhóm" with chosen people):
- `GET /api/workspaces/W/channels` lists G, because `filterAccessible` uses read.
- `GET /api/channels/G/messages`, `/channels/G/search`, `/messages/search`, `/channels/activity` and `/channels/G/pins` all return G's content.
- `POST /api/channels/G/messages` posts into G.
- WS subscribe to G works (`AuthorizeChannelAccess` read), so M follows G live.
- Drive: read/write on G's channel drive files.
- Calls: `POST /api/calls/:callId/token` (write on G) → M joins G's ongoing call silently. `InviteToCall` also treats every workspace member as eligible.
- Channel `RemoveMember` is ineffective: the removed person keeps read through Members UA and can re-subscribe at once (the `RevokeChannelSubscriptions` at `members.go:99` is undone).

**Fix direction.** Decide the model. If groups are private, do not associate Members UA with the Channels OA for read/write. Keep `create_channel` on a separate OA (e.g. a "channel creation" OA, or the Channels OA with the content OAs moved under a sibling container). Workspace-wide "public" channels (like #general) get an explicit Members→Content association. Add PDP test vectors: workspace member not in the channel → DENY on read and write, both for the channel and its drive.

## F2 — `AddChannelMember` puts anyone into the workspace, outsiders included (High)

**Where**
- `backend/services/messaging/internal/domain/members.go:25` `AddMember`. It authorizes only `OpInvite` on the channel Content OA, which every channel member holds through `ChannelMemberOps` (`ngac_ops.go:45`).
- `channels.go:162`: the channel Members UA is assigned **under the workspace PC**.
- `workspace/internal/domain/authz.go:75` `authorizeMember` and `member_directory.go` `reachesPC`: "member" means "the workspace PC is among the caller's ancestors".
- Policy `models.go:112`: a UA→UA assignment is valid.

**Missing checks.** `targetNodeID` is not checked to be (a) a `U` node, nor (b) a member of the channel's workspace. No workspace-level invite right (`OpInvite` on the Mgmt OA) is required.

**Exploit.**
1. *Invite bypass.* Plain member M of W (no invite right) calls `POST /api/channels/G/members {ngac_node_id: X}`, where X is anyone's node ID (their own alt account, or someone from another tenant). X → G's UA → PC_W. X now passes `authorizeMember` for W: `GET /api/workspaces/W` (details, owner names), `/members` (every member, with node IDs), `/departments`, `/roles`, `/permission-areas`, plus `GET /workspaces` lists W. X also gets G's messages, drive and calls.
2. *Removal reversal.* An admin removes X (`detachMember` strips every UA under PC_W). Any remaining channel member re-adds X to a channel, and X is back to all of the above.
3. *Whole-tenant injection.* `ngac_node_id` = the Members UA of another workspace B (its ID is exposed to B members through role summaries and channel/ws payloads). Every member of B now reaches PC_W.

Note: X gets no `tenant_users` row, so contacts and tenant JWT are not granted. But the workspace service ignores the token tenant and asks only the graph.

**Fix direction.** In `AddMember`, require the target to be a `U` node that already reaches the channel's workspace through Members/TenantMember (not merely through some channel UA). Consider putting channel UAs under the Members UA rather than directly under the PC, so that channel membership never implies workspace membership. Tighten `authorizeMember`/`reachesPC` to "under Members UA" instead of "under PC". Same for gRPC `AddChannelMember` (it shares the domain path). Add DENY tests: non-workspace target, UA target.

## F3 — A removed member keeps a tenant-scoped session indefinitely (High)

**Where**
- `backend/services/auth/internal/domain/refresh.go:~200` `RefreshSession` re-mints `GenerateTokenForSession(..., id.TenantID, ...)` from the stored identity with no `tenant_users` check. Refresh TTL is 7 days and rolling (`mint` resets it), so the SPA's auto-refresh keeps the session alive for ever.
- `workspace/internal/domain/members.go:72` `RemoveMember` and `leave.go` revoke no sessions. The workspace service cannot reach the auth refresh store.
- Approval trusts `claims.TenantID`: `approval/internal/rest/handler.go:110` (schema), and `approval/internal/domain/execution.go:59` `CreateApprovalRequest` has no membership check. `template.go:246` `ListTemplates` and `:231` `GetTemplate` check only the org type.
- `approval/internal/domain/execution.go:~176` `actingRow`: for `GrantSource == "direct"` (named approver) it does **not** verify membership.
- Presence: `messaging/internal/grpc/presence.go` and `hub_broadcast.go:381` broadcast to every session whose JWT tenant matches.

**Exploit.** Employee E is a named approver in org W and is removed (or leaves). E's browser keeps refreshing.
- `POST /api/approval/approve {request_id}` still works on any pending request where E has a direct row on the current step. **A removed person approves money/leave requests.**
- `POST /api/approval/requests` files new requests into W's chain. `GET /api/approval/templates` reads W's templates (fields, approver chains).
- WS session keeps receiving W's presence (who is online, display names).
- Other tenant-keyed reads (`/messages/mentions`, `/threads/unread`, `/channels/unread`) are still filtered by NGAC read, so they are mostly empty for E.

**Fix direction.** Re-validate tenant membership on refresh (active `tenant_users` row *and* graph reach). On failure, drop the tenant from the minted token or reject. Have RemoveMember/Leave trigger a per-(user, tenant) session revocation (event to auth). In approval, verify current membership for direct rows in `actingRow` (or check membership in the middleware for every approval call), and gate `CreateApprovalRequest`/templates on NGAC membership, not only the JWT tenant.

## F4 — `CreateDM` has no relationship or type check (Medium)

**Where.** `messaging/internal/rest/handler.go:385` and `messaging/internal/domain/dms.go:57` `FindOrCreateDM`; gRPC `server.go:81` is the same.

**Missing checks.**
- The target is not required to share any workspace with the caller. DMs live under PC_Global, so this is cross-tenant.
- `target_user_id` (used for the DM title, `lookupUsername`) and `target_ngac_node_id` (the node actually added) are never cross-checked.
- The target node is not required to be type `U`. A UA is accepted, and all users under it join the DM.

**Exploit.** Any signed-in user who knows a node ID (former colleague, ex-member, anyone ever seen in a shared workspace or channel member list) can open a DM and push messages to them. `POST /channels/D/calls` then rings them on every device (`callChanged` ring → `DMOthers`). With a UA ID, they can message or ring a whole team or tenant. A mismatched user ID gives a misleading DM title. Members removed from a workspace also keep and can create DMs with its members.

**Fix direction.** Resolve the target node from `target_user_id` server-side (ignore the client node, or require them to match). Require type `U`. Require that the two users share at least one workspace where both are active members, plus the future contacts-visibility rule. Test the DENY cases.

## F5 — Domain auto-join reverses a removal (Medium)

**Where.** `auth/internal/domain/google.go:84` → `tenants.go:24` `joinTenantByDomain`. It treats "no tenant_users row" as "never a member" and calls `joinTenant`. But `RemoveMember`/`Leave` *delete* the row (`workspace/internal/store/directory_store.go:~88` `RemoveTenantUser`) instead of marking it disabled.

**Exploit.** Org W claims `acme.com` (`workspaces.domain`). An admin removes E. On the next Google sign-in with `hd=acme.com`, E is silently re-inserted as an active member and re-assigned to TenantMember UA, so they are fully back.

**Fix direction.** On removal, keep a tombstone (`status='removed'`/`disabled`) instead of deleting. `joinTenantByDomain` already refuses to re-activate an existing row, and contacts/switch already require `active`.

## F6 — `OpInvite` alone can remove admins (Medium)

**Where.** `workspace/internal/domain/members.go:73`: RemoveMember requires `OpInvite` on the Mgmt OA. It escalates to owner+manage only when the target is an Owner. `ngac/areas.go:54` makes `invite` grantable separately from `manage`.

**Exploit.** A custom role "Tuyển dụng" with only `invite` can `DELETE /api/workspaces/W/members/:nodeId` on a member holding a `manage` role (non-owner admin), plus anyone else who is not an owner.

**Fix direction.** Removing a person whose effective grants exceed the caller's (same idea as `guardDelegation`) should be refused, or removal should require `manage`.

## F7 — Contacts and tenant_users drift from NGAC; email shown to everyone (Medium; partly known)

**Where.**
- `auth/internal/domain/users.go:110` `ListContacts` checks `tenant_users.status='active'` only (known).
- `workspace/internal/domain/members.go:103-108`: `RemoveTenantUser` failure is only logged after the NGAC detach.
- `auth/internal/domain/tenants.go:134-139` `joinTenant`: the tenant_users insert succeeds while the NGAC assign is best-effort (`assignUserToTenantNGAC` logs and continues).
- `invitations.go:424-444` `admit` skips `EnsureTenantUser` when the person already reaches the PC (e.g. only through a channel UA from F2), so the two sources diverge the other way.

**Evidence.** The dev DB has 6 active `tenant_users` rows whose user does not reach the workspace PC (seed/test leftovers, but they show drift does happen). Such a row grants contacts, `SwitchTenant` → tenant JWT → approval (F3) without NGAC membership.

**Exposure.** `ListContacts` returns `Email` (and username) of every member to every member, with no visibility scope. Note this for the planned contacts NGAC scope.

**Fix direction.** Gate `ListContacts` (and `SwitchTenant`) on NGAC membership too, or make tenant_users strictly derived from the graph (write graph first, roll back the row on failure, retry the delete). Treat email as scope-controlled in the redesign.

Related known items, assessed:
- `ListDepartments` (`department.go:127`), `ListRoles`/`ListRoleSummaries` (`roles.go:61`, `roles_admin.go:114`), `ListPermissionAreas` and `ListMembers` all use `authorizeMember` (graph reach). On their own they are low (org structure visible to members). Through F2 they become visible to anyone a channel member adds, so fix F2 first. `GetRoleDetail` correctly needs manage.

## F8 — Channel `RemoveMember` weaknesses (Low)

`messaging/internal/domain/members.go:86`:
- Any channel member (they hold `invite` through `ChannelMemberOps`) can remove any other member, including the creator.
- There is no DM guard (`AddMember` has one), so a DM participant can remove the other party.
- `channel_members` is not updated. `FindDMByMembers` (`store.go:133`) keeps returning that DM to the removed party, who then gets 403 on every read: the DM is unusable and no new one is created.

Fix: forbid it on DMs, delete the cache row, and decide who may remove (creator or `manage`).

## F9 — gRPC `SendMessage` honours a caller-supplied `MessageType` (Low)

`messaging/internal/grpc/server.go:104` passes `req.MessageType`. `domain/messages.go:83` checks write only when `msgType == "user"`. Any user-scoped gRPC caller can post a "system" message into any channel without write access. Messaging gRPC is not published (prod compose publishes only LiveKit ports) and REST forces "user", so the risk is limited to internal callers. Fix: ignore the field from gRPC, or allowlist it per calling service (`grpcauth.ServiceFrom`).

## F10 — `DeleteFolder` can delete core OAs (Low)

`workspace/internal/domain/folders.go:81`: `requireInWorkspace(..., TypeOA)` accepts the Mgmt, Channels and Documents OAs and channel content OAs. A non-owner manage holder could break the workspace. It is reachable only through workspace gRPC (no REST route). Fix: restrict it to OAs with the folder name prefix/property.

## P1 — `ListDMs` scans every DM in the system (Perf)

`messaging/internal/domain/dms.go:19` → `store.go:106` `SELECT ... WHERE channel_type='dm'` (no member filter), then one BatchCheck over all of them. This is O(all DMs) per call, and grows with the platform, not with the user. `ListDMsOf` already exists and should be used.

---

## Checked, OK

- **Workspace admin ops**: department CRUD/move/member-dept, role create/delete/set-permissions/assign/unassign, invitations list/revoke/invite-with-extras, details update, convert, folders create. Each uses `authorizeAdmin(OpManage|OpInvite)` on the Mgmt OA resolved under the workspace's own PC.
- **Second-ID scoping**: `departmentInWorkspace`, `requireRole` (custom roles only, not Owners/Members), `nodeInWorkspace`, `requireMemberUser`, `RevokeInvitation(wsID, id)`, and `guardDelegation` for every grant/assignment (role, department, move-under-parent, invite extras, re-checked on accept). No cross-tenant IDOR found in the workspace service.
- **Owner rules**: TransferOwnership, RemoveOwner, RemoveMember-of-owner, Leave and Convert are all done under the owner lock with the last-owner rule.
- **Invitations**: matched to the caller's verified email only. Accept re-checks the inviter's current invite right and drops extras the inviter no longer holds.
- **policyclient**: fail-closed (error or unknown decision = deny). `filterAccessible` lists nothing on a policy error. No fail-open branch found. No `x-caller-*` trust; gRPC identity is an HMAC per-method token, and body fields never set the caller.
- **Messaging per-object checks** look correct (beyond F1): message edit (sender only), delete (sender write, else manage), pin (message must belong to the channel), reactions, threads, polls/tasks, sections (own + placeable), mention pushes (filtered by the recipient's read), `InviteToCall` (target must have write), `FindThreadsByEntity` (filtered per channel), workspace WS subscribe (tenant match + Documents OA read).
- **Auth**: `SwitchTenant`/`GetMe` require an active membership. Avatars are random 128-bit tokens. `ProvisionTenant` is limited to the caller's own tenant.
- **Policy write**: node/assignment type validation, cycle check, EPP invalidation on every write. The PDP shard keyed by the request's workspace cannot widen a decision (unknown node → CTE on the global graph).

## Not covered / partially covered

- Drive, asset and document service internals beyond their workspace scoping: only spot-checked (document routes proxy to drive gRPC with the caller identity; asset handover checks tenant_users).
- Approval visibility paths (`canSeeRequest`, `GetDepartmentRequests` scopes): read but not exhaustively traced.
- Frontend-only "search suggestions" (recent searches, local) have no server endpoint to audit.

## Unresolved questions

1. Product intent for F1: are "nhóm" created through "Tạo nhóm" meant to be private to their members (the code comments and UI imply yes), or public within the workspace like Slack channels? This decides between Critical and "design, but RemoveMember/membership is cosmetic".
2. Should DMs be allowed between people who share no workspace (F4)? This is needed for the contacts-visibility design.
3. Should removal be a tombstone (`status='removed'`) rather than a delete? F5 and F3 both depend on it, and an existing comment talks about "disabled", but no code path ever sets that status.
4. Is `invite` meant to include removing people who hold `manage` (F6)? `ngac/areas.go` says "invite (add and remove members)".
