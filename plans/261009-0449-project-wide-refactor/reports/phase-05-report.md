# Phase 05 report: realtime over WebSocket

Date 2026-10-10. Nothing committed, staged or otherwise written to the git index.

## What was built

**Contract** (`backend/pkg/realtime`, new): `Event` (domain, kind, tenant_id, workspace_id, ids, parent/old_parent, actor, channel_id, user_node_ids), `Validate` (tenant and workspace required), `For(ctx,...)` (tenant and actor from the verified caller), `Producer` (franz-go, fire-and-forget, nil-safe, `Connect(service)`), `Recorder` for tests. Topics `<domain>.events`.

**Wire** (`ws.proto`): `SubscribeRequest/UnsubscribeRequest.workspace_id`, `DomainEvent` (seq per workspace), `WorkspaceSubscribed` (ack with last seq), `ApprovalEvent.tenant_id/workspace_id`. `DriveObjectEvent`, `DrivePermEvent`, `AssetUpdatedEvent` removed, fields 7/13/14 reserved. `make proto` and `npm run proto:gen` both run.

**Hub** (messaging): 3-level fan-out (user node, channel, workspace), always inside the event's tenant. Workspace subscribe needs read on the Documents OA and workspace == session tenant (`AuthorizeWorkspaceAccess`). Per-workspace monotonic seq (Redis INCR, local counter without Redis); ack returns the last seq. Presence offline held back 3 s (reload does not flicker). `BroadcastAssetUpdated` (global broadcast, never called) deleted.

**Consumer**: new `RealtimeConsumer` (own group, starts at end, drops events older than 30 s) reads all `<domain>.events` plus `asset.*`, translating asset events into workspace events. Approval keeps its targeted delivery in the old consumer.

**Producers, all after commit, none on refusal or rollback** (each tested with a recorder and a read-at-emit probe):
- drive: created (folder, confirmed upload, copy), updated (rename, restore), moved (old and new parent), deleted (trash, delete), share_created/revoked. A pending upload emits nothing.
- document: created/updated/deleted (list level; no co-editing).
- workspace: invitation_accepted, member_removed, role_changed, department_changed, updated; plus a user-addressed `permission` event for the affected member.
- channel (messaging): created (workspace, or the two participants for a DM), renamed (channel subscribers), member added/removed (roster + the person).
- policy: `permission/changed` to the affected user nodes (max 200 per event) after assignment/association/prohibition create/remove and node delete, after EPP invalidation.
- approval: tenant + workspace on every event. Asset: tenant stamped from the verified caller on all three topics.

**Frontend**: `lib/realtime.ts` (pure `planFor` event to keys through the factories, `observeSeq`, `createBatcher`), store changes (follow workspace, ack-gated resync, seq hole resync, 100 ms batched invalidation, batched presence, `recentChanges`), `useWorkspaceRealtime` in the shell, `useRealtimeWash` (author-hue wash, reused `useArrivals`, `rt-wash`, `rt-tag`) on the documents list and the people table; drive wash now uses the actor when an event named one. Chat messages and reactions still patch the cache.

**Docs**: `docs/specs/realtime-event-delivery/spec.md` (new, indexed), `drive-realtime-sync` updated with a Status section recording the old divergence and its closure.

## Decision: outbox or best effort
Best effort plus resync. Reasons and the remaining window are in the spec ("Delivery guarantee"): every event is a hint to refetch, a lost event is exposed by the next workspace event's seq hole, and every reconnect refreshes everything.

## Bugs found and fixed on the way
- Approval published with the request's context, which is cancelled when the request answers: every approval event failed with "context canceled" (seen live). Now detached with `context.WithoutCancel`.
- A superseded socket's `onclose` reset the new socket's state (tenant switch). Reconnect now uses the current access token.
- Several drive store errors were ignored and would have announced changes that never happened; they now fail the call (Internal).
- `keys.admin.all(ws)` did not prefix the other admin keys; added `keys.admin.everything()` and `keys.assets.activities(ws)`.

