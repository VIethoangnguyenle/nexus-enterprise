# Phase 04a report — shared frontend data foundation

Status: DONE_WITH_CONCERNS (see Open questions). Nothing committed or staged.

## Files
Created: `frontend/src/hooks/keys/{index,admin,approval,assets,auth,contacts,documents,drive,messaging,notifications,permissions,workspaces}.ts`;
tests `stores/websocket.store.test.ts`, `lib/query-client.test.ts`, `api/access.test.ts`, `hooks/usePermissions.test.tsx`, `hooks/useActiveWorkspace.test.tsx`.
Deleted: `stores/permission.store.ts` (working-tree delete only).
Modified: hooks `useAdmin useApproval useAssets useContacts useDocuments useDrive useMessaging useNotifications useWorkspaces usePermissions useActiveWorkspace`;
`stores/{websocket,auth}.store.ts` (+ auth test); `lib/{query-client,workspace}.ts`; `api/access.ts`; `test/router-mock.tsx` (adds `useSearch`);
routes `_workspace.tsx assets.tsx _auth/login.tsx _workspace/{contacts,documents}.tsx _workspace/admin/{index,users,roles}.tsx`;
components `DriveContextPanel DriveFileList DriveFileRow CreateSpaceDialog SpaceView ThreadPanel`; `TopicStream.test.tsx` (key literal).

## Decisions
- Keys: one factory per domain, aggregated as `keys.<domain>` in `hooks/keys/index.ts`. Workspace-scoped data (drive folders/quota/channel drive, channels list, assets lists/types/summary/requests, contacts, documents, admin) takes `wsId`. Drive keys now all sit under `keys.drive.all(ws)`; `channel` moved from `['drive','channel',ws,id]` to `['drive',ws,'channel',id]` for that reason.
- Id-addressed keys (messages/thread/poll/pins/tasks/members by channel or message id, `asset(id)`, `drive.item/shares`) carry no ws: ids are globally unique and WS events do not carry the workspace. Approval keys carry none either (per-tenant schema).
- Bugs fixed by construction: resync now invalidates `keys.messaging.pollsAll()` (`['poll']`, was `['polls']`); drive event with empty `parentId` hits `keys.drive.folder(ws)` = `'root'` entry; asset event invalidates `keys.assets.summaries()` prefix which matches `['asset-summary', ws]`; unread counters are `messaging.unreadCounts()` and `notifications.unreadCount()` (the latter under `notifications.all()`, so one invalidation covers both notification keys).
- `?ws`: `validateWorkspaceSearch` (lib/workspace.ts) on `/_workspace` and `/assets` (assets is a top-level route, not under `_workspace`). Also added `retainSearchParams(['ws'])` on both so in-app links keep the workspace (otherwise the first link click dropped `?ws`). `useActiveWorkspace` reads it with `useSearch({ strict: false })`; fallback to a workspace the user has is unchanged.
- Switcher in AppSidebar stays a full page load: `drive.store.currentFolderId` is not workspace-keyed and would carry a folder across workspaces (04b drive). Query data would refresh correctly now, the Zustand folder would not.
- Resolvers replaced: admin index/users/roles, contacts, assets layout, DriveContextPanel. `CreateChannelModal` and `channels.*` no longer exist / already use the hook.
- Permissions: `NGAC_OPS` (8 ops) in `api/access.ts`, `ObjectPerms = Record<NgacOp, boolean>`, `NO_PERMS`. `usePermissions` = one `useQueries` entry per object under `keys.permissions.object(tenantId, id)` (tenant-scoped like the old cache); queryFn goes through a same-tick batcher so the network still sees one `/drive/batch-access` call. Public return shape unchanged. Old `delete` has no caller; backend drive checks `ngac.OpWrite` for trash/restore/delete (`drive/internal/grpc/server.go:551-586`), and UI already used `perms.write`. `DriveFileList` fallback `{read:true,...,delete:false}` now `READ_ONLY` built from `NO_PERMS`. Logout already calls `queryClient.clear()`; drivePerm event invalidates the object's key; reconnect invalidates `keys.permissions.all()`.
- Mutation errors: `MutationCache.onError` in `lib/query-client.ts` -> `toast.error(explain(err, meta.action ?? 'thực hiện thao tác này'))`; skips `meta.silentError` and 401. `Register.mutationMeta` typed (`silentError`, `action`). `silentError` set on hooks whose callers show their own error: useCreateChannel, useCreateDM, useUpdateChannel, useSendMessage, useSendReply, useCreateTask, useUpdateTask, useAddChannelMember, useRemoveChannelMember, useCreateAssetType, useTransitionAsset, useCreateAssetRequest, useApproveRequest, useRejectRequest, useInviteMember. Drive mutations got `meta.action` for a specific sentence. The ThreadPanel string match no longer exists (already status-based via `explain`).

## Evidence (frontend/)
- `npm test`: 24 files, 164 tests pass (new: 9 websocket-event, 6 mutation-error, 2 access-ops, 5 permissions, 6 active-workspace/validator).
- Red first: websocket test had 5 failures against the old store before the store was migrated.
- `npm run lint` clean; `npm run typecheck:diff` no file above baseline; `npm run build` exit 0.
- `grep -rn "queryKey: \[" src/hooks src/stores` -> nothing. No key array literals remain anywhere in `src` (also checked get/set/cancel/invalidate with `([`).

## Remaining literal keys (04b input)
0 in non-test source. Not done, by scope: imperative fetches (ImagePreviewCard, DriveContextPanel:118, FolderTreeSelect:135, documents.tsx:38, admin/roles.tsx:84, channels), download-URL duplication, 19 untyped `apiFetch`, drive URL folder state, dead store fields.

## Open questions / concerns
1. No backend code publishes `DriveObjectEvent` today (only `ws.pb.go`). I assumed empty `parentId` = workspace root; if the publisher sends the root folder's own id, the root listing still will not refresh. Decide when the publisher is written.
2. The batch request now asks for all 8 ops per object (was 3-4). Cheap on the policy side but a larger payload; narrow `batchCheckAccess(ids, ops)` per call site if wanted.
3. I briefly ran `git rm --cached` on `permission.store.ts` and immediately undid it with `git reset -q HEAD -- <that file>`; index is back to its prior state for that file (deletion is working-tree only). The user's other staged deletions were untouched.
4. `docs/specs/batch-access-check/spec.md` is already modified by someone else; I did not touch docs (outside frontend/). Spec may need a line that the UI requests all 8 ops.
