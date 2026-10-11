# Code review — personal workspace UI (phase 04 hiding, phase 05 convert, Tạo workspace type)

Date: 2026-10-11. Reviewer: code-reviewer (read-only). Executor: agy/Gemini, plan `plans/261010-2258-personal-workspace/exec-04-05-frontend.md`.
Checked against: `design/mockups/personal-workspace.html` §1–§9, `docs/specs/tenant-identity/spec.md` (convert + details contract), backend `services/workspace` (permission-areas, convert realtime).

## Gates (run by reviewer)

| Gate | Result |
|---|---|
| `npx vitest run` | 1611 pass / 1 fail — the known `AssetsScreen.test.tsx > overview` failure. No other failures. |
| `npx eslint` on the 23 scoped files | clean (exit 0) |
| `npm run typecheck:diff` | **FAIL**: `SettingsScreen.test.tsx` 7 new errors (this task). `spaces/ChatNavigator.tsx` 1 new error (`useRef` unused) belongs to the chat-attachments task, not this one. |
| `npm run build` | pass |

The executor report says typecheck passed. It did not: it ran tsc on only two files.

## Critical

None. No identifiers reach the screen. An unknown `type` never hides anything (`useWorkspaceType` returns `isPersonal=false`, and every hide is gated on `isPersonal`). Organization workspaces render as before. Permission saves send only the areas that changed (`PermissionEditor.tsx:117`), so hiding areas on the client cannot wipe the approval/asset grants a role already holds.

## High

1. **Typecheck gate broken by the tests** — `frontend/src/components/settings/SettingsScreen.test.tsx:65,508,542,567,585,605,625`. The fixture variable `details` is still inferred as `{name, description}`, so `type`, `is_owner`, `owner_name`, `converted_*` are excess properties (TS2353). Fix: declare it `let details: Omit<WorkspaceDetails, 'can_manage'> = …`. Merge blocker under the repo's typecheck:diff gate.

2. **Cold deep link to /assets or /approval in a personal workspace renders the area before redirecting** — `routes/_workspace/assets.tsx:14-31`, `routes/_workspace/approval.tsx:13-31`.
   - `beforeLoad` only reads the cache. `_workspace.tsx` has no loader that warms `keys.workspaces.all()`.
   - So on reload, a bookmark, a notification link, or a workspace switch (the switch drops the cache, and `useSwitchWorkspace` keeps `/assets` as the module root), `beforeLoad` sees nothing and lets the route through.
   - `AssetsRoute`/`ApprovalRoute` then render the real screen while `isLoading`, and only `<Navigate>` once the list arrives. The user sees the screen's skeleton or error state flash, plus whatever 403s the services return (spec: they refuse every call in a personal workspace).
   - Fix: `beforeLoad: async ({context, search}) => { const d = await context.queryClient.ensureQueryData(workspacesQueryOptions()); … }`. Then drop the component guard, or keep it only for a type change mid-session. Factor the guard into one helper; today it is copy-pasted twice and casts `context` needlessly.
   - The redirect tests (`routes/personal-workspace-redirects.test.ts`) only seed the cache, so this path is untested. Add a test that does not seed the cache.

3. **"Areas appear without reload" after convert is not proven** — `SettingsScreen.test.tsx` fixture `/workspaces` (line 40) returns no `type`. The success test therefore checks only the card, never the sidebar or tab bar going from personal to organization. The invalidation list in `hooks/useWorkspaces.ts:50-66` looks right: `keys.workspaces.all()` is the key `useWorkspaceType` reads, and realtime `workspace` events also invalidate it (`lib/realtime.ts:120`). But the acceptance criterion has no test. Add one that renders the sidebar (or MobileNav), converts, and expects Phê duyệt and Tài sản to appear.

## Medium

