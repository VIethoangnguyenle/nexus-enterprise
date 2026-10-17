# Red-team: scope & contracts — 261017-1645-authz-overprivilege-fixes

Reviewer lens: scope/complexity critic + contract verifier. Date 2026-10-17.
Baseline note: local `main` = 97e2317; `origin/main` = 43a1509 is 317 files ahead (calendar service, migrations 057/058).
The plan was written against local `main`.

## Finding 1: Migration numbers 057 and 058 already exist on origin/main
- **Severity:** Critical
- **Location:** plan.md "File ownership" (02: `057_private_groups.sql`, 03: `058_channel_membership_reconcile.sql`, 04: `059_…`, 07: `060_…`); Phase 02/03/04/07 "Related Code Files"
- **Flaw:** The plan assumes the chain ends at 056. `origin/main` already ships `057_calendar.sql` and `058_calendar_occurrence_answers.sql`. The plan says to cut `fix/authz-overprivilege` from main, so it lands on top of those files. The contacts plan's `061_directory_visibility.sql` rests on the same wrong baseline.
- **Failure scenario:** The branch adds a second `057_*.sql` and a second `058_*.sql`. CI's `ls data/migrations/*.sql | sort` runs both 057s in name order. In prod, `deploy/migrate.sh` may already count 057/058 as applied and skip the new files. Either way the private-groups and reconcile SQL may never run while the code assumes they did. In the worst case the code ships, groups still leak, and the unique index is missing.
- **Evidence:** `git ls-tree origin/main data/migrations/` → `057_calendar.sql`, `058_calendar_occurrence_answers.sql`. Local `data/migrations/` ends at `056_channel_avatars.sql`. `plans/261017-1646-contacts-lark-redesign/plan.md:129` reads "057–060 thuộc plan bảo mật".
- **Suggested fix:** Rebase the plan on `origin/main` and renumber to 059–062. Update the contacts plan to 063. Add one line to plan.md: "re-check `ls data/migrations | tail -1` on origin/main at branch cut".

