# Phase 01 · Shell, routing, wide rendering

## Context
Desktop `SettingsScreen` (components/settings/SettingsScreen.tsx) renders 4 tabs; phone renders `MeScreen` and
`/settings/$section` renders `SettingsSectionScreen` with no width check (so desktop gets the phone page today).
Mockup: `design/mockups/settings-desktop.html` (screens 1–6, `.setshell`, `.setnav`, `.setbody`).

## Requirements
1. `SettingsLayout` context (`'phone' | 'wide'`), provided by the desktop shell, read by the settings pieces. Default
   `'phone'` so phone code and tests are untouched. Hook `useSettingsWide()`.
2. Wide rendering of the pieces in `SettingsGroup.tsx`:
   - `SettingsPage` (wide): no back bar; header = section title (h1, display 19/700) + `lead` line (new optional prop:
     "Dùng chung mọi workspace" / "Trong <workspace>"), `action` beside the title; content column max-w 680, scrolls
     inside the shell's right pane. `onBack` ignored.
   - `SettingsGroup` (wide): title 13/600 muted, flat rows with divider (`divide-y divide-line`), no card bg/radius.
   - `SettingsRow` (wide): row height ≥ 52, padding 14px 0, label 15/600, hint 13 muted; value + chevron unchanged.
3. `SettingsShell` (new, `components/settings/SettingsShell.tsx`): page title "Cài đặt" (as today), then grid
   `248px | 1fr` with a top border; left `SettingsNav`, right the section. Provides layout `'wide'`.
4. `SettingsNav` (new): groups "Tài khoản" (account, notifications, appearance, sessions) and "Trong <workspace name>"
   (org: org-profile, chat, calendar, drive, approval, assets, workspace; personal: chat, calendar, drive, workspace),
   icons from lucide matching MeScreen's, labels exactly as MeScreen; each item a `<Link>` to `/settings/$section`
   keeping `ws`; current item `aria-current="page"`, `bg-accent-wash text-accent`. Bottom: `appVersionLabel()` line
   prefixed "Giới thiệu · ". `nav aria-label="Mục cài đặt"`.
5. Routing at wide width:
   - `/settings` (no tab) → shell with `AccountScreen` (section `account`).
   - `/settings?tab=ho-so|workspace|giao-dien|thong-bao` → redirect (replace) to `/settings/account|workspace|appearance|notifications`, keeping `ws`.
   - `/settings/$section` → shell with that section's page (same `PAGES` map). Org-only sections in a personal
     workspace → `/settings/account` (wide) as today's `/settings` fallback.
   - Phone behaviour unchanged (MeScreen at `/settings`, `?tab=` mapping, section pages full screen).
6. The old desktop tab components stay in the tree for now (retired in phase 06); `SettingsTabs` is no longer rendered.

## Files
Modify: `components/settings/SettingsGroup.tsx`, `SettingsScreen.tsx`, `SettingsSectionScreen.tsx`,
`lib/settings-search.ts` (tab → section map). Create: `SettingsShell.tsx`, `SettingsNav.tsx`, `settings-layout.ts`
(context + hook). Tests: `SettingsShell.test.tsx` (new); adjust `SettingsScreen.test.tsx` only where it asserts the
removed tabs (keep the assertions' intent: each old tab's content is reachable at its section).

## Tests first
- Wide `/settings` shows nav with both groups and Tài khoản và hồ sơ current; personal workspace list lacks the 3 org items.
- Each nav item links to `/settings/<section>` with `ws` kept; current item has `aria-current`.
- `?tab=giao-dien` at wide lands on `/settings/appearance`.
- Wide `/settings/approval` in a personal workspace lands on `/settings/account`.
- Version line present at the bottom of the nav.
- Phone: `/settings` still MeScreen; phone section page still shows the back button.

## Risk
`SettingsScreen.test.tsx` (711 lines) asserts tab UI. Rewrite only the tab-navigation assertions; content assertions
move to their section in later phases. Rollback: revert the phase commit.
