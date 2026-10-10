# Phase 04b-assets + 06 assets redesign — report

Nothing committed or staged; the git index was not touched.

## What changed

**Shell.** `/assets` is now `routes/_workspace/assets.tsx` (one screen, `AssetsScreen`): shared guard, WebSocket, logout, sidebar. Tab (`?section=`), filters, page, open asset/request/type, request filter and the new-request form live in the URL (`lib/assets-search.ts`; named `section` because a second `tab` key broke ApprovalScreen's route typing). Old URLs redirect (`routes/assets/*.tsx`, tested against the real route tree): dashboard, list, requests, types -> matching tab; `request/new` -> `?section=requests&compose=request`; `/assets/<id>` -> list with that panel; `?ws=` kept. Old layout and pages deleted. AppSidebar entry -> `/assets`; MobileNav "Work" pointed at a non-existent `/dashboard`, now `/assets`; dead `AssetNav` removed from ListPanel.

**Screens** (mockup assets.html): overview (4 figures as links, spread by state and type, "Chờ bạn duyệt", recent activity with actor; skeletons never show "0"), list (thin table, state chips with counts, type select, debounced search, Trước/Sau pager, detail panel), requests (filter chips, table, panel, reject dialog with required reason, approve-with-asset dialog, new-request panel with pickers), types (table, panel with custom-field editor and read-only lifecycle with rights in words, add-type dialog). Realtime wash on feed and rows via `useArrivals`; motion only via `lib/motion`.

**Data layer.** `api/assets.ts` rewritten against the handler structs (pinned in `api/assets.test.ts`); fixed mismatches: transition sent `action` but handler read `to_state`; request sent `justification` but handler read `reason`; `Transition.to` vs `to_state`; urgency was ignored; reject sent a fixed "Rejected". Keys only from `hooks/keys` (added activity, request, requestsAll...). Imperative fetches are hooks; every mutation is `silentError` (reported inline in its dialog) or has `meta.action`. States are the backend six.

**Backend (asset service).**
- Migration `028_asset_request_urgency_and_transition_subject.sql` (urgency; transition `subject_user_id`; index). DB backed up first; applied with `make db-migrate` and to `ngac_ci`.
- Proto: urgency, `can_decide`/`can_assign`, `assigned_asset_name`, approve `asset_id`, `HandOverAsset`, `GetSummary`, `ListActivity`, type `available_count`/`permissions`/`can_manage`, asset `assigned_to_name`, history subject, list `search`. `make proto` run; asset proto is not consumed by TS (`proto:gen` only generates ws.proto, no diff).
- Names (holder, requester, approver, actor, subject) joined through `tenant_users` of the record's workspace; outsiders are not named.
- **Approve + assign atomic**: `store.ApproveAndAssign` (row locks on request and asset; asset must be available, same type and workspace; requester must be an active member; one tx incl. history). Needs `approve` AND `manage` on the type OA, not own request. Deny/refusal tests: approve-only, manage-only, other type's OA, nothing; asset assigned/in maintenance/other type/missing; request already decided; mid-way failure rolls back; two requests, one asset.
- `HandOverAsset` (manage, recipient active member, asset available/assigned), `AssignAsset` on an approved request (manage, transactional), `ReturnAsset` now requires `assigned` and clears the holder; transitions into stock/retired clear the holder and record who it was; a bare `assign` transition is refused. Reject needs a reason. CreateRequest checks the type belongs to the workspace and validates urgency.
- `GetSummary`/`ListActivity` count only types the caller can `read`. `ListTypes` is now per type OA (any of read/write/approve/manage), returns held ops and `can_manage`.
- Custom fields: the `fields_schema` store already existed. Schema now `properties{title,type,x-kind,enum}`+`required`, validated on save and on asset values (text/number/date/person/choice).
- REST: wired list/get requests (were placeholders), assign, return, hand-over, activity, filters, custom_fields, 409/401 mapping. No vite proxy change: all new routes sit under existing `/api/assets`, `/api/asset-*` and `/api/workspaces/:id/asset*` entries.

**Specs.** `docs/specs/asset-authorization/spec.md` (3 requirements added), new `docs/specs/assets-screens/spec.md`, README index.

## Decisions where the mockup was silent
1. "Thêm tài sản" dialog added (type, name, type's fields): without it no asset or custom-field value could be entered at all.
2. Mockup's inline asset picker in the request panel replaced by the dialog (user decision); "Chỉ duyệt" offered only when nothing is available; approvers without manage get plain "Duyệt".
3. Hand-over picks and assigns immediately (mockup), no note; retire/dispose ask confirmation (not undoable).
4. Legend uses the same word everywhere ("Chờ duyệt", not "Chờ duyệt nhập kho"). Type picker shows category and ready count as a hint (no grouping). Type filter is a native select.
5. Type name/category are read-only, no "Xoá loại": rename/re-categorise need OA re-parenting, delete needs graph deletion; no backend.
6. "Từ chối"/"Đã duyệt" filters; "Đã duyệt" covers approved + fulfilled.

## Evidence
- Backend: `make test s=asset` exit 0 on `ngac` and `ngac_ci`, zero skips (new: store `screens_test.go`, grpc `screens_authz_test.go` 39 cases, rest `screens_test.go` + flow tests, domain `schema_test.go`); `go vet` clean (the earlier editor errors in screens_test.go were stale, tests compile and pass); `make check-ngac` ok; `make build-check`: asset ok.
- Frontend: lint 0; `typecheck:diff` no file above baseline (stale asset entries removed); `npm test` 49 files / 563 tests; `npm run build` ok. New tests: AssetsScreen (66, incl. no UUID rendered, no internal codes, deny cases for member), api, asset-model, asset-fields, assets-search, redirects, ws store. Forbidden-class grep empty.
- Screenshots: `reports/assets-screens/` (86): overview, list, list-detail, requests, request-detail, types, type-detail at desktop/tablet/mobile x light/dark; handover picker, retire confirm, approve dialog (open, chosen, nothing available), reject dialog (empty reason), new request (+errors), add type, add asset, type editor, realtime, reduced motion; loading/empty/error for each tab, 403, member view. Playwright (gstack) against Vite :5173 with page `fetch` answered by `src/test/asset-fixtures.ts`; server started and stopped by me.

## Concerns
1. **`ListTypes` semantics changed** (was: read on the Assets OA or 403): three earlier tests rewritten. Needed so a requester holding only `write` on one type can pick it.
2. `ReturnAsset` previously ignored history-write errors and allowed any state; now strict (one test adjusted to a real actor and assigned asset).
3. No live cross-user updates: messaging's `BroadcastAssetUpdated` is never called. Client invalidates correctly when an event arrives; the wash is tested by feed refetch only.
4. Existing assets' custom-field values cannot be edited; type rename/delete absent (see 5 above).
5. `make build-check` showed workspace service failing mid-run (other group's in-flight work, not touched).
6. Shell avatar overlaps phone sheets (pre-existing).
7. Files outside my list I touched, minimally: MobileNav, ListPanel (dead code), `websocket.store.ts` (+4 invalidations, test), `ChoicePicker` (optional `hint`), `typecheck-baseline.txt` (my 6 stale lines).

## Review fixes

No schema change this round, so no migration or DB backup was needed. New tests were written with the fixes (store, grpc, rest, domain, frontend).

- **H1** `UpdateRequestStatus` is `... AND status = 'pending'`; zero rows -> `ErrRequestNotOpen` (409, reason `request_not_open`). Tests: late approve/reject after approve-and-assign leave the request fulfilled with its asset; race tests (8 rounds each, `-race`): approve-and-assign vs plain approve, vs reject, approve vs reject: exactly one stands and request/asset agree.
- **M1** `ApplyTransition` locks `state, deleted, assigned_to`; refuses with new `ErrStateChanged` (409, `state_changed`) when state differs from `FromState` or, for a return, the holder differs from `ExpectHolder`; deleted -> not found. `ReturnAsset` passes the holder it read. `TransitionAsset` errors go through `storeErr` (no more Internal). Tests: stale step, return with a changed holder, retire vs approve-and-assign, return vs hand-over (races), grpc stale-step conflict.
- **M2** Summary holders count only `state = 'assigned'`. **M3** REST maps Internal and unknown errors to a generic body and logs the detail; test asserts no SQLSTATE or host text. Refusals now carry `reason` (ErrorInfo -> REST `{message, reason}`).
- **L1** `GetAvailableTransitions` requires read on the type OA (deny test: read on another type). **L2** write-only callers get the type without counts (ListTypes and GetType); spec updated. **L3** person-kind values must be active members of the type's workspace (`AllActiveMembers`; deny test with another tenant's user, create and update). **L4** required text not blank; a type with no fields accepts no values; asset name 1-120, reasons at most 1000 (tests). **L5** `GetType` uses the per-type model; a missing type answers like a forbidden one.
- **L6** `explainAsset` picks the sentence from the server reason (request already decided vs asset just given away vs asset changed vs person no longer a member vs wrong type vs same holder); panel actions use it (toasts), the dialogs show it inline. **L7** approve, reject and assign authorize before saying anything about the request (missing and forbidden answer identically; test).
- **Mockup gaps closed:** type filter is a "Loại: Tất cả" chip with a menu; request panel title is the request's subject (first line of the reason; requests carry no separate subject); feed also shows request decisions (approved without asset, rejected; fulfilled ones already appear as the hand-over step); maintenance card says "N quá 14 ngày" (latest step into maintenance older than 14 days, else last change); person field label tied to its picker (`aria-labelledby`, `PeoplePicker.labelledBy`). Proto regenerated (`maintenance_overdue`, `request_status`); `make proto` also rewrote the admin group's policy generated files from their current .proto (not edited by me).

Verification: `make test s=asset` exit 0, no skips, `ngac` and `ngac_ci`; asset tests also run under `-race`; `make check-ngac` ok; `make build-check` all services ok (workspace compiled this time); frontend lint 0, typecheck:diff clean, 50 files / 581 tests, build ok. Screenshots were not regenerated for these small UI changes (chip, titles, note).

Status: DONE_WITH_CONCERNS
Summary: Assets moved into the shell as four tabs on the Tín hiệu design; approve-and-assign is one transaction needing approve+manage, with names tenant-scoped, custom fields editable, specs updated, all gates green on both DBs.
Concerns: ListTypes now per type OA; no backend broadcast for live asset updates; asset field values and type rename/delete not editable.