## Finding 2: The calendar service (9th service on origin/main) carries the same F1/F2 bug and is absent from the plan
- **Severity:** Critical
- **Location:** Phase 01 "Membership helper" (consumers listed: messaging, workspace, auth, approval); Phase 02 "Spec"; Phase 03 "Requirements"; Phase 05 "Calls"
- **Flaw:** The plan's job is to make "workspace member" mean "reaches `<W>_Members`" everywhere. Calendar still uses the old "PC among ancestors" test. It also assigns event-guest UAs directly under the PC. Channel calendars sit under `Ch_*_Content`, so the Phase 02 graph change alters their access. Even so, `docs/specs/calendar` and `pdp_calendar_scenarios_test.go` are not named. That violates CLAUDE.md §5: a policy-model change must update the matching spec and test vectors. Event calls (`calls_event.go`) are a call entry point that Phase 05 does not mention.
- **Failure scenario:** After all 8 phases, someone added to one group (or one event's guests UA) still reaches `PC_W`. Calendar `inWorkspace` returns true, and attendees.go:119 then lets them be added as a workspace attendee. The F2 class "becomes a workspace member through a side UA" survives in calendar. Phase 03's reconcile query ("users reaching PC but not Members = 0") also never reaches 0, because the event-guest UA path keeps adding such users.
- **Evidence:** `origin/main:backend/services/calendar/internal/domain/membership.go:50-62` (`n.GetId() == ws.PCNodeID`); `origin/main:…/calendar/internal/domain/attendees.go:119,153` (`prov.Assign(ua, ws.PCNodeID)`); `origin/main:…/calendar/internal/domain/calendars.go:124-128` (channel calendar under the content OA); `origin/main:backend/services/policy/internal/ngac/pdp_calendar_scenarios_test.go:21,113-120`; `origin/main:backend/services/messaging/internal/domain/calls_event.go` (new file).
- **Suggested fix:** Phase 03 also switches `calendar/internal/domain/membership.go` to `policyclient.IsWorkspaceMember`. Phase 02 names `docs/specs/calendar/spec.md` and adds a deny vector: a workspace member not in G is DENIED on G's calendar. Phase 03's success query must exempt `CalEv_*_Guests` UAs or count them explicitly. Phase 05 states whether event calls need a guard.

## Finding 3: Refresh membership check runs after `Rotate`, so the "503, don't revoke" path still ends the session
- **Severity:** Critical
- **Location:** Phase 04 "Requirements" item 4 ("after `Rotate` … Policy/DB error → `ErrUnavailable` 503 without revoking")
- **Flaw:** `Rotate` marks the presented token spent before it mints the next one. On any error the handler returns without setting the new cookie. After a 503 the browser still holds a spent token. Its next refresh is a replay, and `Rotate` then burns the whole session family. The risk table's mitigation, "Policy outage logs everybody out → 503 path without revoke; test", does not hold.
- **Failure scenario:** Policy is briefly unreachable (a deploy restart). Every client that refreshes in that window gets 503. The frontend keeps the session for 5xx and retries. The retry presents the spent token, hits "refresh token replayed", and the user is logged out. This is the mass-logout the plan says it prevents.
- **Evidence:** `backend/services/auth/internal/domain/refresh.go:162-180` (spent before mint; replay revokes the family); `refresh.go:201-212` (RefreshSession); `backend/services/auth/internal/rest/session.go:111-121` (cookie set only on success); `frontend/src/api/client.ts:43-47` (5xx keeps the session and retries later).
- **Suggested fix:** Run `requireActiveMember` before `Rotate`. `s.refresh.Lookup` already exists (`refresh.go:224`) and returns the identity without spending the token. Only "not a member" revokes. The 503 path leaves the token unspent. Add a test: 503, then a second refresh with the same cookie succeeds.

## Finding 4: Closing the removed member's sockets does not cut them off, because WS auth checks only the JWT
- **Severity:** High
- **Location:** Phase 04 "Requirements" item 4 ("0 for … live sockets (item 6)") and item 6 ("Hub: … closes every client")
- **Flaw:** The socket is closed, but the frontend reconnects with backoff. `handleAuth` accepts any unexpired HS256 token carrying `tenant_id`. The removed member's access token is valid for up to 15 more minutes. They reconnect at once and are placed back in tenant presence broadcast (presence fans out by JWT tenant). The plan's "exposure 0 for live sockets" is wrong; actual exposure equals the access TTL, the same as REST.
- **Failure scenario:** An admin removes X. X's tab socket closes, reconnects within the backoff window, re-auths with the same token, and keeps receiving presence for tenant W until the token expires. The hub test in Phase 04 step 1 passes because it only asserts the close.
- **Evidence:** `backend/services/messaging/internal/grpc/hub_conn.go:98-110` (JWT parse only, no membership check); `backend/services/messaging/internal/grpc/hub_domain.go:237-252`; `frontend/src/stores/websocket.store.ts:50,120` (reconnect backoff/timer). The plan cites `hub_broadcast.go:380` for presence by tenant.
- **Suggested fix:** Either (a) state honestly that socket exposure ≤ access TTL and drop the "0" claim, or (b) add an `IsWorkspaceMember` check in `handleAuth` when `TenantID != ""`, plus a reconnect-after-removal test. Option (b) adds a policy call per connect. That is a design choice to put to Zane, not to assume.

## Finding 5: Phase 07 builds two fixes for F7 where the audit asked for one, and adds a GetMe gate nobody requested
- **Severity:** High
- **Location:** Phase 07 "Requirements" (Readers gate + Writers graph-first + migration 060); "Readers: … `GetMe` (`tenants.go:173`) same check"
- **Flaw:** The audit's fix direction is "gate ListContacts/SwitchTenant on NGAC **or** make tenant_users strictly derived". Phase 07 does both: it adds a per-request policy call to every reader, and it also reorders writers graph-first and runs a reconcile migration. It then adds a third reader, GetMe, which the audit lists under "Checked, OK" (audit §Checked: "`SwitchTenant`/`GetMe` require an active membership"). No user decision covers F7's mechanism. Today GetMe degrades softly (a non-active row gives `tInfo=nil`, no error). With the gate, a policy outage returns 503.
- **Failure scenario:** GetMe runs on every app boot. With policy down for a deploy, the SPA cannot load the current user at all, where today it would load. The availability coupling grows (contacts, switch, me, refresh, approval, DM send all now hit policy) while the drift problem is already solved by the graph-first writers plus 060.
- **Evidence:** `plans/reports/security-261017-1630-authz-overprivilege-audit.md:133` ("…**or** make tenant_users strictly derived…") and `:166` (GetMe OK); `backend/services/auth/internal/domain/tenants.go:184-201` (GetMe soft path).
- **Suggested fix:** Pick one mechanism. Graph-first writers plus 060 plus the existing Phase 04 refresh gate cover the risk. Drop the GetMe gate. Keep the ListContacts/SwitchTenant gate only if Zane wants defence in depth, and if so return the soft result rather than 503 on a policy error.

## Finding 6: `ngac/topology.go` is a new abstraction with one production consumer, and it does not reach existing workspaces
- **Severity:** Medium
- **Location:** Phase 01 "Requirements" (`WorkspaceAssociations()`/`WorkspaceAssignments()`, `Role ∈ {Owners, Members, Mgmt, Docs, Channels}`), Phase 01 "Modify" (refactor `service.go:183-211`)
- **Flaw:** It adds a role-enum mini-DSL and refactors `CreateWorkspace`'s provisioning loop. The stated goal is "a grant change is felt in tests". The test already pulls ops from the vocabulary package for one row (`vocab.MemberDocumentOps()`). Swapping the two remaining literals for `ngac.AllOwnerOps()`/`ngac.MemberChannelOps()` (plus the new `OwnerChannelOps()`) achieves the same thing with zero new types. The topology data also never touches existing workspaces; migration 057 rewrites those by name-pattern SQL. The real source of truth for deployed grants stays the SQL, not `topology.go`.
- **Failure scenario:** Four hours of Phase 01 go into a refactor of workspace bootstrap, the most sensitive provisioning path. The risk table itself rates it L×H ("Topology refactor changes grants"). The authz fix gains nothing from it.
- **Evidence:** `backend/services/workspace/internal/domain/service.go:183-211` (two literal tables, 7 assignments + 5 associations); `backend/services/policy/internal/ngac/pdp_workspace_topology_test.go:47-58` (already mixes `vocab.MemberDocumentOps()` with literals).
- **Suggested fix:** Delete `topology.go` from scope. In Phase 02, change the test's literal op lists to the `ngac.*Ops()` helpers and leave `CreateWorkspace` as is, apart from the one-line `OwnerChannelOps()` swap.

## Finding 7: The policy startup cache flush duplicates an existing RPC and fights the Phase 01 harness
- **Severity:** Medium
- **Location:** Phase 02 "Requirements" last bullet ("Policy startup flushes the Redis L1 decision cache"), plan.md "Cross-cutting rules"; Phase 01 harness ("no Redis/Kafka (policy only warns)")
- **Flaw 1 (gold plating):** Explicit invalidation already exists as `WriteServer.InvalidateCache` / `invalidation.InvalidateAll`, and L1 TTL is 30 s. A new startup side effect adds behaviour to the policy service that the audit never asked for.
- **Flaw 2 (contract):** The harness claims "no Redis". Policy defaults `REDIS_URL` to `redis://localhost:6379/0`, and CI runs Redis on 6379. Every test package that starts a policy process will therefore connect to the shared Redis. With Phase 02, each start flushes the decision cache.
- **Failure scenario:** On a dev machine, running `make test s=messaging` flushes the running dev stack's decision cache on every package start. In CI, concurrent policy processes from different packages share and flush one cache, which makes test results order-dependent.
- **Evidence:** `backend/services/policy/cmd/main.go:39` (`REDIS_URL` default); `.github/workflows/ci.yml:40-41` (Redis on localhost:6379); `backend/services/policy/internal/grpc/write_server.go:257,324` (existing InvalidateAll / InvalidateCache RPC); `backend/services/policy/internal/ngac/pdp_decision_cache.go:14` (TTL 30 s).
- **Suggested fix:** Drop the startup flush. After migrating, run `InvalidateCache` once (or wait 30 s) in the deploy runbook step. The harness must pass `REDIS_URL` pointing at an unreachable address and `KAFKA_BROKERS=""` explicitly, and Phase 01 should test that it does.

## Finding 8: Harness fixtures contract is unstated; existing SQL fixtures are invisible to an already-started policy process
- **Severity:** Medium
- **Location:** Phase 01 "Requirements" (harness), every later phase's `*_integration_test.go`
- **Flaw:** The policy process loads the graph once at start. The repo's standard fixture `testutil.CreateWorkspace` (and `CreateUser`) writes `ngac_nodes` with raw SQL. If a test creates fixtures that way after `StartPolicy` (one process per package via `TestMain`, as the plan says), those nodes are absent from the in-memory graph. The plan's own "real policy proves graph shape" claim then breaks. Tests can also pass for the wrong reason: a DENY because the node is unknown, not because the grant is missing. That makes them phantom deny tests. Separately, `make test s=…` runs with `-timeout 60s`, and the harness does a `go build` of the policy module inside that window.
- **Failure scenario:** Phase 03 test "target removed from W → DENY" passes because policy never saw the user node, not because detach worked. The same test against a real flow would ALLOW.
- **Evidence:** `backend/testutil/testutil.go:97-102` (`INSERT INTO ngac_nodes` via SQL); `backend/services/policy/cmd/main.go:57` (single `LoadGraph` at start); `Makefile:426` (`-timeout 60s`).
- **Suggested fix:** Phase 01 states the rule: "integration tests create every graph node/edge through the harness's write client; SQL fixtures only for business rows." Add a helper for that. Every deny test pairs with an allow test on the same fixture, so an unknown node cannot fake a deny. Build the policy binary in `TestMain` before any timer-sensitive test, or document `-timeout` headroom.

## Questions (requested scope, not cut recommendations)
- Q-A (decision "owners don't read private groups"): owners keep `manage` on groups they can no longer list (`ListChannels` filters on read). Is there a UI path for an owner to rename or delete a group they cannot see, or is `manage` effectively only the Phase 08 "remove member" power? Evidence: `backend/services/messaging/internal/store/store.go:97`, Phase 02 Q1.
- Q-B (decision "DM only within shared workspace"): Phase 05 adds 2× `GetAncestors` plus N× `FindNodeByName` to every DM message send. Confirm the per-send gate, rather than gating only DM create and call start, is what Zane wants for existing DMs.

Status: DONE_WITH_CONCERNS
Summary: Three contract breakers block execution as written. The plan targets a stale main: migrations 057/058 already exist on origin/main, and the calendar service keeps the F2 "reaches PC" membership bug. Phase 04's refresh check after Rotate turns the "don't log out on policy outage" path into a session revocation. Scope creep sits in Phase 01's topology DSL, Phase 02's startup flush, and Phase 07's double mechanism plus GetMe gate.