4. **Client-side area filtering contradicts the design and covers a backend gap** — `admin/PermissionEditor.tsx:45-49`, `admin/RolePanel.tsx:48-50`.
   - Mockup §4 note: "server không trả Phê duyệt, Tài sản; giao diện chỉ vẽ cái được trả, không tự lọc theo loại".
   - Backend `services/workspace/internal/domain/permission_areas.go:52` does not filter by type. The client filter is the only thing hiding them.
   - Decide: either move the filter to `ListPermissionAreas`/`grantsByArea` (correct per design and spec) and delete the client filter, or record the client filter as an intentional stopgap.
   - Admin activity (mockup §4, "hoạt động quản trị") is not filtered either.

5. **Touch targets under 44 px on phones** — `settings/WorkspaceTab.tsx:262` (convert button `min-h-9`, mockup §5: "Trên điện thoại … cao ≥ 44") and `:326` ("Đặt quyền Phê duyệt và Tài sản" link `h-8`). This breaks the executor rule "44px targets on phones". Use `max-sm:min-h-11`.

6. **Invalid listbox content in the desktop switcher** — `patterns/AppSidebar.tsx:179-231`. The container is `role="listbox"`, and the new label `div`, the separator, and the "Tạo workspace" `<Link>` now sit inside it. A listbox may only own options or groups, so screen readers will announce or skip these inconsistently. Move the label and the create link outside the listbox, in a wrapper `div`. The create link also has no `focus-ring` (the mobile one is a `Link` without it as well, `MobileMoreSheet.tsx:279`).

7. **Radio cards have no visible keyboard focus** — `auth/CreateWorkspaceScreen.tsx` (type cards). The real `input type=radio` is `opacity-0`, and the card has no `has-[:focus-visible]:` ring, so Tab and arrow keys move focus invisibly. Mockup §8 also asks for the first focus on desktop to be the selected option (`autoFocus` was removed and nothing replaced it). Add `has-[:focus-visible]:outline …` on the label and focus the checked radio on desktop.

8. **Back button on Tạo workspace always goes to /workspace-select** — `CreateWorkspaceScreen.tsx` (IconButton `navigate({to:'/workspace-select'})`). Mockup §9: when you arrive from the in-app switcher, Back returns to the workspace you left. The new switcher entry links to bare `/onboarding` with no origin, so Back sends an in-app user to the sign-in chooser.

9. **`owner_name` can be several names** — spec: "the names of the current owners joined with ', '". `WorkspaceTab.tsx:274-275` puts the joined string into one `Avatar`, which gives initials of "A, B" and reads as "Chỉ chủ sở hữu, A, B, mới…". Split on ", " and render one chip per owner, or ask the backend for an array.

## Low

10. `AppSidebar.tsx:190` — when `type` is unknown, the row says "Workspace tổ chức" (the code assumes organization). The mobile sheet (`lib/auth-flow.ts` `workspaceRowDetail`) instead falls back to the headcount line. Use one fallback in both places. Only `workspaceRowDetail` is tested.
11. `WorkspaceTab.tsx:233,285` — `details.type` undefined renders the organization card. The spec guarantees `type`, so this is acceptable, but it is a guess.
12. `WorkspaceTab.tsx:357-359` — after a successful convert, the dialog's return-focus target (the convert button) unmounts, so focus falls to `body`. Focus the card heading or the new admin link instead.
13. `WorkspaceTab.tsx:378` — `role="alertdialog"` where the mockup uses `dialog`. This is defensible for an irreversible action. Leave it, but the tests now depend on it.
14. Mockup §7: other open sessions should toast "<tên> đã chuyển workspace này thành workspace tổ chức". Realtime already refreshes their nav (`realtime.ts:120`), but there is no toast. It was not in the plan steps, so record it as a follow-up.
15. Plan step 1 "DocumentScreen/TextsTable: no Gửi duyệt": there is no Gửi duyệt button anywhere in the frontend today, so nothing needed changing. The report does not say this, and it should.

## Spec and contract check

