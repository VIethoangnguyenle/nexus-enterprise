---
phase: 6
title: "Frontend data layer"
status: "completed (not committed)"
priority: P1
effort: "6h"
dependencies: [5]
---

# Phase 6: Frontend data layer

## Overview
Types, hooks and pure model functions every UI phase uses. Splits "browse directory" (`useContacts`, visibility-filtered)
from "resolve names" (`usePeople` falls back to a batched resolver) without touching the ~80 `usePeople` callsites.

## Requirements
- `Contact` (`frontend/src/hooks/useContacts.ts:15`) gains `department_id: string`, `roles: {id, name, kind}[]`;
  response gains `visibility: 'all' | 'limited'`. `useContacts` keeps loading the whole list app-wide (brainstorm: no lazy
  people).
- Resolver `frontend/src/lib/people-resolver.ts`: per-workspace store; `request(id)` idempotent, batches misses in a
  microtask/50 ms window, `POST /workspaces/:id/contacts/resolve` (≤200 per call), caches results and "not found";
  subscribe via `useSyncExternalStore`. Never fetches during render synchronously (enqueue only).
- `lib/people.ts`: `PeopleDirectory` (`people.ts:17`) gains `isVisible(userId)` and `request`; `displayName`/`avatarOf`
  (`people.ts:60`, `:68`) call `dir.request(userId)` on a miss and read resolved entries from `byUserId`/`byNodeId`; resolved people are
  **not** added to `list` (pickers, mentions, suggestions keep showing only visible people).
- `usePeople` (`hooks/usePeople.ts`) merges contacts + resolver snapshot.
- Category hooks (lazy, `enabled` only when the category is open): departments — reuse `useDepartments`
  (`hooks/useAdmin.ts:45`, same endpoint; extend `Department` type (`api/admin.ts:5`) with `hidden?: boolean`); groups —
  `useChannels` (`hooks/useMessaging.ts:16`; `ListChannels` returns only channels the caller can read since the private-groups
  fix; keep `channel_type === 'private'`, which drops `#general` (`workspace` type, everyone) — plan Q2; `member_count`
  already in payload, `api/messaging.ts:16`); group members — `useChannelMembers` (`useMessaging.ts:105`) only when a group
  is opened (members-only read; `can_remove_members` ignored here).
- `components/contacts/contacts-model.ts` rewrite (pure, unit-tested):
  - `sortContacts`: given name = last whitespace word, `localeCompare(…,'vi',{sensitivity:'base'})`, then full name
    (`contacts-model.ts:16` today sorts by full name).
  - `buildTree(departments, contacts)` → nodes by **id** with children, direct people, subtree count (visible people),
    sub-department count; roots = departments whose parent is absent (limited users get their own dept as a root);
    org root "Toàn <workspace>" holds people with empty `department_id`.
  - `pathOf(deptId)` full ancestry by id; `colleaguesOf` by `department_id` (leaf), not name (`:54-56`).
  - `rolesOf(contacts)` → Quản trị viên first (kind owners), then custom by name, each with visible holders; no Members.
    Owners are always in the payload (plan decision), so a limited user's Vai trò still lists Quản trị viên.
  - `filterContacts` keeps accent-insensitive name/title/email + online filter; search result sub-line = title · full path.
- Admin hooks for phase 10 in `hooks/useAdmin.ts`: `useDirectoryScopeRules`, `useHideDepartment`, `useUnhideDepartment`,
  `useLimitSubject`, `useUnlimitSubject` (+ `api/admin.ts` calls), invalidating departments + contacts keys.
- Shared primitives (here so phases 07 and 09 can run in parallel): `components/primitives/PersonLink.tsx` — a button
  around avatar/name for a user id: visible → `onOpenProfile` (default: navigate to `/contacts?person=…`); not visible →
  opens `HiddenPersonCard.tsx` (popover ≥768, bottom sheet <768): avatar, name, "Hồ sơ này không còn hiển thị với bạn.",
  no title/department/email, no actions; Esc/outside closes, focus returns.
- Realtime: nothing to add. `frontend/src/lib/realtime.ts:122-123` already invalidates `keys.contacts.every()` and
  `keys.admin.everything()` on every workspace event (incl. `department_changed`, `role_changed`); the phase-04 rule
  writes announce those kinds. Resolver cache is not invalidated by them (names do not depend on rules); `profile_updated`
  refreshes it.

