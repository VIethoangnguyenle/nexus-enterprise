---
phase: 6
title: "Frontend data layer"
status: pending
priority: P1
effort: "5h"
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
- `lib/people.ts`: `PeopleDirectory` gains `isVisible(userId)` and `request`; `displayName`/`avatarOf` (`people.ts:63`,
  `:70`) call `dir.request(userId)` on a miss and read resolved entries from `byUserId`/`byNodeId`; resolved people are
  **not** added to `list` (pickers, mentions, suggestions keep showing only visible people).
- `usePeople` (`hooks/usePeople.ts`) merges contacts + resolver snapshot.
- Category hooks (lazy, `enabled` only when the category is open): departments — reuse `useDepartments`
  (`hooks/useAdmin.ts:45`, same endpoint; extend `Department` type with `hidden?: boolean`); groups — `useChannels`
  (`hooks/useMessaging.ts:16`, filter `channel_type` group, `member_count` already in payload, `api/messaging.ts:16`);
  group members — `useChannelMembers` (`useMessaging.ts:105`) only when a group is opened.
- `components/contacts/contacts-model.ts` rewrite (pure, unit-tested):
  - `sortContacts`: given name = last whitespace word, `localeCompare(…,'vi',{sensitivity:'base'})`, then full name
    (`contacts-model.ts:16` today sorts by full name).
  - `buildTree(departments, contacts)` → nodes by **id** with children, direct people, subtree count (visible people),
    sub-department count; roots = departments whose parent is absent (limited users get their own dept as a root);
    org root "Toàn <workspace>" holds people with empty `department_id`.
  - `pathOf(deptId)` full ancestry by id; `colleaguesOf` by `department_id` (leaf), not name (`:61`).
  - `rolesOf(contacts)` → Quản trị viên first (kind owners), then custom by name, each with visible holders; no Members.
  - `filterContacts` keeps accent-insensitive name/title/email + online filter; search result sub-line = title · full path.
- Admin hooks for phase 10 in `hooks/useAdmin.ts`: `useDirectoryScopeRules`, `useHideDepartment`, `useUnhideDepartment`,
  `useLimitSubject`, `useUnlimitSubject` (+ `api/admin.ts` calls), invalidating departments + contacts keys.
- Shared primitives (here so phases 07 and 09 can run in parallel): `components/primitives/PersonLink.tsx` — a button
  around avatar/name for a user id: visible → `onOpenProfile` (default: navigate to `/contacts?person=…`); not visible →
  opens `HiddenPersonCard.tsx` (popover ≥768, bottom sheet <768): avatar, name, "Hồ sơ này không còn hiển thị với bạn.",
  no title/department/email, no actions; Esc/outside closes, focus returns.
- Realtime: department/role change events already invalidate admin keys — make them also invalidate `keys.contacts`
  (find the handler for `department_changed`/`role_changed` in `stores/websocket.store.ts` or its hook; verify).

## Architecture
TanStack Query = server state (contacts, departments, channels, members, resolver results are cached in the resolver
store keyed per workspace and cleared on sign-out with other caches). No Zustand additions.

## Related Code Files
- Create: `frontend/src/lib/people-resolver.ts`, `people-resolver.test.ts`,
  `frontend/src/components/primitives/PersonLink.tsx`, `HiddenPersonCard.tsx`, `PersonLink.test.tsx`
- Modify: `frontend/src/hooks/useContacts.ts`, `hooks/usePeople.ts`, `lib/people.ts`, `lib/people.test.ts` (if present),
  `components/contacts/contacts-model.ts`, `contacts-model.test.ts`, `hooks/useAdmin.ts`, `api/admin.ts`,
  `api/admin.test.ts`, `hooks/keys/contacts.ts`, realtime invalidation file (verify path)
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
  them in the PR; intended behaviour = visible only.
- Rollback: revert; API fields are additive.
