# Code review: search suggestions, phase 04 (Tài liệu, Tài sản) and phase 05 (Phê duyệt)

Scope: commit e1dcc7c plus uncommitted `frontend/src` changes (ApprovalScreen, ApprovalTable, approval-search, Marked, suggestions/approval, ChatSearchScreen extraction). Shared hook/pattern pieces were reviewed earlier and are only referenced here.
Verification re-run by reviewer: vitest on approval, drive, assets and suggestions suites: 239/239 passed.

## High

### H1. Batch approve can approve rows the search is hiding
`frontend/src/components/approval/ApprovalScreen.tsx:132-136`, `:280-283`, `:386`, `:521-527`

`selection` is `ticked` intersected with **`rows`** (the whole loaded tab), not with `shownRows`. Reproduce: on Chờ tôi duyệt tick A and B, then type a word that only matches B. The table shows B alone, but the bar reads "Đã chọn 2 đề nghị", and "Duyệt 2 đề nghị" sends `[A, B]` (`doBatch` spreads `selection`). The user approves a request they cannot see at that moment. The comment at :385 ("a search narrows what one tick approves") claims the opposite.

The select-all toggle is also wrong in the same state. `ApprovalTable.tsx:87` computes `allTicked` over the shown rows, so the header box shows as checked. `onToggleAll` compares `selection.size` (3) with `shownRows.length` (2), so clicking the checked box does not untick. It sets `ticked = shownRows`, which quietly drops the hidden A, and the box stays checked. The user ends up unticking something they cannot see.

Fix: make selection follow what is shown, and toggle against it:
```ts
const shownIds = useMemo(() => new Set(shownRows.map((r) => r.request.id)), [shownRows])
const selection = useMemo(() => tab !== 'pending' ? new Set<string>() :
  new Set([...ticked].filter((id) => shownIds.has(id))), [ticked, shownIds, tab])
onToggleAll: () => setTicked(shownRows.every((r) => selection.has(r.request.id)) ? new Set() : new Set(shownRows.map((r) => r.request.id)))
```
Alternative: clear `ticked` whenever `typed` changes. Either way, add a test: tick 2 rows, filter to 1, expect "Duyệt 1 đề nghị", and expect the batch request body to hold one id. No test today covers selection together with search. The new tests (lines added to ApprovalScreen.test.tsx) only cover filtering, empty state, tab reset, suggestions and keeping a search.

## Medium

### M1. Tài sản keeps the TabBar live while phone suggestions cover the list
`frontend/src/components/assets/AssetsScreen.tsx:409-418` (TabBar not gated by `covered`); compare with `ApprovalScreen.tsx:443-456`, which hides it.
On a phone with the suggestions open, a tab tap unmounts `AssetListBody`, but `panel` stays engaged because the screen variant does not close on blur. When the user comes back to Danh sách, `covered` is still true and the overlay is back without a focused box. This is also inconsistent with Phê duyệt, and with the commit's claim that "the filters step aside". Gate the TabBar on `!covered` as Approval does, or call `panel.close()` on tab change.

### M2. Phê duyệt: a search typed less than 300 ms before a tab switch moves to the new tab
`ApprovalScreen.tsx:175-181`. `tabSearch` drops `q`, but if `q` was never committed, `search.q` stays `undefined`. The sync effect at :176 then does not fire and `typed` survives. The pending timeout (or a new one, since `search.q` did not change) writes `?q=` onto the new tab. This breaks the contract in `approval-search.ts` ("another tab starts without them"). The test "starts empty on another tab" presumably waits past the debounce. Fix: reset `typed` in the tab-change effect at :130 (`useEffect(() => { setTicked(new Set()); setTyped('') }, [tab])`).

### M3. Debounced URL write can overwrite keystrokes typed while it lands (Approval; same pattern already in Assets)
`ApprovalScreen.tsx:176` and `AssetsScreen.tsx:115`. When the router commits `q="ab"` while the input already holds `"abc"`, the effect `setTyped(search.q)` overwrites it. The window is the router's commit latency, so this is rare locally and more likely on slow devices. The Assets code had this before this change; Approval copies it. Guard it: only sync from the URL when `search.q` differs from the value last written (keep it in a ref), or when the change did not come from this debounce. Untested.

## Low