## Architecture
TanStack Query = server state (contacts, departments, channels, members, resolver results are cached in the resolver
store keyed per workspace and cleared on sign-out with other caches). No Zustand additions.

## Related Code Files
- Create: `frontend/src/lib/people-resolver.ts`, `people-resolver.test.ts`,
  `frontend/src/components/primitives/PersonLink.tsx`, `HiddenPersonCard.tsx`, `PersonLink.test.tsx`
- Modify: `frontend/src/hooks/useContacts.ts`, `hooks/usePeople.ts`, `lib/people.ts`, `lib/people.test.ts` (if present),
  `components/contacts/contacts-model.ts`, `contacts-model.test.ts`, `hooks/useAdmin.ts`, `api/admin.ts`,
  `api/admin.test.ts`, `hooks/keys/contacts.ts`
- Leave untouched: `lib/suggestions/contacts.ts` (works on the visible directory — hidden people correctly not suggested;
  owned by 261017-1516)

## Implementation Steps (TDD)
1. Vitest first:
   - sort: "Nguyễn Văn An" before "Trần Thị Bình"; equal given names ordered by full name; accents.
   - tree: two "Đối soát" under different parents keep their people; subtree counts; limited root; no-dept at org root.
   - path/colleagues by id; rolesOf excludes Members, owners first.
   - resolver: 3 misses in one tick → one POST; repeat miss → no refetch; 250 ids → 2 calls; error → retried on next miss
     after backoff, name falls back to `UNKNOWN_PERSON` meanwhile; hidden person resolved → `displayName` returns name,
     `isVisible` false, not in `list`.
   - sign-out clears resolver cache.
   - PersonLink: visible → profile callback; hidden → card with the sentence and no buttons/email/title; keyboard + Esc.
2. Implement; `npm test`, `npm run lint`, `npm run build`.

## Success Criteria
- No `usePeople` callsite changed; a message from a hidden person renders their real name in a test of `MessageBlock`.

## Risk Assessment
- Render-time enqueue causing render loops (Med×Med): enqueue does not notify synchronously; notify only after fetch.
- Many components using `dir.list` for "everyone" semantics (Low×Med): grep `\.list` usages of PeopleDirectory and list
  them in the PR; intended behaviour = visible only. Known on main: calendar pickers (`calendar/AddCalendarPicker.tsx:28`,
  `GuestsPanel.tsx:74`, `EventEditor.tsx:57`) become visibility-scoped (plan Q3).
- Rollback: revert; API fields are additive.

## Todo / Status (2026-10-18)
- [x] Types: `Contact` gains `department_id`, `roles[{id,name,kind}]`; `useContacts` returns `{contacts, total, visibility}` and fills both fields for an older server. `Department.hidden?`, `DirectoryRules`, `ResolvedPersonWire`.
- [x] `api/contacts.ts` (`resolve`) and `lib/people-resolver.ts`: one store per workspace, `request`/`requestNode` only queue, 50 ms batch, <= 200 ids per call (250 -> 2), found and not-found cached, in-flight de-duplicated, failure retried after a 2 s doubling backoff (max 60 s), `refresh()` (profile changed) keeps old names until the new ones land, `clear()` drops everything and ignores answers still in flight. Ids that are not UUIDs are never sent.
- [x] `lib/people.ts`: `PeopleDirectory.isVisible` and `request`; resolved people are in `byUserId`/`byNodeId` and not in `list`; `displayName`/`avatarOf` queue a miss. `usePeople` merges contacts + resolver snapshot (`useSyncExternalStore`) and asks nobody until the directory has loaded. No `usePeople` call site changed.
- [x] `contacts-model.ts`: `givenNameOf`/`sortContacts` (given name, then full name), `buildTree`, `pathOf`/`pathLabel`, `searchSubline`, `colleaguesOf` by `department_id`, `rolesOf` (Quản trị viên first, no Members), `privateGroups`; the old name-based `departmentsOf`/`department` filter stay for the current screen (phase 07 replaces them).
- [x] `hooks/useContactCategories.ts`: `useDepartmentTree` (lazy), `useRoleGroups` (from loaded people), `useMyGroups` (lazy, private groups only), `useGroupMembers` (lazy per opened group, hidden members named by the resolver). `useDepartments`, `useChannels`, `useChannelMembers` gained an optional `enabled`.
- [x] Admin: `useDirectoryScopeRules` (`ready:false` is data), `useHideDepartment`, `useUnhideDepartment`, `useLimitSubject`, `useUnlimitSubject` + `adminApi` calls + `keys.admin.directoryScope`; each write invalidates rules, departments and `keys.contacts.all(ws)`.
- [x] `components/primitives/PersonLink.tsx`, `HiddenPersonCard.tsx` (popover >= 768, `NavSheet` bottom sheet below; Esc/outside closes, focus returns; only avatar, name and the sentence). Not re-exported from the primitives index (import cycle through `NavSheet`).
- [x] Sign-out clears the resolver (`onSessionEnd`), a workspace switch clears it (`resetTenantScopedState`), `profile_updated` refreshes it (`websocket.store`).
- [x] Tests written first (red for the right reason), then green: see the report; `npm test` 3534 pass, lint clean, typecheck:diff clean, build green.

