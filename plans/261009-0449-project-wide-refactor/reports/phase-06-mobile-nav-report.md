# Phase 06 - mobile navigation, workspace switch re-scope, drive folder hard delete

Decision 16 (USER DECISION) implemented design-first. Nothing committed, staged or touched in the git index.

## 1. Design

- `DESIGN.md` section 5: the breakpoint line "tab bar đáy 5 mục" replaced, plus one item "Điều hướng di động" (bar, sheet, behaviour, motion). Nothing else in the file changed. Only existing tokens (`raised`, `accent`, `accent-wash`, `overlay`, `scrim`, `line`, `--z-sticky/backdrop/modal`, section 7 durations/eases).
- Bar: 3 labelled tabs + Thêm, 4 icons total (Thêm = three dots). 48px tabs, 4px padding (56) + `env(safe-area-inset-bottom)`, active = accent label 600 + `accent-wash` 48x24 radius 10 behind the icon, badge only on Phê duyệt (`99+`, hidden at 0), Thêm lit when the screen is Tài sản/Danh bạ/Quản trị/Cài đặt.
- Sheet: handle row + Đóng, workspace+person row (opens switcher), Tài sản, Danh bạ, Quản trị, Cài đặt, divider, Đăng xuất. Esc, scrim, Đóng, swipe-down (>25% or fast), focus trap, return focus. Motion: scrim fade 220, sheet translateY 280 expo, exit 210 ease-in, reduced = fade 120.
- Mockups: `core-screens.html` section h-mobile redrawn (light row: bar on Tin nhắn with a working Thêm demo, Phê duyệt with badge and detail sheet, Thêm open, switcher; dark row: Tài sản with Thêm lit, Thêm open, switcher; 7 annotations; reduced-motion CSS). `admin.html`, `assets.html`, `contacts-documents-settings.html`, `auth.html` bar CSS and every bar instance converted; admin text "vào từ avatar ở góc" fixed. Mockup shots: `mock-mobile-closed.png`, `mock-mobile-demo-open.png`.

## 2. Code

- `components/patterns/MobileNav.tsx` rebuilt (Vietnamese labels, `useApprovalPending().total` for the badge, TanStack `Link`, `Pressable` for Thêm).
- `components/patterns/MobileMoreSheet.tsx` new: portal, `useModalFocus` (initial focus = workspace row, Esc, trap, return focus, inert app), `m.sheet` motion preset, `useDragControls` from existing `motion` dep for swipe-down, z-index tokens only, logout through `logoutSession` (single path).
- `lib/motion.ts` gets a `sheet` preset (reduced = fade 120ms). `index.css` gets `pb-tabbar`, `bottom-tabbar`, `pb-bar`, `pb-sheet`, `max-h-sheet` utilities; `index.html` viewport `viewport-fit=cover` so safe-area insets are real.
- `routes/_workspace.tsx`: `pb-14` -> `pb-tabbar`, mobile list overlay `bottom-14 z-40` -> `bottom-tabbar z-modal`.
- Floating avatar: **no such element exists in the current source** (checked every `fixed`/Avatar use; the sidebar, which has the user avatar, is `hidden` below `lg`). The only trace was the `max-md:pl-16` workaround in `UserPanel`, `DepartmentPanel`, `RolePanel` footers; removed (grep `max-md:pl-16` = 0). Screenshots of admin user/role panels and the approval panel at 360/820 show the footers at the left edge. A runtime scan of `position: fixed` elements shows the nav plus two layout/toast containers, no avatar. The earlier reports likely saw the sidebar's user row bleeding through; I could not reproduce any floating avatar.

## 3. Workspace switch re-scopes the token

- `hooks/useSwitchWorkspace.ts`: `useSwitchWorkspace` (switch-tenant first, nothing changes if refused; then cancel in-flight queries, replace token, `queryClient.clear()`, reset drive store, navigate to the same module root with `?ws=` only, folder/view dropped) and `useWorkspaceSwitcher` (list from `/api/me/workspaces`, current from `useActiveWorkspace`, choosing the open one just closes). Failure = shared toast "Chưa chuyển workspace được...", token and data untouched.
- `AppSidebar` dropdown and the sheet both use it; the old full page reload and the `/workspaces` list (everything the graph reaches) are gone. `useActiveWorkspace` doc updated.

