---
title: "Menu chính thu gọn / mở rộng (dải icon)"
status: ready (start after personal-workspace UI is committed — shares AppSidebar.tsx)
owner: Claude (planning, review) · executor: agy/Gemini
created: 2026-10-11
---

# Execution plan — collapsible app sidebar

APPROVED (user, 2026-10-11): two states — 72px icon+label rail and today's 232px sidebar; **default collapsed**
on every desktop width; toggle button + **Ctrl+\\ / ⌘\\**; remembered per person on this device; add **Thông báo**;
highlight = radius-10 box like the phone tab bar. Source of truth: DESIGN.md §5 "Menu chính" (updated) and the
mockup `design/mockups/app-rail.html` + report `plans/reports/ui-ux-designer-261011-1100-app-rail.md`.

## Steps (tests first for each)
1. State: `useSidebarCollapsed()` — localStorage key per user id (`nexus-sidebar-collapsed:<userId>`, try/catch;
   default `true`). If `prefs.sidebarCollapsed` exists in the preferences store (`stores/ui.store.ts` /
   `hooks/usePreferences.ts`), check whether it is server-synced: if it is, migrate to the per-device key and stop
   writing the server one (report what you found). Ctrl+\\ / ⌘\\ toggles anywhere except inside a text field? — no:
   it toggles everywhere (the composer does not use it); `aria-keyshortcuts` on the toggle.
2. `components/patterns/AppSidebar.tsx`: render both states per the mockup — toggle row on top (IconButton 32,
   aria-expanded, labels "Thu gọn menu chính"/"Mở rộng menu chính", tooltip with the shortcut); collapsed rail:
   workspace initial button (opens the existing workspace switcher menu; menu header shows the full name wrapping),
   items 64×56 icon-in-box + 11px label (full names), active = accent-wash box + aria-current, counts as corner
   pills (99+, hidden at 0, ", N chưa đọc" hidden text), bottom Thông báo · Quản trị · Cài đặt · avatar menu
   (name, role, status, Cài đặt tài khoản, Đăng xuất). Expanded = today's sidebar + Thông báo. Respect the
   personal-workspace hiding already in AppSidebar (no Tài sản/Phê duyệt/Quản trị). Transition per mockup; reduced
   motion → instant. One `<nav aria-label="Điều hướng chính">`.
3. Tooltip primitive (if none exists in `components/primitives`): overlay token, 400ms hover / instant focus,
   Esc hides, never the only label — per DESIGN.md §6.
4. Thông báo: a panel per the APPROVED mockup `design/mockups/notifications.html` using the EXISTING messaging
   endpoints (`GET /api/notifications`, counts, `POST /api/notifications/{id}/read`, `POST /api/notifications/read-all`
   — read `docs/specs/notifications/spec.md` for the exact contract) with TanStack Query keys from
   `hooks/keys/notifications.ts`; unread count on the rail item; WS `notification` frames invalidate it if the
   store already handles them. If the mockup needs data the API does not return, build what the API supports and
   write NEEDS_CONTEXT for the rest — do not invent fields.
5. `routes/_workspace.tsx`: the list panel / content get the freed width automatically (no fixed offsets).
6. Gates: vitest (only known AssetsScreen failure), lint, typecheck:diff, build; Playwright screenshots at 1280 and
   1440, both states, light+dark → `plans/reports/app-rail-screens/`.

## Rules
- PROGRESS LOG after EVERY step: `plans/reports/agy-progress-app-rail.log`; read it first when resuming.
- Touch only: AppSidebar(+test), new hook/primitive/panel files, ui store/preferences if needed, `_workspace.tsx`
  (minimal), notifications hooks/api files. No backend. No commits, no git index.
- Report: `plans/reports/agy-<date>-app-rail.md`.
