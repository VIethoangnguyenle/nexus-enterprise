# Phase 04b-drive + 06 drive redesign — report

Status: DONE_WITH_CONCERNS (see Open questions). Nothing committed or staged; the index still holds only the user's own staged deletions.

## What changed

**URL-driven folder.** `/drive?folder=<id>` (+ `ws`, `view=shared`) via `validateDriveSearch` (`lib/drive-search.ts`). Reload, Back/Forward and links keep the folder; breadcrumb ancestors are real links. `drive.store` now holds only `selectedItemId` and the tree's open nodes. Workspace switch drops `folder`/`view` (one-line change in `AppSidebar.handleSwitchWorkspace`); a folder that 404/403s shows "Không mở được thư mục này" with a link to the root.

**Screens** (mockup §2 + §4): list panel = one `TreeView` under the workspace + "Khác › Được chia sẻ với tôi" + quota; hairline table (rows are real `a`/`button`, ↑/↓, ⋯ menu in a Popover); detail panel on `SidePanel` (owner, people with access, activity); one `ShareDialog`; `NameDialog` (new folder + rename, replaces `prompt()`); `MoveItemDialog` (TreeView picker); delete through the one `ConfirmDialog` + "Hoàn tác" toast. Drop a file on the list to upload. Skeleton rows / empty / error / not-found states.

**Data layer.** `hooks/useDownloadUrl.ts` (`useDownloadUrl` query for previews, `useDownloadFile` for saving) replaces 4 copies (drive screen, `FilePreviewCard`, `SpaceFiles`, `ImagePreviewCard`) and the 3 imperative fetches. All drive keys through `hooks/keys`; every drive mutation has `meta.action`. Dead code removed: `ui.store` `activeModal`/`peekPanel*` (no consumers anywhere), `drive.store` location/selection duplicates, `lib/fileIcons.ts` (hex colours), `DriveSidebar/FileList/FileRow/ContextPanel/DeleteConfirmDialog/FolderTreeSelect/DrivePreviewDialog/DriveFilterPills`.

**Motion.** Row stagger (first batch, 30ms, max 8 rows, none under reduced motion) via new `staggerDelay`/`withDelay` in `lib/motion.ts`; row insert/remove via `m.row`; panel/dialog enter+exit via `SidePanel`/`Dialog`; folder change fades content `m.route`; realtime wash via existing `useArrivals` + `rt-wash`/`rt-tag` (person hue, 2.4s); burst line ≥3 changes. No `transition-all`, raw `duration-*`, `animate-*` in drive files (grep empty).

**Backend (drive REST, additive).** `owner_name` was in the proto but never filled. New `rest.WithOwnerNames` decorator (`internal/rest/owner_names.go`) fills it on list/get/shared-with-me from `store.DisplayNames` (users.display_name, falls back to username; matches owner by user id or NGAC node id). Wired in `cmd/main.go` (2 lines). Also fixed `store.GetBreadcrumb`, which ordered the path by id (arbitrary order for depth ≥ 2). No new route, so `vite.config.js` unchanged. I did not touch `handler.go`, any gRPC server, proto or `pkg` (the other agent was editing `handler.go`).

**Specs:** `docs/specs/drive-tree-navigation/spec.md` (folder path in URL, workspace switch, shared view, keyboard, trail) and `drive-context-panel/spec.md` (owner/actor by name, sections instead of 4 tabs, one share dialog, delete + undo).