## 4. Drive hard delete of a folder with text documents

- `store.CountTextDocumentsUnder` (recursive), `store.ErrFolderHasDocuments` (also mapped from FK violation 23503 on `text_documents_folder_id_fkey`), `reason.FolderHasDocuments`.
- `DeleteItem` order now: write check (403 first, so a denied caller learns nothing) -> count documents, refuse `FailedPrecondition` -> row delete -> stored objects, quota -> folder OA delete. A refusal anywhere before the row delete leaves OA, files, quota untouched.
- REST: 409 `{error, reason: "folder_has_documents"}`; generic `FailedPrecondition` -> 409.
- UI: `lib/errors.ts` `reasonOf` + `explain` sentence "Chưa xoá thư mục được vì trong thư mục còn văn bản. Chuyển hoặc xoá các văn bản trước, rồi xoá thư mục."; `useDeleteItemPermanently` hook in `useDrive.ts`.
- Specs: `docs/specs/drive-context-panel/spec.md` (new requirement), `docs/specs/lark-sidebar-layout/spec.md` (phone nav + switcher).

## Tests

- Frontend: `MobileNav.test.tsx` (16: 3 tabs + Thêm, badge/99+/0, active states, Thêm lit, no floating avatar, sheet items and order, person row, focus in/Esc/return, tab trap, Đóng, navigation closes, logout once, switcher lists only `/me/workspaces`, switch re-scopes token + clears cache + resets drive store + opens `?ws=`, refusal leaves everything, choosing the open one is a no-op), `AppSidebar.test.tsx` (+1 switch), `useSwitchWorkspace.test.ts`, `motion.test.ts`, `useDrive.test.tsx` (+2).
- Backend: `grpc/delete_folder_test.go` (refused with document, refused with document in a descendant, files-only still deletes with objects removed and OA deleted, succeeds after documents removed, denied caller gets 403 not 409, store reports `ErrFolderHasDocuments`), `rest/delete_item_test.go` (409 + reason, other codes keep status, success).
- Results: `npm run lint` exit 0; `typecheck:diff` no new errors; `npm test` 78 files / 1052 tests pass; `npm run build` ok. `make build-check` ok, `make check-ngac` ok, `make test s=drive` exit 0 with zero skips on `ngac` and on `ngac_ci`.
- Swipe-down is not unit-tested (jsdom has no pointer drag geometry); the close handler is a pure threshold check; verified by the closing paths (Esc/Đóng/scrim) only. Not exercised on a real touch device.

## Screenshots

`reports/mobile-nav-screens/` (46 files): `bar-{chat,drive,approval,assets}`, `sheet`, `switcher`, `sheet-after-switch`, `drive-delete-refusal`, `admin-user-panel`, `admin-role-panel`, `approval-panel`, each at 360 and 820, light and dark, plus the two mockup shots. Playwright (gstack) against Vite on :5173 that I started and stopped; `/api` answered in the page by the existing fixtures through a temporary adapter module (deleted). No DB writes. Drive refusal toast was produced by running the real `driveApi.deleteItem` through the shared mutation cache, because no screen calls permanent delete yet (see concerns).

## Concerns

- No screen reaches permanent delete today (the UI only trashes), so the refusal message is wired and tested at hook and error level but has no button to show it from. The hook is ready for the future "delete forever" action.
- Bar breakpoint: code shows the bar below `lg` (1024) because the sidebar is hidden there; DESIGN.md says rail at 768-1023 and bar below 768. Pre-existing divergence, left alone (820 screenshots show the bar).
- Quản trị always listed in Thêm (the sidebar does the same); DESIGN says "khi có quyền" but there is no cheap permission signal in the shell, the admin frame already shows a forbidden state.
- `MobileNav` fetches the approval pending list on desktop too (hidden by CSS); one cached GET shared with the Phê duyệt screen.
- Toasts still sit at `bottom-4` and can overlap the bar (not in scope).
- Drive delete: a failed OA delete after the row is gone is logged (`slog.Error`) not returned (the caller can no longer retry); it leaves an unreachable OA to clean up. Subfolder OAs of a deleted folder are as before (not touched).
- After a switch the new workspace's data comes from the real services; the shots show skeletons for it only because fixtures cover one workspace.

