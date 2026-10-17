# Code review: phase 03 (Chat + Danh bạ search suggestions)

Date: 2026-10-17. Branch `feat/search-suggestions`, working tree on top of `d0445e8`. Review only, no edits.

## Scope
- `frontend/src/hooks/useSuggestionPanel.ts` (Esc now popover-only)
- `frontend/src/hooks/useRecentSearches.ts` (new)
- `frontend/src/lib/suggestions/chat.ts`, `contacts.ts` (+ tests, new)
- `frontend/src/components/spaces/ChatSearchScreen.tsx` (+ test)
- `frontend/src/components/contacts/ContactsScreen.tsx` (+ test)
- `frontend/src/components/patterns/SearchSuggestions.test.tsx` (+1 case)
- Re-ran focused tests: 5 files, 91/91 pass. Did not re-run the full suite, lint or build; the author reports all green.

## Overall
The deny rules hold. Suggestions come only from lists the server already returned. The person using the app is filtered out by id. A DM partner outside the workspace directory is dropped twice: by `dmPartner` → `usePeople`, then by `recentlyMessaged`'s `byId`. Nothing here renders an id: `key` and `hueKey` are never shown, titles are names, and meta is a role line or "author: preview". `formatListTime(new Date(c.lastActivity))` is correct: `lastActivity` is epoch ms (`conversations.ts:247` via `toMillis`), and a bare number would have been read as unix seconds by `toDate`, so wrapping it in `Date` is the right call. Zero activity is filtered out first. Esc ordering on desktop Contacts is right too. The popover's React handler calls native `stopPropagation` plus `preventDefault` at the root container, so the profile's `window` keydown listener never fires (`ContactsScreen.tsx:76-83` also checks `defaultPrevented`).

No critical issues. There is one probable behaviour regression in ChatSearchScreen, a real cost problem in Contacts, and two DESIGN.md deviations on Contacts phone that the plan does not list.

## Critical
None.

## High
None.

## Medium

### M1. ChatSearchScreen: suggestions now depend on a programmatic focus that may never happen
`ChatSearchScreen.tsx:105-123, 213-214` together with `useSuggestionPanel.ts:87, 131-134`.
Before this change, the recent list rendered whenever `typed === ''`. Now `<SearchSuggestions>` renders only when `panel.visible`, which needs `engaged`, which is set only by the input's `onFocus`. The only focus comes from `useModalFocus`'s `requestAnimationFrame` → `inputRef.focus()` (`useModalFocus.ts`, initial-focus effect).
- Proven: the screen is blank for at least one frame. The test had to change from `getByText('Tìm gần đây')` to `await findByText(...)` (ChatSearchScreen.test.tsx, "before anything is typed").
- Risk, not tested on a device: iOS Safari restricts programmatic `focus()` outside a user gesture, and rAF runs outside the tap's gesture. If the focus is dropped, the screen shows only chips and an empty area until the user taps the box. The old screen always showed the recent list.
- Fix: this screen *is* the search, so start it engaged. For example, add `initiallyEngaged?: boolean` to `useSuggestionPanel` (or `engaged` default `variant === 'screen' && opts.autoOpen`) and pass it from ChatSearchScreen. Contacts keeps the focus-driven behaviour.

### M2. Contacts mounts the full `useConversations` pipeline for a five-row hint
`ContactsScreen.tsx:99-100`.
On `/contacts` nothing else mounts `useConversations`: `ListPanel` renders `ChatNavigator` only for `activeModule === 'messaging'`, and `MobileNav` only on chat list paths. So this hook now adds the following on desktop and phone alike, whether or not the box is ever focused:
- It sends a WS `subscribe` for every space and DM (`useConversations.ts:77-81`, `websocket.store.ts:352-363`). The server then pushes every message in every conversation to a screen that shows none of them.
- It selects `lastMessages` and `lastThreadMessages` from the store (`useConversations.ts:36-37`). Every incoming message anywhere rebuilds `conversations`, and `directs` is a fresh `.filter` array every render (`:86`), so `lately`'s memo never hits. All of `ContactsScreen` re-renders, including the unmemoized `ContactsTable`/`ContactCards` over the whole directory. `openProfile` is also recreated every render.
- It fires 4 queries plus `usePeople`: channels, dms, unread, activity.
The plan specified "`usePeople` + danh sách DM". Recommended fix: a small selector that uses only `useDirectMessages()` + `useConversationActivity()` (+ `usePeople` for `dmPartner`), with no `subscribeMany` and no `lastMessages`. Ideally gate it with `enabled` behind the first focus of the box (DESIGN.md already uses "chỉ tải khi gợi ý mở lần đầu" for Tài liệu). Activity is a startup snapshot, which is good enough for "Nhắn gần đây".

### M3. Contacts phone: "Quay lại hoặc back: về danh sách, giữ vị trí cuộn" is not met
`ContactsScreen.tsx:254-258` and `206-220`.
- Scroll position: the suggestions replace `body` inside the same `overflow-y-auto` container. The shorter content clamps `scrollTop`, so after Back the list is at the top.
- System back: no history entry is pushed. ChatSearchScreen owns one, per the comment in `PhoneSearchBar.tsx`. Android back or the swipe gesture therefore leaves Danh bạ entirely instead of closing the suggestions.
- Hardware Esc (tablets under 1024 with a keyboard): the screen variant no longer handles Esc (`useSuggestionPanel.ts:120`), and Contacts has no modal to take it. Esc does nothing to the suggestions; on `type="search"` it only clears the native field.
- Fix: keep `body` mounted and hidden (`hidden` attribute) under the suggestions, or render the suggestions as an overlay over the scroller. Push a history entry or use `useChatOverlay`-style state for the screen. Call `panel.close()` on Esc in the Contacts `onKeyDown` when `phone`.
None of these is listed as a deliberate deviation in phase-03 "Thực tế khi làm"; only the 52px pill is listed.