- L1. `DriveScreen.tsx:335`: in the shared view the group reads "Sửa gần đây trong Được chia sẻ với tôi", because `title` is the view name there. Use a different label when `shared` is true, e.g. "Sửa gần đây".
- L2. `lib/suggestions/assets.ts:30` names the actor from `e.actor_name` only. The avatar at `AssetsScreen.tsx:150` uses `assetPerson(people, …)`, which also checks the directory. When `actor_name` is empty, the meta says "Không rõ" next to a correctly named avatar. Pass the resolved name into `stepText`.
- L3. Drive: a file opened from the desktop popover sets the opener to `searchRef` (`DriveScreen.tsx:323`, and through `openPreview(f, searchRef.current)`). Closing the viewer returns focus to the box. Its `onFocus` clears `dismissed`, so the popover opens again over the files. This is acceptable for "back where you were", but confirm it matches DESIGN.md. No test covers focus return for suggestion-opened files (the existing focus tests are at DriveScreen.test.tsx:466/484/868 and cover list and card opens only).
- L4. `ApprovalTable` gets `highlight={typed}`, so every memoised row re-renders on each keystroke. That is fine at loaded-page sizes and the list filters anyway. `Marked` only marks the title, so a row that matched by sender or kind shows no mark. This is cosmetic.
- L5. Drive `loneFile` uses the current workspace's `people` for the owner of a file shared from elsewhere, so the owner may show as unknown. `sharedWithMe` is not scoped to a workspace (`api/drive.ts:225`). Cosmetic. Download stays authorised by the server (`/drive/files/:id/download`), and `canOpenInDrive` is off for `origin="drive"`, so no navigation goes to a foreign folder.

## Checked, no issue found

- Deny/visibility: every suggested item comes from a list the server returned to the caller. Sources are `useApprovalPending`/`useApprovalMyRequests`, the open drive listing, `drive/shared-with-me`, the asset activity feed, and `useAssets({assigned_to})`. The backend filter `store.go:166` sits inside the `VisibleTypeIDs` restriction, so assets of types the caller cannot read are not suggested. No client-side widening.
- Raw identifiers: ids are used only as React keys and in `onSelect`. Titles, metas and avatars render names; `stepSegments` renders names and pills, never ids. The empty-state text uses the user's own query. This satisfies the identifiers-in-UI rule.
- Esc ordering: the popover Esc in `useSuggestionPanel` calls `preventDefault`. The phone `covered` Esc in all three screens calls `preventDefault`. The window listeners for the panel and selection (`DriveScreen.tsx:209`, `ApprovalScreen.tsx:256`, `AssetsScreen.tsx:190`) skip events that are `defaultPrevented`. One Esc closes one thing.
- Drive gallery memo: `openPreview` depends on `[folderFiles, keepSearch]`. `keepSearch` is stable (`useRecentSearches` `add` is in `useCallback([module, me])`), and the query is read through `queryRef`, so typing does not change the handlers passed to the gallery or table.
- Tablet: `phone` means `layout === 'phone'`, so the tablet gets the popover. `onBack` is passed only when `covered`. Toolbar layout is unchanged otherwise.
- Tabpanel ARIA: when `covered`, the TabBar is unmounted and the `aria-labelledby` target disappears, but the panel is `aria-hidden` and `inert` at the same time, so nothing dangles for assistive tech.
- URL: `querySearch` uses `replace: true`, so no history entry per pause. Assets' `go` still pushes (behaviour that predates this change).
- `assigned_to` exists end to end: `api/assets.ts:148`, then `rest/handler.go:198`, then `store.go:166`. The test fixture mirrors it.
- `Marked` extraction is behaviour-preserving for ChatSearchScreen. The new empty-query short-circuit only skips a no-op `highlight`.
- Conventions: tokens only (`bg-sunk`, `text-ink-muted`, `bg-warning-wash`, `z-sticky`), primitives only (`SearchField`, `IconButton`, `Avatar`, `Button`), no new REST routes, no proto/schema.

## Untested risks
1. Batch approve or select-all while a search hides ticked rows (H1).
2. Tab switch inside the 300 ms debounce (M2), and URL sync clobbering keystrokes (M3).
3. Assets phone: switching tab while the suggestions are covering the list (M1).
4. Drive: focus return and popover re-open after closing a viewer opened from a suggestion (L3). The lone-file viewer opened from Được chia sẻ với tôi while a folder file preview index is stale.

Status: DONE_WITH_CONCERNS
Summary: Phase 04 holds up on visibility, Esc ordering and memo stability. Phase 05 has one real defect: batch approve and select-all act on ticked rows the search is hiding.
Concerns: H1 should be fixed before merge. M1 and M2 are small. M3 is a pattern that predates this change and now has a second copy.