---

# Follow-ups (coordinator round 2)

Design first: `DESIGN.md` (mobile-navigation item only, plus the breakpoint bullet it sits in) and the h-mobile mockup (7 phones, each now with the top bar; approval draws none because it has no search) were updated before the code.

1. **Top bar replaces "Menu".**
   - `MobileTopBar` (new): "Nexus · <workspace>" on the left, search action on the right, `pt-safe` for the notch, rendered only below 1024. The Menu row, the slide-over list overlay, its state and the `open-mobile-list` event are gone from `routes/_workspace.tsx`.
   - Search: screens mark their own search with `data-module-search` (`SearchField moduleSearch`, used by Tài liệu, Danh bạ, Văn bản, Tài sản, Người dùng; the channel's Tìm button in `SpaceView`). The bar watches the screen (MutationObserver) and shows the action only when one exists: an input is focused, a button is pressed. No action on Trang chủ, Phê duyệt, Cài đặt, Quản trị overview/roles, Tài sản Tổng quan (no search exists there; I did not invent one).
   - Nothing became unreachable. The drawer only ever held the Tin nhắn navigator (`ListPanel` is null for every other module). Its extras are now covered: the conversation list is Trang chủ (the Tin nhắn tab); "Trò chuyện mới" (direct message, Tạo nhóm) was only in the navigator, so it is extracted to `NewChatMenu` and also drawn in Trang chủ's header on phones; the channel back arrow now navigates to `/channels` instead of firing the drawer event.
   - Tests: `MobileTopBar.test.tsx` (6: no Menu, no action without module search, focus, button press, follows the screen, hidden on desktop), `HomeView` (Trò chuyện mới reachable), `SpaceView` (back goes to Trang chủ, search marked).
2. **Toasts** sit at `max-lg:bottom-above-bar` (`calc(4.5rem + env(safe-area-inset-bottom))`, 56 bar + 16) below 1024; `Toast.test.tsx`. Screenshot `toast-above-bar-360-*`.
3. **Pending query gated.** `MobileNav` returns null and passes `enabled: false` unless `useBottomBarLayout()` (new in `usePhone.ts`, `(max-width: 1023.98px)`). I did not use the literal `usePhone` (< 768): the bar shows up to 1023, so a 768 gate would drop the badge at 820. Tests: no bar and no `/approval/pending` request on desktop; request made when narrow.
4. **Breakpoint stated accurately** in DESIGN.md: "dưới 1024 cho tới khi rail 64px của 768–1023 được làm". Follow-up (not built): the 64px rail for 768-1023; when it lands, narrow `useBottomBarLayout` and the `lg:` classes on the bars to 768 and drop the DESIGN note.

Extra: the Trò chuyện mới popover on Trang chủ is right-aligned (`bottom-end`) so it does not run off a 360 screen (caught in the screenshot).

Gates: lint 0, typecheck:diff no new errors, `npm test` 80 files / 1064 tests, build 0, `make build-check` ok. No backend change this round, so the drive suites were not re-run. Screenshots (360, light and dark, in `mobile-nav-screens/`): `topbar-{chat-home,chat-channel,drive,approval,assets}`, `chat-home-new-chat`, `toast-above-bar`, `search-action-focus`, `sheet`, `switcher`; the older 360 bar/sheet shots were replaced, the 820 ones from round 1 are unchanged and predate the top bar. Vite on :5173 started and stopped (pid checked); temporary adapter deleted.

New concerns: a top bar `pt-safe` and `viewport-fit=cover` only matter on notched devices and were not seen on one; Quản trị tabs other than Người dùng, and Cài đặt, have no search so the action is absent there; the other four mockups still draw phone screens without the top bar (only h-mobile was asked for).

Status: DONE_WITH_CONCERNS
Summary: Top bar with "Nexus · workspace" and a context-aware search action replaces the Menu trigger (with Tin nhắn's new-chat menu and back navigation moved so nothing is lost), toasts clear the bar, the approvals count is fetched only when the bar shows, and DESIGN.md states the below-1024 behaviour. All gates pass; 360 screenshots retaken.
Concerns: no search on several screens so no action there; notch insets and the 820 shots not re-verified on device/after this round; 768-1023 rail still to build.