## Real bugs found and fixed on the way
- `api/drive.ts` sent bodies the REST layer ignores: rename `new_name` (server reads `name`: **a rename blanked the item's name**, reproduced live), move `new_parent_id` (`target_folder_id`), share `target_ngac_id`+`operations` (`target_node_id`+`permission`). Fixed in the client; verified against a freshly built drive service (rename/move/breadcrumb/`owner_name`).
- The old delete dialog said "permanently delete" but only trashed; permanent delete had no UI path. Now: confirm → trash → Undo.
- `ConfirmDialog` was `Modal`-based (no focus trap, `z-[200]`, `bg-black/50`). Re-implemented on `Dialog`, same props (+`cancelLabel`); `Cancel` default now `Huỷ`.

## Decisions where the mockup was silent
1. Owner shown, not "Người sửa gần nhất": the API has no last-editor. Same for Activity: only "X đã tải lên/tạo thư mục · when" (creator + `created_at`); no edit/share events, because they carry no actor (`created_by` is not in `ShareInfo`).
2. Table columns pick by the table's own width (container query `@2xl`), not the window: with the 360px panel open the table is ~530px at 1440, and 5 columns leave 20px for the name. Narrow = owner (+time) under the name; wide = mockup columns.
3. Kept the file-type chips and a name filter (the old screen had them): "Tìm trong thư mục này" filters the open folder only (no search API). Dropped Recent/Starred/Trash/Help/grid-view (no backend, grid never existed).
4. Tree shown ≥1024px; below it, breadcrumb + rows, and two chips (Tất cả tệp / Chia sẻ với tôi) reach the shared view. Detail panel: overlay <1280 and full-screen sheet <768 come from `SidePanel` (not a bottom sheet).
5. Not built: presence cluster "2 người đang xem" and the reconnect banner (phase 05 / shell).
6. `owner_id` is the only realtime author available; edits by someone else on another person's file are washed in the owner's hue. My own rename/move/upload are marked mine for 15s so they are not washed.
7. Revoking a share: the share dialog steps aside while the shared `ConfirmDialog` is up (two stacked `Dialog`s fight over focus-trap/Esc).
8. Shared state components: used `spaces/EmptyState` + `skeleton` (mockup §4). The legacy `components/{Empty,Error,Loading}State` are now unused by drive.

## Evidence (frontend/)
- TDD: tests written first for URL navigation (reload/back/forward/breadcrumb/tree/keyboard/not-found/workspace kept), no-UUID render (text + `aria-label`/`title`/`alt`/`placeholder`/input values, on list, panel, share, move), loading/empty/error, `useDownloadUrl`, TreeView, ConfirmDialog, `drive-search`, `drive-model`, motion helpers, store, realtime wash/own-change/burst (own-change test confirmed red without `markMine`). Backend store tests red first (breadcrumb order seen failing against old query); REST decorator tests written with the implementation.
- `npm run lint` 0; `typecheck:diff` no file above baseline (drive entries removed from `typecheck-baseline.txt`); `npm test` 32 files / 241 tests pass (was 24 / 164); `npm run build` ok; `grep transition-all|duration-[0-9]|animate-bounce` in drive: empty (also no `animate-pulse`/hex/rgba).
- `make build-check` ok; `make test s=drive` exit 0 with zero skips (new: `TestDisplayNames_*`, `TestGetBreadcrumb_OrderedRootFirst`, `TestOwnerNames_*`).

## Screenshots
`plans/261009-0449-project-wide-refactor/reports/drive-screens/` (Playwright from the gstack install against the running Vite on :5173, API mocked with the UUID fixtures; no DB writes; 3 sizes x light/dark):
`drive-{desktop,tablet,mobile}-{light,dark}-{list,detail}.png`, plus desktop-light `-empty -error -loading -menu -share -move -delete -realtime -realtime-reduced`. Compared with mockup §2: hairline table, tree under workspace, panel sections and wash match. Gaps found and fixed: name column collapsed at 1440 with panel open (container query), tree lacked the workspace root level, mobile owner truncated.

## Open questions / concerns
1. **Share "Có thể sửa" may not let the grantee read.** REST sends `Operations: [permission]`, so `write` creates an association with only `write`. Backend should expand `write` to `read+write` (or the UI send both). Not changed: policy semantics, and `handler.go` was being edited elsewhere.
2. `admin/index.tsx` and `admin/roles.tsx` call `ConfirmDialog` with props it never had (`message`, `onCancel`, `variant`, no `open`): their dialogs never show. Pre-existing (in baseline); not touched.
3. The running drive service is the old build, so on :8185 `owner_name` is still absent; the UI falls back to the contacts directory, then "Thành viên". Restart drive to get names for owners outside the workspace.
4. Folder owner is a node id, file owner a user id (and `system` on root folders) — inconsistent at the source; the UI and `DisplayNames` handle both.
5. `documents.tsx` has its own download flow on `documentApi` (different endpoint), left alone. `composites/Breadcrumbs` and `ResponsiveDetailPanel` are still used by approval/contacts; `Breadcrumbs` is no longer used by drive.
6. Test-env: `Not implemented: scrollTo` is stubbed in the drive test; other router-mounted tests may need the same.
7. Plan/phase files not updated (04b-drive, 06 group 3 can be ticked by whoever owns them).

Status: DONE_WITH_CONCERNS
Summary: Drive screens rebuilt to mockup §2 with the folder in the URL, names instead of ids, one share dialog, one tree, shared download hook, owner names from the drive REST, specs updated; all gates green and screenshots saved.
Concerns: write-share semantics (1), admin ConfirmDialog misuse (2), running drive service needs restart for `owner_name` (3).

## Review fixes

All tests written first and seen red (move dialog, hooks, Esc, share rows, foreign folder, tree tab stop, NameDialog, connection banner). Gates: lint 0, typecheck:diff no file above baseline, `npm test` 35 files / 259 tests, build ok. Nothing committed or staged.

1. **MoveItemDialog** (critical): pick and open-nodes now reset whenever the item changes (state adjusted during render, so no render sees a stale pick). Confirm disabled when target is the item itself, or the current parent. Descendants: the tree already omits the item and its whole subtree, and with the reset a stale pick can no longer exist, so there is no separate ancestry walk.
2. **Esc**: `Dialog`, `Popover`, `Modal` now `preventDefault()`. That alone was not enough: DriveScreen's listener was registered first (panel opens before dialog) and `stopPropagation` does not stop sibling listeners on `document`. DriveScreen and `SpaceView` (same pattern, same bug) now listen on `window`, which runs after every `document` handler. Tests: Esc in share dialog closes only the dialog, next Esc closes the panel; Esc in a row menu closes only the menu. Other Esc users checked: `PeoplePicker` (stopImmediatePropagation, fine), `AppSidebar` dropdown and `MentionDropdown` (local), unaffected.
3. **Invalidations**: rename/move/trash/restore also invalidate `drive.sharedWithMe()` and `drive.item(id)` (`invalidateItem` in `useDrive.ts`); trash optimistic removal also patches and rolls back the shared-with-me cache.
4. **Share**: gating stays on `perms.share` (unchanged). Choices are now "Có thể xem" -> `read`, "Có thể sửa" -> `write`; API field names untouched. `sharePermissionLabel` says "Có thể xem" for read (mockup text was "Chỉ xem"; used the wording you specified). Spec line updated.
5. **Foreign folder**: `useDriveItem(folderId)` gives the folder's own `workspace_id` (works for empty folders; breadcrumb entries carry no workspace id), fallback `items[0].workspace_id`. Mismatch = not-found state, listing hidden, permission batch skipped, upload buttons hidden, `uploadFiles` and drop refuse.
6. **Low**: TreeView tab stop falls back to selected, then first root, whichever is actually rendered (effect checks the DOM). NameDialog seeds only on open (ref holds the latest initial name). Deleting the last row: table is held until `AnimatePresence.onExitComplete` (`onRowsGone`), then EmptyState; reduced motion swaps at once. jsdom cannot show the animation; the test covers the end state only, the hold is verified in code.
7. **Mockup**:
   - Search moved to the list panel under the title ("Tìm tệp hoặc thư mục"); below lg (panel hidden) the same field sits above the table. File-type chips and `FILE_FILTERS`/`matchesFileType` removed (and their tests).
   - Detail panel rows: inline dropdown (with chevron) when `perms.share`, plain label otherwise. **No share-update API exists**, so `useChangeSharePermission` = revoke then create; if create fails the previous permission is created again, then the error is raised (not atomic: a crash between the calls would lose the share; a backend update endpoint would fix it).
   - Offline banner built: `useConnectionLost` (websocket dropped and retrying, or `navigator.onLine` false) after 3 s, banner "Đang kết nối lại…". Not built: the "Đã cập nhật" 2 s confirmation on reconnect.
   - Presence "N người đang xem": **backend gap, not built.** The websocket store only knows workspace-wide online users, not who has this folder open; showing that count would be false. Needs a per-folder presence event.
8. **Spec** `docs/specs/drive-context-panel/spec.md`: Share Dialog wording ("Có thể xem" / "Có thể sửa", inline dropdown in the panel) and one new scenario "Change a person's permission". Share/NGAC backend lines untouched.

Screenshots refreshed in `drive-screens/`: `drive-desktop-{light,dark}-{list,detail}.png` and new `drive-desktop-{light,dark}-share.png` (same Playwright + fixtures method; the old light share shot via the menu is now replaced by the one opened from the panel).

Status: DONE_WITH_CONCERNS
Summary: Items 1-8 fixed; presence indicator is a backend gap.
Concerns: permission change is revoke+create (non-atomic); presence needs backend; "Đã cập nhật" reconnect note skipped; `write` still needs the backend expansion to read+write.
