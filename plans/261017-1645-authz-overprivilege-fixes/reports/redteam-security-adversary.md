# Red-team review: security adversary + fact check

Plan: `plans/261017-1645-authz-overprivilege-fixes/` (plan.md + phase-01..08). Reviewed 2026-10-17.
Method: I opened every symbol and file:line I relied on, plus read-only queries against the dev DB (`ngac`, 5433).
Dev counts I confirmed: 125 Members and 125 Owners associations on `*_Channels`, 15 NULL `ngac_node_id`
rows, 6 orphan `channel_members` rows, and 2 workspaces with no ID-keyed Members UA or PC.

## Finding 1: The refresh "503 without revoke" path still logs the user out, because the token is spent before the check
- **Severity:** High
- **Location:** Phase 4, Requirements item 4 and the Risk row "Policy outage logs everybody out"
- **Flaw:** The membership check runs *after* `Rotate`. `Rotate` marks the presented refresh token spent before it mints the next one. On a policy or DB error the plan returns 503 and withholds `next`, so the cookie still holds the spent token. The client's next refresh is then treated as a replay, and `Rotate` burns the whole session family.
- **Failure scenario:** Policy restarts (every migration deploy restarts it on purpose, `deploy.sh:101-107`), or the DB blips for a few seconds. Every SPA that refreshes in that window gets 503, retries with the same cookie, and hits "refresh token replayed". The session is revoked and the user is forced to sign in again. This is exactly the mass logout the plan says it prevents, and the test "policy unreachable → 503 and session NOT revoked" passes anyway because it never makes a second call.
- **Evidence:** `backend/services/auth/internal/domain/refresh.go:161-166` (a spent token revokes the family), `:168-176` (marked spent before minting), `:201-212` (RefreshSession runs Rotate first). `backend/services/auth/internal/rest/session.go:111-118`.
- **Suggested fix:** Run the membership check before rotating: add `Lookup(token)` (it exists, used by `EndSessionByRefreshToken`), check membership, then `Rotate`. Alternatively, on `ErrUnavailable` still set the new cookie and return a tenant-less access token. Add a test that makes two refresh calls across an outage.

## Finding 2: "0 exposure for live sockets" is false, because a removed user's socket reconnects with the access token it already holds
- **Severity:** High
- **Location:** Phase 4, Requirements item 4 ("0 for ... live sockets") and item 6
- **Flaw:** The fix only closes sockets that are open when the event arrives. `handleAuth` checks nothing but the JWT signature and expiry, then joins presence for `claims.TenantID`. The SPA reconnects at once with the same access token, which stays valid for up to 15 minutes, and is announced to and receives the tenant's presence again. Presence goes to every session whose JWT tenant matches, with no subscription or graph check. Socket close also depends on Kafka delivering `member_removed` within `maxEventAge`. Phase 7 makes that event conditional (Finding 3).
- **Failure scenario:** An admin removes E. Hub item 6 closes E's socket, E's client reconnects 1 s later, and for up to about 15 minutes E keeps seeing who is online in W and broadcasting their own presence. The planned test "removal event closes sockets" passes.
- **Evidence:** `backend/services/messaging/internal/grpc/hub_conn.go:98-137` (no membership check, `userCameOnline`), `hub_broadcast.go:160-175` (`broadcastToTenant` matches only `client.tenantID`), `hub_broadcast.go:381-407`, `messaging/internal/events/realtime_consumer.go:203-216`.
- **Suggested fix:** In `handleAuth`, when `TenantID != ""`, call `policyclient.IsWorkspaceMember` (fail closed) before `userCameOnline`. Otherwise keep a per-(tenant, user) "removed" set in Redis, published by the revoke path and checked on auth. Reword the exposure claim, or add a test that reconnects after removal.

## Finding 3: Phase 7 suppresses `member_removed` when the tombstone write fails, leaving a detached user with live feeds, calls and a tenant sign-in
- **Severity:** High
- **Location:** Phase 7, Requirements, "Writers, graph first", RemoveMember/Leave bullet ("the event still fires only on success")
- **Flaw:** The graph detach has already happened. If `MarkTenantUserRemoved` then fails, the plan returns 500 and skips `announce`. Socket revocation, subscription revocation and call eviction are all driven by that event. Today the event fires even when the listing update fails. The plan turns a cosmetic drift into an access drift: the row stays `active`, and the subscriptions authorized before removal stay open.
- **Failure scenario:** A DB hiccup at the tombstone step. E is detached in the graph, but every channel subscription E opened while a member keeps streaming messages, because subscriptions are checked only when they open. E stays in ongoing calls, since the workspace sweep never runs, and keeps presence. Because the row is still `active`, E's next Google or OTP sign-in mints a W-scoped JWT (Finding 7). Nothing retries automatically.
- **Evidence:** `backend/services/workspace/internal/domain/members.go:100-113` (current order: listing failure logged, event still announced), `leave.go:44-55`, `messaging/internal/events/realtime_consumer.go:207-216` (revocation is triggered only by the event), `realtime_consumer.go:110-132` (the call sweep is event-driven).
- **Suggested fix:** Emit `member_removed` and `announceAccessChanged` as soon as `detachMember` succeeds, whatever the tombstone outcome, then return the tombstone error. Add a test: tombstone fails → event still emitted.