## Deviations
- `PersonLink` takes the `people` directory as a prop (the caller has it, like `MessageBlock`) instead of reading the workspace itself.
- Default `onOpenProfile` navigates with `search: { person }`; the `/contacts` route does not declare `person` yet (phase 07 adds `validateSearch`), so the call carries a documented `as never` cast.
- Sign-out reaches the resolver through `lib/session-end` rather than an import in `auth.store`: the direct import made a cycle (`auth.store` -> resolver -> API client -> `auth.store`) that broke mocking of the client in tests. `query-persist.checkScope` is left alone: the resolver is memory only and empty at boot.
- Touched outside the listed files: `ContactsScreen.tsx` (personal workspace also blanks `department_id`, since colleagues now follow the id) and its test (new sort order), `suggestions/contacts.test.ts` and `SpaceFiles.test.tsx` fixtures (new required fields), `chat-fixtures.ts` (department ids), `useMessaging.ts`, `websocket.store.ts`, `useSwitchWorkspace.ts`.
- Direct `people.byUserId.get(id)` callers (about 25) see a resolved person only after something called `displayName`/`avatarOf` for that id (most render a name beside the avatar, so they do). Not changed, per "no call site changes".

## Review fixes (2026-10-18)
- [x] H1: `PeopleDirectory.ready` (`EMPTY_DIRECTORY` false, built from contacts true; `usePeople` serves the empty one while loading and on error). `PersonLink` has three states: unknown (press does nothing, `aria-busy`, no `aria-haspopup`/`aria-expanded`, no card), visible, hidden (only these get the popup attributes). `useGroupMembers().visible` is `undefined` until the directory is ready.
- [x] M1+M2: `byUserId`/`byNodeId` are `AskingMap`s: `get` of a missing key queues `request`/`requestNode` (`has`, `peek`, iteration do not); answered and not-found ids are never asked again. `requestNode` is part of `DirectoryOptions`/`PeopleDirectory`. `ApprovalScreen` no longer returns a node id as a user id (`byNodeId` path, `undefined` while unresolved). The ~25 direct-map call sites resolve hidden people unchanged.
- [x] M3: after a failure one timer fires when the earliest backoff ends and re-asks the due ids (up to 5 failures in a row, then only a new request retries); `clear()` cancels it.
- [x] M4: `dmPartner(ch, me, dir, members?)` never titles a DM by a username: a partner the directory does not list is looked up by user id among the channel's members (`useConversations` loads members only for such DMs; `SpaceView` reuses its members query) and named by the resolver, "Thành viên" until then.
- [x] L1: `refresh(ids?)`: ids in flight are marked stale and asked again when their answer lands; node-only entries refresh by node; with ids (the `profile_updated` event's) only those held people are asked about, without ids everyone held is.
- [x] L2: `buildTree` drops departments unreachable from a root (parent cycle) and files their people at the organisation root.
- Tests that failed first: `people.test` (ready, AskingMap), `PersonLink.test` (unknown state), `useContactCategories.test` (visible while loading/failed), `usePeople.resolver.test` (direct id/node lookups, not ready on error), `conversations.test` (username title, mentions request), `people-resolver.test` (auto retry, give-up, in-flight refresh, targeted refresh), `contacts-model.test` (cycle people), `websocket.store.test` (refresh with ids).
