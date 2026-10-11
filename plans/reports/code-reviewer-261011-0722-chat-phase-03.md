# Code review: chat phase 03, realtime thread events

Branch `claude/inspiring-cray-32y4n1`, uncommitted on top of e8576c5. Staged `.claude/skills` deletions, CLAUDE.md and `.claude/settings.json` were not reviewed.

## Scope
- Backend: `ws.proto` and `ws.pb.go`, `domain/threads.go`, `store/threads.go`, `grpc/hub_broadcast.go`, `grpc/server.go`, `rest/handler.go`, `rest/threads.go`, plus tests (`grpc/hub_threads_test.go`, `domain/threads_test.go`, `store/threads_test.go`)
- Frontend: `lib/thread-cache.ts` and its test, `stores/websocket.store.ts` and its test, `hooks/keys/messaging.ts`, `api/messaging.ts`, generated `ws.ts`, `typecheck-baseline.txt`
- Specs: `realtime-event-delivery/spec.md`, `chat-threads/spec.md`

## Verification run
- `make test s=messaging`: exit 0. events, rest, store, domain and grpc all pass, with no skips (the DB-backed tests ran).
- `npx vitest run src/lib src/stores`: 322 passed, 0 failed.
- `tsc --noEmit`: 5 errors, all in generated files. That matches the raised baseline exactly (timestamp.ts 2, ws.ts 3). The extra ws.ts error is the same generator `string | undefined` pattern on the new Timestamp field, so raising the baseline is justified.
- `eslint` on the changed frontend files: clean.

## Overall
The hub routing itself is sound. `SendThreadStates` reuses the tenant- and node-scoped `usev:` route, and the hub test proves that another tenant and a non-follower get nothing at the hub layer. The weak point is earlier: deciding **who counts as a follower** and **which tenant to use**. The fan-out also turns a reply's live delivery into all-or-nothing.

## High

### H1. Thread state is pushed to followers who cannot read the channel
`store/threads.go:116` `ThreadFollowers` and `domain/threads.go:123` `ThreadReplyFanout` build the audience only from `thread_follows`, the root's sender and the root's mentions. Nobody re-checks `read` on the channel's content OA.

Two ways a non-reader ends up in that set:
1. **Mentions are not authorized.** `store/store.go:258` creates a follow row for any `users.id` named in a reply's mentions (`WHERE u.id = ANY($1)`), and the implicit rule follows anyone named in the root's mentions. A member of a private channel can mention any colleague in the same tenant, including one who is not a channel member.
2. **Access revocation does not clear follow rows.** Nothing deletes `thread_follows` when someone leaves a channel or loses access. Only the channel subscription is revoked (`RevokeChannelSubscriptions`).

Failure scenario: Alice, in private channel `#legal`, writes a reply that mentions Bob, who is not a member. From then on, every reply in that thread sends Bob's sessions a `thread_state` frame carrying `root_message_id`, `channel_id` and a rising `unread_count`. Bob learns that the channel exists and how active it is.

Before this change, thread unread reached a user only through `GetUnreadCounts`, which counts only channels the caller may read (`reactions_pins_receipts.go:246`). The push path skips that gate. It also breaks the spec's own rule that a summary carries "nothing a channel reader could not already see". `TestThreadEndpoints_DeniedWithoutRead` covers the REST and gRPC endpoints but not the fan-out.

**Fix options:**
- (a) In `ThreadReplyFanout`, drop followers who fail `checkAccess(node, ch.NGACOaID, read)`. This is one in-memory policy check per follower; run it concurrently with a bound.
- (b) Join `ThreadFollowers` against the `channel_members` cache. This is cheaper but the cache is denormalized, so it is a weaker guarantee.

Also add a deny-case test: a mentioned non-reader gets no state. CLAUDE.md §5 requires deny-path coverage for new decision paths.

## Medium

### M1. The follower tenant is the sender's session tenant, not the channel's tenant
`rest/threads.go:33` and `grpc/server.go:112` pass `claims.TenantID` / `caller.TenantID` to `SendThreadStates`. Channel authorization is pure NGAC (`authz.go:46`) and does not check `workspace_id == session tenant`. So a user whose session is in tenant B can reply in a channel of tenant A they are a member of.