## Verification
- `npm run lint`: clean. `typecheck:diff`: no file worse (baseline for generated `ws.ts` raised 1 to 2, the generator's `string[]` indexing). `vitest`: 1103 pass. `npm run build`: ok.
- `make build-check`, `make check-ngac`: ok.
- Go, all touched services plus auth, `-count=1`, zero skips, zero failures, on both `ngac` and `ngac_ci`: policy 226, workspace 475, document 96, messaging 91 (also `-race`), asset 225, drive 128, approval 129, auth 296.
- Hub tests: cross-tenant isolation, unsubscribed gets nothing, subscribe without read denied, seq monotonic per workspace (also across two instances over Redis), user-targeted reaches only that user in that tenant, channel-addressed, invalid event dropped, presence grace.
- **Live two-client E2E** (fresh `make dev` stack, Playwright from gstack, four browser contexts: A actor, B same workspace, C another tenant, D joiner). B's open screen updated without refresh: folder rename 184 ms, new text document 207 ms, new member in contacts 195 ms, approval 201 ms. C received only auth, presence and subscribed frames: zero domain or approval frames. Another-tenant check is by frame type on C's socket.
- **30 s disconnect**: B's app socket was blocked for 30 s while A renamed a folder and created a document. B kept the stale name while offline, then showed the new name 2.8 s after the network returned (backoff), and live updates resumed (181 ms). Evidence, log and scripts: `reports/realtime-e2e/` (8 screenshots, `e2e-log.txt`, `e2e.mjs`, `setup.mjs`).
- Services started by me were stopped; ports checked free. `.dev-pids` removed (stale, from an earlier session).

## Deviations and gaps
- A's actions in the E2E were REST calls with A's token; only the observers are browsers. The screens that B watched were drive, text documents, contacts and approval. The admin people table was not driven live (unit-tested wash, same events as contacts).
- No channel archive exists, so there is no archive event. `member_added` for workspaces does not exist: joining is by invitation, so `invitation_accepted` is the event.
- Document upload events: drive emits for files; the document service's storage gRPC emits nothing (it only handles objects).
- Not covered: notification read-state from another device; reactions/polls already realtime. Asset types as their own entity have no producer; asset lists/summary/types refresh on asset events.
- Channel- and user-addressed events are unnumbered (spec "Known limits").
- Workspace events carry NGAC node ids for members (the domain only has node ids); the people table matches on them.
- Approval fix has no unit test (no Kafka seam in that producer); it was verified live (approval frame arrived after the fix).
- `docker-compose.yml` got `KAFKA_BROKERS` for drive, document, workspace and `Makefile dev` passes it; the Docker path was not run.
- The phase file's first success criterion is marked partial for the reasons above.

## Review fixes

Each fix was written test first. Proto changed once more (`WorkspaceSubscribed.denied`); `make proto` and `npm run proto:gen` both re-run.

**H1 subscriptions outlived their justification.**
- `Hub.RevokeWorkspaceSubscriptions(ws, node)`: local delete plus a Redis `wsrevoke:` fan-out. It drops the workspace stream and every channel subscription the user holds in that tenant (this also fixes the channel-side leak on removal).
- The workspace service is a separate process, so it cannot call the hub. Its `member_removed` event (RemoveMember and LeaveWorkspace both emit it) makes the messaging consumer call the revoke before the event is delivered. That is best effort, so two more layers back it up: a permission event naming the user makes every hub instance re-run the workspace read check for that user's sessions (Redis `permev:` channel) and drop failures, and a session is closed at its token's `exp` (client reconnects with its refreshed token).
- Tests: revoke stops delivery to that user only, not another tenant's session of the same node, across two instances over Redis; permission event drops a demoted user and keeps a colleague; a user who still passes is kept; session closes at exp.

**H2 tab stopped following after a token refresh.** `disconnect()` no longer clears the followed workspace; the follower's release does. Store test: follow, disconnect, connect, auth, subscribe is sent; release still ends it across a reconnect.

**H3 permission events were far too wide.** Audience is now only the node whose standing the write changed: assignment, the child's users (never the parent); association, the users beneath the UA; prohibition, the subject's users; deleted node, the users it held. Object attributes have no audience, so a folder creation or move announces nothing as permission (the drive event covers it, and `moved` now also drops the permission cache on the client). The global `GetNodesByType(UA)` scan and BFS are gone, so nothing expensive remains on the write path. Tests assert exact audiences, with bystander denials (another member of the group, another holder of rights on the same object, workspace members on folder creation).
- Replica freshness question: yes, a refetch could reach policy-read before it applied the mutation, since both are fed from Kafka separately. No graph version exists on the read path, so the sound bounded option chosen is: the consumer holds permission events 750 ms, and the client repeats its refresh once at +2 s. Tested (consumer settle, store retry). A replica later than ~2.8 s is repaired by the next event or reconnect. Documented in the spec.

**H4 producers could block requests.** `pkg/realtime` and approval (and asset, same pattern) now use `TryProduce`, `RecordDeliveryTimeout(10s)`, `MaxBufferedRecords(2000)`; approval's context is `WithTimeout(WithoutCancel(ctx), 10s)` released in the promise; `Close` flushes for at most 3 s. Tests with an unreachable broker: thousands of emits return in well under 2 s.

**M1** The server answers a refused workspace subscribe with `WorkspaceSubscribed{denied:true}` (plus the 403). The client then still resyncs and retries after 5, 15 and 45 s, stopping when granted.
**M2** Share events refresh only `shares(id)` and `item(id)`.
**L1** Asset topics are keyed by workspace id (tested).
**L2** Channel `created` is addressed to the people who can read the channel at birth (creator, DM participants), not the workspace; owners holding Channels-area rights learn at the next refetch. Recorded in the spec.
**L3** The reconnect timer checks the token is still present and is cleared on disconnect (tested).
**L4** `Store.SetTrashed` trashes or restores a folder and its subtree in one transaction; a trigger-injected failure leaves the folder active and emits nothing (tested). The unused children helpers were removed.

**Verification (re-run).**
- Frontend: lint, `typecheck:diff`, vitest (1113), build all exit 0.
- `make build-check` and `make check-ngac` pass.
- Go, zero skips, on `ngac` and `ngac_ci`, matching counts: policy 229, workspace 475, document 96, messaging 100, asset 226, drive 129, approval 130, auth 296; plus `backend/pkg/...`.
- Live E2E re-run on a fresh stack, with both scripts in `reports/realtime-e2e/` (`e2e.mjs`, `e2e2.mjs`): the original scenarios again (rename 181 ms, document 203 ms, member 195 ms, approval 202 ms, other tenant 0 frames, 30 s outage resynced and live updates resumed), plus:
  - (a) B removed while the tab was open: after removal, 0 drive/document/asset frames reached B (a rename and a folder creation were made 1.2 s after the removal). B received only permission frames about itself and the denied answer to its re-subscribe.
  - (b) a 401 forced in B's tab made the app refresh its token and re-authenticate its socket (auth frames 2 to 3); afterwards an approval appeared in 181 ms and a folder rename on the workspace stream in 209 ms.
  - (c) creating a folder gave the bystander B exactly one frame, `drive/created`, and no permission frame; a new member joining gave B only `workspace/invitation_accepted` and no permission frame.
- Everything I started is stopped, ports checked free, `.dev-pids` removed. No git index operations.

**Remaining gaps.** Removal revoke through the consumer is best effort (permission re-check and token expiry back it up). The 750 ms and 2 s settle values are chosen, not measured against replica lag. Removed B received 6 permission frames about itself (one per graph edge the removal deleted); harmless, not coalesced.

Status: DONE_WITH_CONCERNS
Summary: Realtime is implemented end to end (contract, hub, consumer, seven producers, FE reducer/resync/batching/wash, specs) and proven live: another user's changes reach an open screen in about 200 ms, nothing crosses tenants, and a 30 s outage resyncs correctly. All Go suites (both DBs, zero skips) and the frontend gates pass.
Concerns: delivery is best effort (documented); a few event sources are absent (notification read state, channel archive); E2E actor actions were API calls, observers real browsers; the Docker compose path and a live admin-table wash were not exercised; approval context fix is only live-verified.