- `POST /me/workspaces` sends `{name, type}`. The default is `personal` (`CreateWorkspaceScreen` state), and the button label follows the choice. OK.
- The details fields (`type`, `is_owner`, `owner_name`, `converted_by_name`, `converted_at`) are typed as optional and consumed. OK.
- Convert answers: 403 shows a toast, keeps the dialog open and refetches the card (matches the mockup). 409 shows a toast, closes the dialog and refetches. Any other error shows the network toast and keeps the dialog open. OK.
- Focus starts on Huỷ, and Huỷ is disabled while converting. The dialog copy matches mockup §6 verbatim. OK.
- MobileNav: in a personal workspace the 3rd tab is Danh bạ with no count, the pending-approval query is disabled, and the Thêm sheet drops Tài sản and Danh bạ (matches §2 and §7). OK.

## Exact file list for the commit (this task only)

```
frontend/src/api/auth.ts
frontend/src/api/workspaces.ts
frontend/src/hooks/useAuth.ts
frontend/src/hooks/useWorkspaces.ts
frontend/src/lib/auth-flow.ts
frontend/src/lib/auth-flow.test.ts
frontend/src/components/admin/AdminScreens.test.tsx
frontend/src/components/admin/PermissionEditor.tsx
frontend/src/components/admin/RolePanel.tsx
frontend/src/components/auth/CreateWorkspaceScreen.tsx
frontend/src/components/auth/CreateWorkspaceScreen.test.tsx
frontend/src/components/patterns/AppSidebar.tsx
frontend/src/components/patterns/AppSidebar.test.tsx
frontend/src/components/patterns/MobileMoreSheet.tsx
frontend/src/components/patterns/MobileNav.tsx
frontend/src/components/patterns/MobileNav.test.tsx
frontend/src/components/settings/WorkspaceTab.tsx
frontend/src/components/settings/SettingsScreen.test.tsx   (partial — see note)
frontend/src/routes/_workspace/approval.tsx
frontend/src/routes/_workspace/assets.tsx
frontend/src/routes/personal-workspace-redirects.test.ts
frontend/src/test/admin-fixtures.ts
frontend/src/test/auth-fixtures.ts
```

Note: `SettingsScreen.test.tsx` mixes in one PWA-task hunk, the tab-list expectation that adds 'Thông báo'. Either stage it without that hunk (`git add -p`, and use a temp index per repo hygiene), or commit after the PWA task lands `SettingsScreen.tsx` and `settings-search.ts`. Committed alone, the test fails against HEAD's `SettingsScreen.tsx`.

Not this task (exclude): `SettingsScreen.tsx`, `lib/settings-search.ts`, `main.tsx`, `routes/_workspace/channels.index.tsx`, `Dockerfile`, `vite.config.js`, `test/chat-fixtures.ts`, `lib/chat-cache.ts`, `lib/pwa-register.ts`, `sw.ts`, every chat/spaces/push/attachment file.

## Recommended actions (order)

1. Type the `details` fixture (High 1). This unblocks the gate.
2. Make `beforeLoad` await `ensureQueryData`, put the guard in one helper, and add a test with an empty cache (High 2).
3. Add a test that the sidebar or tab bar gains the areas after convert (High 3).
4. Fix the 44 px targets, the listbox structure and the radio focus ring (Medium 5–7).
5. Settle where area filtering belongs: the server, as the mockup says, or the client as a stopgap (Medium 4). This needs the plan owner's decision.

## Unresolved questions

- Area filtering: should the backend `ListPermissionAreas` drop approval and assets for personal workspaces, as mockup §4 says? If yes, the client filter goes away.
- Should "Tạo workspace" from the switcher carry its origin so that Back returns to the workspace (§9)? It needs a search param on `/onboarding`.
- Tailwind 4 classes `ring-1.5` and `ring-5` in `CreateWorkspaceScreen.tsx` were not verified in the built CSS (the dist read was blocked). Confirm the selected-card ring renders.

Status: DONE_WITH_CONCERNS
Summary: The hiding, convert and type-choice work matches the mockup and the spec contract, and vitest, lint and build pass. The typecheck gate fails on the new test fixture, cold deep links to /assets and /approval only redirect after rendering the area, and "areas appear without reload" has no test.
