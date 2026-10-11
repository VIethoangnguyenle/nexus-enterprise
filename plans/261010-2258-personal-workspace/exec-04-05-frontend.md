---
title: "Execution plan — personal workspace phase 04 (hide) + 05 UI (convert) + Tạo workspace type"
owner: Claude (planning, review) · executor: agy/Gemini
status: ready
---

# Execution plan — frontend only

Design (APPROVED 2026-10-11, render it exactly): `design/mockups/personal-workspace.html` §1–§9 and its notes;
decisions in `plans/261010-2258-personal-workspace/plan.md` "Quyết định" 1–6. Read both fully first.

## Backend contract (being built in parallel by Claude — code against it; mock it in tests)
- `useWorkspaceType()` (`frontend/src/hooks/useWorkspaceType.ts`) → `{ type, isPersonal, isLoading }` — exists.
- `POST /api/me/workspaces` body `{name, type: 'personal'|'organization'}` → `MyWorkspace` incl. `type` — exists.
- `GET /api/workspaces/{id}/details` → adds `type`, `is_owner` (bool), `owner_name`, `converted_by_name`,
  `converted_at` (RFC3339 or empty) to the existing `{name, description, can_manage}`.
- `POST /api/workspaces/{id}/convert-to-organization` → 200 `{type:'organization', converted_by_name,
  converted_at}`; 403 not the owner; 409 already an organization. After success the client must invalidate
  the workspaces list (`useWorkspaces` key), `/me`, details, and every query of assets/approval so the areas
  appear at once.

## Steps (tests first for each; vitest)
1. Hide in a personal workspace (§1–§5): `components/patterns/AppSidebar.tsx` (no Tài sản, Phê duyệt;
   label "Workspace cá nhân" under the name), `MobileMoreSheet.tsx` (no Tài sản), `MobileNav.tsx` (3rd tab is
   Danh bạ `/contacts`, no count, instead of Phê duyệt), `components/documents/DocumentScreen.tsx` + `TextsTable.tsx`
   (no "Gửi duyệt"; "Chia sẻ" not promoted), admin roles screen (no Phê duyệt / Tài sản areas, no Duyệt column —
   find it under `components/admin/`). While `type` is unknown (`isLoading`), show nothing extra and hide nothing
   that would flash (follow `useWorkspaceType` semantics: unknown ≠ personal).
2. Route guards: `routes/_workspace/assets.tsx` and `approval.tsx` → in a personal workspace redirect to
   `/channels` (Tin nhắn), no error page.
3. Tạo workspace (§8, §9): `components/auth/CreateWorkspaceScreen.tsx` gets the type cards (default Cá nhân),
   one-way note, button text follows the choice, sends `type`; switcher entry "Tạo workspace" row at the end of
   the workspace list (desktop sidebar menu + phone "Đổi workspace" sheet), each row showing its type line.
4. Cài đặt → Workspace card "Loại workspace" (§6, §7): owner sees "Chuyển thành workspace tổ chức"; others see
   the explanation naming the owner. Confirm dialog per mockup (lists Tài sản, Phê duyệt, Duyệt tài liệu; says
   messages/documents/members/roles stay; says hidden assets and approval requests reappear; "Không chuyển ngược
   lại được."; focus starts on Huỷ; in-progress and network-error states). Success: toast "Đã chuyển thành
   workspace tổ chức", card shows who/when + shortcut to Quản trị permissions, invalidations above. 403/409 →
   messages from the mockup (409: refresh the card, it is already an organization).
5. Gates: `cd frontend && npx vitest run` (only the known AssetsScreen failure), `npm run lint`,
   `npm run typecheck:diff`, `npm run build`.

## Rules for the executor
- PROGRESS LOG (mandatory): after EVERY step append `<time> | step | files | result` to
  `plans/reports/agy-progress-personal-ws-04-05.log`; read it first when resuming and skip what is done.
- Do NOT edit: `backend/**`, `frontend/src/components/spaces/**`, `frontend/src/components/chat/**`,
  `frontend/src/stores/websocket.store.ts`, `frontend/src/api/messaging.ts` (other work is in progress there).
- No UUID/ID on screen; 44px targets on phones; Vietnamese copy from the mockup. Don't invent UI not in the mockup —
  write NEEDS_CONTEXT in the report for that item instead.
- Do NOT commit; do NOT touch the git index.
- Report: `plans/reports/agy-<date>-personal-ws-04-05-ui.md` — files, tests, gate output, skipped items.