## Low

### L1. Contacts phone: filters act on a list the user cannot see
`ContactsScreen.tsx:236-245`. While the suggestions cover the list, the department filter, "Đang trực tuyến" and the 640–1023 view toggle stay live. The tap blurs the box, but by design the screen stays up, so the filter changes with no visible effect until Back. Either hide the filter row while `phone && panel.visible`, or `panel.close()` when a filter changes.

### L2. Enter during IME composition records a search
`ContactsScreen.tsx:228-231`. A raw `keydown` Enter check with no `e.nativeEvent.isComposing` guard. With macOS Vietnamese/Telex marked text, the Enter that commits the composition also records the search. Chat avoids this because it uses form `onSubmit`. The same gap exists in the popover Enter at `useSuggestionPanel.ts:114` (phase 02). Add `if (e.nativeEvent.isComposing) return`.

### L3. Focus is lost after a profile opened from a phone suggestion closes
Picking "Nhắn gần đây" calls `select` → `dismissed`, which unmounts the suggestion button that was focused. `ContactPanel`'s return-focus target is then detached, and focus falls to `body`. Minor a11y issue.

### L4. Leftover fallback in `useRecentSearches`
`useRecentSearches.ts:15`. `state.me === me ? state.list : []` never takes effect: React discards the render output after `setState` in render and re-renders immediately. Harmless. The setState-during-render itself (`:14`) is the documented "adjust state on prop change" pattern and cannot loop, because the guard converges in one pass. Note that for `'chat'`, `readRecent` performs the legacy-key migration write during render. It is idempotent, so it is safe under StrictMode double render, and it was already done in the old `useState` initializer.

### L5. "Trò chuyện gần đây" has no `loading`
`ChatSearchScreen.tsx:111-121`. While `nav.conversations` is still loading, the group is absent and then pops in. DESIGN.md asks for 2 skeleton rows while a group loads. Pass the loading flag through from `PhoneChatHome`.

### L6. Unrelated changes in the working tree
`frontend/e2e/wave1-two-people.mjs` (+116/-183, OTP rate-limit rework, hard-coded test address) and many `plans/261010-*/reports/*.png` and `plans/reports/video-call-*` files are modified. They are not part of phase 03. Do not include them in the phase-03 commit (use a temp index, as with the staged `.claude/skills` deletions).

## Acceptance check (phase-03)
| Criterion | Status |
|---|---|
| Sort by last activity, limit N (chat 5, contacts 5) | Met. Unit tests cover it. Display limit is enforced again by LIMITS. |
| Deny: unreadable conversation never suggested | Met. `chat.test.ts` covers the pure function; `ChatSearchScreen.test.tsx` "never suggests a conversation the person can no longer read". |
| Deny: person outside directory / me | Met. `contacts.test.ts` covers both. |
| No ids in title/meta | Met. `expectNoIds` in the Contacts component test. |
| Focus on empty box shows groups; typing hides them | Met in tests. On a real phone, see M1. |
| Save search on Enter / opening a result | Met (Contacts Enter + openProfile; Chat unchanged). IME caveat in L2. |
| ChatSearchScreen.test expectations unchanged except the redesign | Mostly. One `getByText` became `findByText`, which is itself the evidence for M1. The remove label changed case ("Tìm gần đây"), as DESIGN.md specifies. |

## Regressions checked and found OK
- ChatSearchScreen: Esc and back close the modal. The screen variant leaves Esc unhandled and `useModalFocus` takes it; there is a pattern-level test, but no integration test with focus in the box (see untested risks). Focus trap is unchanged. Chips are unchanged. Recent reuse and remove work through the shared group, and `reuse` re-focuses the box. Opening a DM is unchanged. `autoComplete="off"` is now supplied by `inputProps`.
- Contacts: filters, the `combobox` role swap (tests updated), the `/dms` POST assertion tightened by method, and profile Esc both with and without the popover.
- `PhoneSearchBar.goToSearch`: `data-module-search` sits on the `<input>` (`SearchField.tsx`), so `focus()` engages the screen variant. The Back button is a conditional sibling slot, so the input does not remount when it appears.

## Untested risks
1. iOS Safari: whether the rAF `focus()` in ChatSearchScreen fires `focus` (M1). Needs a device check.
2. ChatSearchScreen Esc with focus in the empty box while suggestions show. Only the pattern-level test (`SearchSuggestions.test.tsx`) proves Esc bubbles out.
3. Desktop Contacts Esc with both the popover and the profile open: the first Esc closes only the popover. The logic is sound, but no test covers it.
4. Contacts under live WS traffic: re-render cost (M2). No perf test.
5. Android back on Contacts while suggestions show (M3).

## Recommended actions
1. M1: start ChatSearchScreen's panel engaged.
2. M2: replace `useConversations` in Contacts with a DM + activity selector, without subscriptions, ideally lazy.
3. M3: keep the list mounted under the suggestions, push a history entry or handle back, and close on Esc. Alternatively, record these as deliberate deviations in phase-03 and decide in phase 06.
4. L1/L2 as quick follow-ups. L6 at commit time.

Status: DONE_WITH_CONCERNS
Summary: Phase-03 acceptance (deny cases, no ids, ordering) is met and the focused tests pass (91/91). No critical issues. There are three medium findings: ChatSearchScreen suggestions now hinge on a programmatic focus; Contacts pays for the full useConversations pipeline (WS subscribe to every conversation plus a re-render per message); Contacts phone misses the DESIGN.md "back / keep scroll" rule.
Concerns: M1 needs an iOS device check. The unrelated e2e and PNG changes must stay out of the phase-03 commit.