## Finding 4: The tombstone breaks readers the plan never audited; one keeps removed people eligible as approvers, and the store change reactivates `disabled` people
- **Severity:** High
- **Location:** Phase 4, Requirements item 2 and the Risk row "Previous release sees `removed` rows ... all its readers filter `active`"
- **Flaw:** The claim that all readers filter `active` is false. Today's DELETE also served as the filter for several queries that never check status:
  - `CanonicalApprover` accepts any `tenant_users` row. A removed person can still be named `specific_user` approver in new or updated templates. After Phase 4's `actingRow` fix they cannot act, so every request routed to them stalls.
  - `OtherMembers` counts every row. After one removal, the creator can never delete the workspace ("other people belong to this workspace").
  - The planned `EnsureTenantUser ... DO UPDATE SET status='active'` reactivates `disabled` and `invited` rows too. That reverses the invariant documented on the function: "a person an administrator disabled does not come back by being invited".
- **Failure scenario:** (a) An admin builds a leave template naming ex-employee X, whose row is now `removed`. It validates, and every request waits forever on X. (b) A disabled user accepts an old invitation and `admit` silently re-activates them.
- **Evidence:** `backend/services/approval/internal/store/names.go:113-117` (no status filter), `backend/services/workspace/internal/store/teardown.go:30-33` with caller `workspace/internal/domain/workspace_delete.go:76-83`, `backend/services/workspace/internal/store/directory_store.go:70-80` (comment and current ON CONFLICT).
- **Suggested fix:** Inventory every `tenant_users` reader (`grep -rn tenant_users backend/services`, about 30 hits) in Phase 4 and add `status='active'` where membership is meant. Reactivate only `WHERE tenant_users.status = 'removed'`. Add deny tests for CanonicalApprover with a removed user and for delete-workspace after a removal.

## Finding 5: No phase repairs the legacy workspaces (decision 4 / Q6); migrations 058 and 060 then wipe channel membership and tombstone every member there
- **Severity:** High
- **Location:** plan.md "Quyết định ... 4" vs Phase 3 Requirements (058) and Phase 7 Requirements (060 step 2)
- **Flaw:** Zane decided that workspaces missing ID-keyed nodes are repaired in a migration, not left to deny. No phase file contains that repair, and no ordering puts it before 057/058. Phase 3's 058 deletes every `U → Ch_*_Members` edge whose user does not reach `<W>_Members`. In a workspace with no `<W>_Members` node, that is every member. Phase 7's 060 step 2 tombstones every active row there. `TenantMember` is attached to Members only when the Members UA exists (`if a.parent == "" continue`), so these tenants may have users who reach only the PC.
- **Failure scenario:** Prod has a pre-023 workspace. After 058 every channel membership in it is gone, and after 060 everyone's row is `removed`. They get refresh 401, approval 403 and empty contacts. 058 is documented as "not auto-restorable".
- **Evidence:** Dev DB: `test-ws-561b804a5443` and `test-ws-c56ed191dc29` have 0 `<id>_Members` and 0 `PC_<id>` nodes, but have 1 active row and 1 channel. `backend/services/auth/internal/domain/provisioning.go:176-191`. Phase 3 rollback admits 058 deletions are not restorable.
- **Suggested fix:** Add an explicit repair migration (e.g. `056a`) or a Phase 01 step that creates or renames `<ws>_Members`/`PC_<ws>` and re-links TenantMember. Make 057/058/060 abort (`RAISE EXCEPTION`) if any workspace still lacks an ID-keyed Members UA, the same way `025_node_and_drive_root_uniqueness.sql:32-43` does.