- Followers whose sessions are in tenant A (the channel's own tenant) get no state.
- Followers who happen to be in tenant B get state for a tenant-A channel, which is the cross-tenant delivery the spec rules out.

For DMs (`workspace_id` NULL), the other participant gets state only if their current session tenant equals the sender's. Otherwise their unread badge silently goes stale.

The repo already has the right rule in `domain/service.go:49` `channelEvent`: use `ch.WorkspaceID`, and fall back to the caller's tenant only for DMs. The fan-out should return the tenant itself (it already loads the root; add `channels.workspace_id` to `ThreadSummary`), and the transports should not pick one. The spec wording "the tenant of the reply" should then say "the channel's tenant (the sender's for a DM)".

### M2. When the fan-out fails, the reply itself is not delivered live
`rest/threads.go:27-31` and `grpc/server.go:110`: if `ThreadReplyFanout` errors, nothing is broadcast, including the reply message. Before this change a reply always went out as `chat_message`.

The fan-out uses the request context after commit. Echo cancels that context when the client disconnects, which is common on mobile right after the POST is written. When that happens, `ThreadSummary` / `ThreadFollowers` fail with `context canceled`, and every other viewer misses the reply until they refetch.

The gRPC path swallows the error with no log at all.

Fix: on error, still `BroadcastThreadReply` with the bare reply (and no summary) and log. Run the fan-out under `context.WithoutCancel(ctx)` with a timeout, or detach it into a goroutine.

### M3. The fan-out cost is unbounded and runs synchronously in the request path
For each reply, the fan-out does:
- one summary query;
- one followers query, with a correlated `COUNT(*)` per follower over `messages` (indexed on `parent_message_id`, so acceptable);
- one **synchronous Redis `PUBLISH` per follower** (`hub_broadcast.go:279` → `route`), all before `c.JSON(201)` returns.

A thread with 300 followers means 300 sequential Redis round trips added to the sender's send latency. Nothing caps or batches this.

Fix: move the fan-out off the request goroutine (fire-and-forget with its own context), or publish one tenant-scoped frame carrying a map and let each hub filter locally.

### M4. Replies no longer update the conversation list preview or ordering
Previously replies arrived as `chat_message`. The `chatMessage` case (`websocket.store.ts:337`) set `lastMessages[channelId]`, which drives the preview, `lastActivity` and the sort in `lib/conversations.ts:84`, and it also invalidated unread counts.

The `threadReply` case (`websocket.store.ts:415`) does neither. Now a reply does not move the conversation to the top or change its preview. Non-followers' unread counts are unaffected, which is correct because channel unread excludes replies. Followers are covered by `applyThreadState`.

This may be the intended product rule ("topics only"), but the plan and spec do not say so, and no test pins it. The lead should decide and record it in `chat-threads/spec.md`. Otherwise it is a silent behaviour regression.

The `if (msg.parentMessageId) injectReply` branch inside `chatMessage` is now dead for both transports.

### M5. Out-of-order frames can write a stale summary or unread count over a newer one
`ThreadSummary` and `ThreadFollowers` are read after commit, outside the insert transaction. Two concurrent replies R1 and R2 can publish R2's frame (count 2) before R1's (count 1, read earlier). Likewise, a read on device 1 (`unread 0`) can be followed by a reply fan-out computed earlier (`unread 1`). `applyThreadReplySummary` and `applyThreadState` (`thread-cache.ts:21-41`) overwrite blindly.

The result: a wrong count sticks until the next event or refetch. This directly hits the success criterion "the second session returns to 0".

Fix: compare `last_reply_at` before applying a summary. For state, put a server-side `updated_at` or sequence on `ThreadStateEvent` and drop older frames.

## Low

- **L1. REST response shape (question 3).** No caller depends on `{"status":"ok"}`. There is no frontend API for follow/unfollow/read yet, and the REST tests only assert the status code. However:
  - The new body is `pb.ThreadStateEvent` marshalled with `encoding/json`, so its `omitempty` tags drop `following:false` and `unread_count:0`. An unfollow returns `{"root_message_id","channel_id"}` only, and a client reading `res.following` gets `undefined`.
  - REST uses snake_case while the WS frame is camelCase, so the same entity has two shapes.
  - Document the body in `chat-threads/spec.md`, and either use a plain response struct without `omitempty` or protojson with `EmitUnpopulated`.
- **L2. `last_reply_at` has mixed shapes in one cache field.** REST JSON gives `{seconds: number, nanos}`, while the WS patch writes the protobuf-ts `Timestamp` with `seconds: string` (`api/messaging.ts:35` types it as `unknown`). Nothing reads the field yet. Normalize it to an ISO string at the boundary before phase 04 renders it.
- **L3. The plan's storm mitigation is not used.** `thread-cache.ts:29,39,40` call `queryClient.invalidateQueries` directly instead of going through the per-frame `invalidations` batcher (`websocket.store.ts:63`). The plan's Risk section explicitly names that batcher. Every reply in a busy channel invalidates `['threads', channelId]` for every viewer, and every follower also refetches `unreadCounts`. Only active queries refetch, but the plan's signal of >20 refetch/s is reachable. Route these keys through the batcher.
- **L4. `chatMessageOf` duplicates `messageToChatMessage`** (`domain/threads.go:161` vs `grpc/hub_broadcast.go:14`). The field lists are identical today and will drift.
- **L5. Gaps in test coverage.**
  - The hub test hand-builds the states map, so "non-follower gets nothing" only proves the hub delivers what it is given.
  - The domain test does not assert that a non-follower is absent from `fan.States`.
  - No transport test checks that a REST reply emits `thread_reply` (not `chat_message`), or which tenant is used.
  - No test proves that a follower in another tenant, or one without read access, gets nothing (see H1 and M1).
- **L6. Stray file.** An untracked `frontend/tsconfig.tsbuildinfo` sits in the tree. Do not commit it; add it to `.gitignore`.
- **L7. Plan front matter** still says `status: pending`, and the success-criteria boxes are unticked. That is for the lead to update.

## Answers to the five questions
1. **Security.** `users.ngac_node` is the node every session authenticates as. Login, Google sign-in and `SwitchTenant` all mint the token from the same global user node (`auth/domain/tenants.go:160`), not from `tenant_users.ngac_node_id`. So keying by node is correct, and the `tenant:node` route keeps other tenants out. The problems are elsewhere: the audience is not filtered by read access (H1), and the tenant is the sender's rather than the channel's, with DMs depending on both parties being in the same session tenant (M1).
2. **The `chat_message` to `thread_reply` move** changes the conversation preview and ordering (M4). The unread invalidation is preserved for followers through `thread_state` and is not needed for non-followers. The reply still reaches the open thread through `injectReply` in the `threadReply` case.
3. **Response shape.** No dependents. Shape caveats in L1.
4. **Cost and errors.** Unbounded synchronous publish per follower (M3). Failures drop the whole reply, not just the summary, and the gRPC path does not log them (M2).
5. **Acceptance criteria.**
   - Hub and reducer tests: green.
   - Proto regenerated on both sides: yes.
   - "Session 2 returns to 0 within 1 s": the mechanism exists (the REST `threadStateAnswer` pushes the caller's state), but it is not proven end to end and can be overwritten by a stale frame (M5).
   - "Keep tenant and channel scope": not met (H1, M1).

## Recommended actions
1. H1: filter fan-out recipients by read on the channel OA, and add the deny test.
2. M1: derive the tenant from `channels.workspace_id`, falling back to the caller's tenant for DMs, as `channelEvent` does.
3. M2 and M3: always broadcast the reply; move the fan-out off the request context and goroutine.
4. M4: decide whether replies should bump the conversation list, and record the rule in the spec.
5. M5, L1–L3: add ordering guards, fix the REST body shape, and batch the invalidations.

Status: DONE_WITH_CONCERNS
Summary: Tests are green (Go messaging, 322 vitest) and the hub routing is correctly tenant- and node-scoped. However, the follower audience is not re-authorized: mentioned non-members and users whose access was revoked receive `thread_state` for private channels. The tenant also comes from the sender's session rather than the channel.
Concerns/Blockers: H1 should be fixed before commit. M1 and M2 change delivery correctness and should be fixed in this phase. M4 needs a product decision.
