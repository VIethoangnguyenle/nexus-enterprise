# App rail (menu chính thu gọn / mở rộng) — implementation report

Plan: `plans/261011-1100-app-rail/plan.md`. Status: DONE_WITH_CONCERNS (NEEDS_CONTEXT items below). No commits, git index untouched, build not run (Claude runs it).

## Files (all under `/home/zane/Desktop/projects/nexus-enterprise`)

New
- `frontend/src/hooks/useSidebarCollapsed.ts` + `.test.tsx`: collapsed state, Ctrl+\ / ⌘\ shortcut, `sidebarShortcutLabel()`
- `frontend/src/components/primitives/Tooltip.tsx` + `.test.tsx` (exported from `primitives/index.ts`)
- `frontend/src/api/notifications.ts`
- `frontend/src/hooks/useNotifications.ts` + `.test.tsx` (count, infinite list, optimistic mark one / all with rollback)
- `frontend/src/lib/notification-copy.ts` + `.test.ts` (sentence by type, target, time label, day groups)
- `frontend/src/components/patterns/NotificationPanel.tsx` + `.test.tsx`
- `frontend/e2e/app-rail-screens.mjs` (screenshot script)
- `plans/reports/app-rail-screens/` (20 PNG + results.json), `plans/reports/progress-app-rail.log`

Modified
- `frontend/src/components/patterns/AppSidebar.tsx` (rewritten) + `AppSidebar.test.tsx` (rewritten, 21 tests)
- `frontend/src/components/primitives/Popover.tsx`: placements `right-start`, `right-end`
- `frontend/src/lib/preferences.ts` + `preferences.test.ts`: default `sidebarCollapsed: true`, only a real boolean overrides it
- `frontend/src/components/settings/AppearanceTab.tsx`: switch description was wrong now ("tên mục hiện khi rê chuột"); label "Thu gọn sidebar" unchanged
- `frontend/src/components/settings/SettingsScreen.test.tsx`: sidebar switch is on by default; landmark name `Điều hướng chính`
- `docs/specs/lark-sidebar-layout/spec.md`: Status + requirements for the two states, toggle/shortcut/memory, rail names, Thông báo panel (the old "Sidebar Collapse 64px" requirement replaced)

Not touched: `routes/_workspace.tsx` (nothing to change: no fixed offsets anywhere, columns take the freed width; `unreadByLevel` badge still feeds `unreadCounts.messaging`), `vite.config.js` (`/api/notifications` already proxied), `MobileNav.tsx`, chat/spaces/backend.

## Findings
- `prefs.sidebarCollapsed` is NOT server-synced: `lib/preferences.ts` keeps preferences in localStorage under `ngac-prefs:<userId>` (per person, per device), "nothing here is sent to the server". So no migration and no new key: the hook wraps it, default flipped to `true`. Consequence: a person whose stored prefs already contain `sidebarCollapsed:false` (written whenever any pref was saved) stays expanded; those without the field get the rail.
- WS `notification` / `unreadCount` frames already invalidate `keys.notifications.all()` / `unreadCount()` in `websocket.store.ts`; the rail count and an open panel refresh through them. No change needed.
- Quản trị stays in personal workspaces as instructed; note `design/mockups/app-rail.html` §4 draws it hidden there (`org: true`). Mockup/code disagree: decide which wins.
- Behaviour per mockup: one `<nav aria-label="Điều hướng chính">` (old `<aside>` + inner "Phân hệ" nav removed); workspace menu lists only the OTHER workspaces under "Chuyển sang" with the open one in the header (before: all, with a tick); names wrap.
- Tooltip decides "keyboard focus" by last input (pointer vs key), not `:focus-visible` (jsdom cannot answer it).
- Toggle also closes the panel/menus; panel header counts from the list's own `unread_count`.

## NEEDS_CONTEXT (not invented)
1. NotificationRow needs the actor's name/avatar and the object's name; `GET /api/notifications` returns neither (only `type,title,body,read,created_at,entity_type,entity_id`; title/body are English with ids and DESIGN says never print them). Built: icon tile (the mockup's "no person" fallback) + sentence by type without names ("Một đề nghị của bạn đã được duyệt."), domain + time meta, link to the entity. Missing: names, rejection reason quote, realtime row overlay/"vừa duyệt" label, toasts, grouped toast. Needs backend fields (actor display name, object name, reason).
2. `notifications.entity_type` column is missing in this dev database (`GET /api/notifications` answers 500: `column "entity_type" does not exist`; `data/init.sql` has it, the table predates it). Not migrated by me (shared dev DB). The populated-panel screenshots stub the API response; the error screenshot is the real 500. Run `make db-migrate`/fix the table to see real rows.
3. Phone: Thông báo in the Thêm sheet (notifications.html §4) not built (MobileNav out of scope).

## Gates
- Touched tests: green (preferences, useSidebarCollapsed, Tooltip, useNotifications, notification-copy, NotificationPanel, AppSidebar, SettingsScreen).
- Full `npx vitest run`: 2239 pass, 1 fail = known AssetsScreen overview.
- `npm run lint`: clean. `npm run typecheck:diff`: no file above baseline.
- Build skipped.
- Playwright (real stack, fresh address, no reloads) 1280x800 and 1440x900, light+dark, rail+expanded: rail 72px, expanded 232px, default on a fresh account = rail, no page overflow, no label overflow. Extra frames: workspace menu, account menu, toggle tooltip, notification panel (rail + expanded, stubbed rows), error state. 2 console errors = the real 500s of item 2. Devtools button hidden by the script (it floats over the avatar in dev).

## Unresolved
- Mockup vs code on Quản trị in personal workspaces (above).
- Existing users with stored `sidebarCollapsed:false` keep expanded; acceptable?