## Finding 6: The plan forgets the `policy-read` replica; after the SQL migrations, private-group drives stay open on the stale replica
- **Severity:** High
- **Location:** plan.md "Cross-cutting rules" (restart policy and messaging; startup L1 flush), Phase 2 Requirements (flush in `policy/cmd/main.go` only) and steps 4g/6, Phase 3 step 4
- **Flaw:** 057/058/060 write `ngac_*` with SQL, which emits no graph-mutation event. `policy-read` learns of changes only from those events, and drive and workspace route reads to it. The plan's restart list ("policy and messaging", `make redeploy s=policy`/`s=messaging`) never restarts `policy-read`, and the L1 flush is added only to the writer. In prod, `deploy.sh` recreates `policy` and `policy-read` together. While the old `policy-read` is still up it can write stale ALLOWs back into the Redis L1 cache it shares with the writer, after the writer's flush, for up to `cacheTTL` = 30 s.
- **Failure scenario:** On the docker dev stack (`policy-read` present), after 057 plus the prescribed restart, messaging returns 403 to a non-member, so step 6's spot check passes. Drive still answers from the old graph: the non-member keeps read/write/upload on `Ch_G_Drive` until someone happens to restart `policy-read`.
- **Evidence:** `docker-compose.yml:178,420,458` (`policy-read`, `POLICY_READ_SERVICE_ADDR`), `backend/services/drive/cmd/main.go:41,61-62`, `backend/services/workspace/cmd/main.go:45`, `backend/services/policy/cmd/policy-read/main.go:56,64-77` (event-driven sync, its own Redis L1), `deploy/deploy.sh:101-107`.
- **Suggested fix:** Restart `policy` **and** `policy-read` in every migration step and header note. Put the `InvalidateAll` in both binaries, or flush once after both are healthy. Add a verification step against drive as a non-member (a 403 on a channel-drive file read), not only messaging.

## Finding 7: The Phase 7 read gate misses token minting at sign-in, so a drifted `active` row still yields a tenant JWT
- **Severity:** Medium
- **Location:** Phase 7, Requirements "Readers" (only ListContacts, SwitchTenant, GetMe, plus Refresh from Phase 4)
- **Flaw:** Google and OTP sign-in pick the default tenant from `ListTenantsByUser`, which reads the row only. The resulting token goes to every service that trusts `claims.TenantID` (hub presence, approval up to the middleware, asset by row). `ListWorkspaceSummaries`, which feeds the workspace switcher, is also row-only. Drift is still produced after this plan, by Finding 3 and by the lenient sign-in path.
- **Failure scenario:** The graph is detached but the row is still `active` (Finding 3, or a prod row 060 did not catch). E signs in fresh, gets a W-scoped JWT, sees W in the switcher, and joins W's presence. Refresh only catches it 15 minutes later.
- **Evidence:** `backend/services/auth/internal/domain/google.go:198-214`, `otp.go:245-251` and `:306-308`, `auth/internal/store/store.go:160-166`, `auth/internal/store/directory.go:72-82`.
- **Suggested fix:** Filter `ListTenantsByUser` results through `IsWorkspaceMember` in `signinResultFor`/`otpResultFromExistingUser`. On error, fall back to a tenant-less token, not a 503 at sign-in. Apply the same gate to the workspace-summaries listing. Add deny tests.

## Finding 8: The DM relationship gate covers only SendMessage and StartCall; every other DM write path stays open to ex-colleagues and cross-tenant pairs
- **Severity:** Medium
- **Location:** Phase 5, Requirements (StartCall, SendMessage) and Architecture
- **Flaw:** Decision 2 is to block new messages and calls. Several DM write paths authorize `OpWrite` themselves and never reach the gated SendMessage branch:
  - polls write their announcement message straight to the store as `system`
  - tasks
  - reactions
  - pins
  - EditMessage, which rewrites old content and is delivered as new text
  - CallToken, which joins an ongoing DM call
  - call eviction (`mayStayInCall`), which is write-based

  If the gate sits inside the existing `msgType == "user"` block, every in-process system line also bypasses it.
- **Failure scenario:** A removed employee, or an outsider who had DM'd someone before this lands, keeps pushing content into the DM: a poll with a harassing question, a task, an edited old message, or joining the call the other person starts. Every Phase 5 test passes.
- **Evidence:** `backend/services/messaging/internal/domain/polls_tasks.go:105,126-136` (poll: write check, then a direct `system` insert), `:265` (task), `reactions_pins_receipts.go:22,33,71,91`, `lifecycle.go:43-48` (EditMessage), `calls.go:243` (CallToken), `messages.go` SendMessage write check only for `msgType=="user"`.
- **Suggested fix:** Enforce it at one choke point: in `authorizeChannel`/`authorizeMessage`, when `ch.ChannelType=="dm"` and op ∈ {write, invite}, also require `SharedWorkspaces ≠ ∅`. Apply the same rule in `mayStayInCall`. Test polls, edit and CallToken on the deny side.

