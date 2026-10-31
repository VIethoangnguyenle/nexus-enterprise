# Phase 06 · Entry points, retire tabs, specs, verification

## Requirements
1. Desktop entry points to module settings: where a desktop module header has a ⋯ / settings affordance, its first item
   "Cài đặt <phân hệ>" opens `/settings/<module>` (DESIGN.md §6: "⋯ của phân hệ → dòng đầu"). Find them; do not invent
   new buttons where a desktop module has no menu (list the ones found in the report).
2. Retire the unused desktop tab components (`SettingsTabs`, `ProfileTab`/`AppearanceTab`/`WorkspaceTab`/
   `NotificationSettings` if fully absorbed) with `safe` deletion: nothing may still import them; move any test that
   still describes live behaviour to the section that now owns it.
3. Specs: `docs/specs/user-settings/spec.md` (desktop section list, same addresses, touch-only rows) and
   `docs/specs/contacts-documents-settings-screens/spec.md` (tabs → section list); keep scenarios testable.
4. Verification: `npm test`, `npm run lint`, `npm run build`; Playwright screenshots of `/settings/*` at 1440 light/dark
   (org admin, org member, personal) saved to `plans/reports/desktop-settings-shots/`, compared by eye to the mockup.

## Done when
Plan acceptance criteria 1–7 hold.