## Finding 9: The F6 guard checks only `manage` on Mgmt, so an invite-only role can still remove people who hold more power than it does
- **Severity:** Medium
- **Location:** Phase 6, Requirements F6
- **Flaw:** The plan escalates only when the target holds `manage` on `<W>_Mgmt`. Removal strips the target of everything under the PC. An invite-only HR role can still remove a holder of `manage` on Assets, on Documents/`share`, or (after Phase 2) on Channels. A Channels `manage` holder is exactly the moderator who, under Phases 2 and 8, can remove people from every private group. The audit's direction was "effective grants exceed the caller's (same idea as `guardDelegation`)". The plan narrowed it without saying so.
- **Failure scenario:** The "Tuyển dụng" role (invite only) removes the asset manager or the channels moderator. That person loses all access until re-invited, and the privilege inversion persists for every non-Mgmt area.
- **Evidence:** `backend/ngac/areas.go:52-60` (AreaOps: manage is grantable on Channels and Assets separately from Mgmt), `backend/services/workspace/internal/domain/members.go:72-96`.
- **Suggested fix:** Require that the caller holds, on each area OA (Mgmt, Documents, Channels, Assets), every operation the target holds there (reuse the `guardDelegation` comparison). Or simply require `manage` on Mgmt whenever the target holds `manage` on any area. Add deny vectors for Assets and Channels managers.

## Finding 10: Phase 2 leaves side doors into private groups: blind `manage` powers, old drive shares, and a caller-chosen `workspace` type over gRPC
- **Severity:** Medium
- **Location:** Phase 2, Requirements (Owners/custom roles keep `manage`; gRPC CreateChannel "keeps `workspace`") and step 5 pre-flight
- **Flaw:**
  1. `manage` on `<W>_Channels` is inherited by every `Ch_G_Content`. Owners and *any custom role* with Channels manage can then, in groups they cannot read: delete any message (which also posts a system line naming the author), rename the group, change its avatar, close polls, reassign tasks, and (Phase 8) remove members. Decision 1 accepted "manage" for owners, but custom roles silently get the same powers.
  2. Drive shares wrap the item in a Share OA under PC_Global, which bypasses the Channels hierarchy entirely. Shares of channel-drive files to `<W>_Members`, role UAs or "public", made while groups were de facto public, survive 057. The pre-flight does not list them.
  3. gRPC `CreateChannel` honours `channel_type: workspace` from any caller identity. Only auth's provisioning legitimately sends it. This is the same caller-trust pattern as F9.
- **Failure scenario:** A custom "Moderator" role deletes messages and evicts members in a private HR group it cannot see. A file from G that an owner shared to "workspace" before the change stays readable by everyone after G becomes private.
- **Evidence:** `backend/services/messaging/internal/domain/lifecycle.go:117-160`, `channels.go:255`, `channel_avatar.go:88`, `polls_tasks.go:216,369`; `backend/services/drive/internal/domain/sharing.go:77-100` (Share OA under PC_Global plus association); `backend/services/messaging/internal/grpc/server.go:35-48` and `backend/services/auth/internal/domain/provisioning.go:72-74` (the only caller).
- **Suggested fix:** (1) Keep `AreaOps(AreaChannels)` at `{create_channel}`, or confirm with Zane that custom roles get blind moderation. (2) Add to the 057 pre-flight: shares whose item is a `drive_context='channel'` file of a now-private group and whose target is not that group's UA; report them per workspace for a decision. (3) Accept `workspace` from gRPC only when `grpcauth.ServiceFrom(ctx) == "auth"`.

Status: DONE_WITH_CONCERNS
Summary: The plan's core graph fixes are sound, but Phase 4's cutoff claims don't hold: refresh still logs users out during an outage, a removed user's socket reconnects, and the tombstone breaks readers nobody checked. Phase 7 suppresses the removal event, legacy workspaces have no repair, and `policy-read` is missing from every restart and cache-flush step.

## Unresolved questions
1. Does prod have any pre-023 workspaces (no `<id>_Members`)? This decides whether Finding 5 blocks deploy.
2. Should custom roles with Channels `manage` really moderate groups they cannot read (Finding 10.1)?
3. Is `disabled` used in prod data, and should an invitation ever reactivate it (Finding 4)?
