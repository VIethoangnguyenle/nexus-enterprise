# Code review: chat phase 04 (in-line thread stream, mobile-first)

Plan: `plans/261010-1559-chat-inline-threads/phase-04-inline-stream-ui.md` · implementer report
`reports/fullstack-developer-261011-0800-phase-04.md` · read-only review of the uncommitted tree, nothing edited.

## Scope
- Files: `frontend/src/components/spaces/{ConversationStream,ThreadSummary,ThreadsPanel,MessageActionSheet,MessageBlock,SidePanel,SpaceView}.tsx`,
  `components/chat/{HoverActionBar,ThreadPanel}.tsx`, `hooks/{useThreads,useLongPress,useArrivals,useMessages}.ts`,
  `lib/{stream-model,thread-cache}.ts`, `api/messaging.ts`, `test/chat-fixtures.ts`, `TopicStream.*` deleted;
  `docs/specs/chat-threads/spec.md`, `docs/specs/lark-messaging-layout/spec.md`.
- About 670 lines added and 540 removed in tracked files, plus about 1,300 lines in new files.
- Gates, run here: `npx vitest run` 1288 pass / 1 fail (the known `AssetsScreen.test.tsx:103`); `npm run lint` clean;
  `npm run typecheck:diff` reports no file with more errors than the baseline; `npm run build` ok (the chunk-size warning was already there).
- Backend contracts checked against the code: `GetThread` sets `following` and `thread_unread_count` on the root
  (`domain/messages.go:174`). `unread_count` counts root messages only (`store/reactions_pins_receipts.go:278`).
  `before_id` is accepted (`rest/threads.go:83`).

## Overall
The structure is sound, the tests are mostly behavioural, and no identifier reaches the screen. Messages are only
authorised on the server. One real data-correctness defect is merged in with a test that hides it (keepOlderPages). There
is one mobile regression: text can no longer be copied on a phone. There are also two smaller timing and accessibility
problems. None of them is a trust-boundary issue.

## Critical
None.

## High

### H1. A refetch after a long gap silently drops messages and can claim there is no more history
`frontend/src/lib/stream-model.ts:187-200`, wired in `hooks/useMessages.ts:10-18`.
`keepOlderPages` adds every previous message that is older than the oldest message in the fresh page. It assumes the
fresh page overlaps the previous cache. When more than one page (50) of messages has arrived since the last fetch, the
two do not overlap:
1. A reader has channel X open (or cached), then the laptop sleeps or the tab is hidden past `staleTime` (30 s).
2. Sixty messages arrive. On focus or a WS resync the newest page is refetched: messages n1..n50.
3. The previous cache (messages p1..p50) is all older than n50, so it is appended: `[n1..n50, p1..p50]`.
4. The 10 messages between p1 and n50 are never shown. `has_more` is taken from `previous.has_more`, so with a fully
   scrolled history it becomes `false` and the gap can never be filled. `loadOlder` pages from the oldest item, below
   the gap.

The test in `stream-model.test.ts:209-214` only covers overlapping pages. The no-overlap case has no test and is wrong.
This is a regression: before this change a refetch replaced the cache, which lost scroll position but never lost
messages.
Fix: keep the older pages only when the fresh page overlaps the previous cache (some fresh id is in `previous`, or
`fresh.has_more === false`); otherwise return `fresh`. Add a test for the no-overlap case. Longer term, the messages
cache rebuilds `useInfiniteQuery` by hand (`useLoadOlderMessages` writes pages with `setQueryData`, and the queryFn reads its own
cache). An infinite query refetches every page in order and would remove this whole class of bug. That is a larger
change because every chat-cache patcher assumes the `{messages, has_more}` shape.

### H2. Phones can no longer copy message text
`components/spaces/MessageBlock.tsx:95` (`pointer-coarse:select-none`) and `hooks/useLongPress.ts:50-52`
(`onContextMenu` preventDefault for touch).
On a touch screen a message cannot be selected, and the native long-press menu is suppressed. The new action sheet
(`MessageActionSheet.tsx:105-123`) has no "Sao chép" item, which DESIGN.md:451 lists, because the implementer deferred
it to a later wave. Net effect: a phone user cannot copy any chat text (an account number, a link to paste elsewhere).
This was possible before the phase. DESIGN's rule that unbuilt items are not shown is fine for menus, but here it removes an
existing capability.
Fix: add "Sao chép" (`navigator.clipboard.writeText(previewText(m.content))` and a toast) to the sheet in this phase.
Alternatively, drop `select-none` until the copy action ships.

## Medium

### M1. The channel can be marked read before the stream has decided where the reader is
`components/spaces/SpaceView.tsx:102` (`useState(true)` for `atEnd`) with `:121-126`.
`atEnd` starts `true`. The stream only reports its real position after `open.resolved`, and that waits for
`useUnreadCounts`. On a cold deep link, if `/channels/unread` (a batch policy check) takes more than 1 s longer than
`/messages`, the 1 s timer fires `markRead(lastPersisted)` while the stream still shows a skeleton or is unplaced. The
unread refetch then resolves with the new read position, so the "Tin mới" divider never appears. A long unread run is
marked read without being seen, which defeats the gating this phase added.
Fix: start `atEnd` as `false`, or only arm the timer after the first `onAtEndChange` report (`reportedEnd` is reset on
open, so the report always comes).

### M2. Loading older pages makes screen readers read out up to 50 old messages
`components/spaces/ConversationStream.tsx:284-289`: `role="log"`, `aria-live="polite"`, `aria-relevant="additions"`
on the scroller that older pages are inserted into. NVDA, JAWS and VoiceOver announce additions anywhere in the
region, so each scroll-up page queues every prepended message, and each one is named by its whole text
(`aria-labelledby …-text`).
Fix: keep `role="log"` but set `aria-busy="true"` while `loadingOlder`. Better, move the live announcement to a separate
visually-hidden `role="status"` that only announces messages from others (the `fresh` map already has this). The phase
requirement ("luồng là role=log có aria-live lịch sự") is met in letter but not in intent.

### M3. Every message re-renders and re-sanitises its HTML on each typing or presence event, and the cost grows with the pages loaded
`SpaceView.tsx:89-90` subscribes to `typingUsers[channelId]` and the whole `onlineUsers` map. Each change re-renders
`ConversationStream`. Its `row()` (`:235-279`) renders a non-memoised `MessageBlock` for every loaded message, and each
one runs `sanitizeHtml(highlightMentions(content))` (`chat/MessageContent.tsx:20`) and `useLongPress`. With pagination
the list is now unbounded: the report's own 130-message case means 130 DOMPurify passes per typing tick or presence
change, plus a new `unread={{…}}` object every render. There is no virtualisation. TopicStream had the same shape, but
it did not page.
Fix: `React.memo(MessageBlock)` with stable callbacks (`useCallback` in SpaceView, or pass ids), and memoise
`MessageContent` on `(content, contentFormat)`. Narrow the `onlineUsers` selector to the members of this channel.
Virtualisation can wait until the memo is measured.

### M4. Messages from older pages pile up across visits
`hooks/useMessages.ts:10-18` together with `gcTime` of 5 minutes. When a reader returns to a channel within 5 minutes, every older page they
loaded comes back and is kept on refetch, so the stream renders all of them on open, with the cost described in M3.
Consider trimming the older pages when the stream unmounts (keep the newest page). Alternatively, if H1 is fixed by
moving to `useInfiniteQuery`, use `maxPages`.

### M5. Phone conversation keeps the app's top bar and the bottom tab bar (open question 1 in the implementer report)
Screenshot `phase-04-screens/390-light-1-stream.png`: the "Nexus · workspace" bar, the conversation header, the tab
row and the bottom tab bar together take about 250 of 844 px. DESIGN.md:446 and `chat-mobile.html` give the conversation its own top
bar and no tab bar. The cause is `_workspace.tsx`, which is outside this phase's files, but the phase is labelled
mobile-first and its first criterion is "Ảnh chụp khớp mockup ở 390px". This is a real deviation that needs a decision
from the user before phase 05 verification, not a silent pass. The DESIGN group icon is 28 px; the screenshot shows
about 40 px.

### M6. The back arrow on the thread sheet is labelled "Đóng thread" but goes back to the list
`components/spaces/SidePanel.tsx:89-92`: `aria-label={\`Đóng ${closeLabel}\`}` with `onClick={onBack ?? onClose}`. When
a thread was opened from the list, a screen-reader user hears "Đóng thread" and lands in the thread list. Use
"Quay lại danh sách Thread" when `onBack` is set, as the wide-screen branch at `:95` does.

## Low

- **L1.** The `viewChannel` registration (`SpaceView.tsx:110-111`) is correct: it returns its own cleanup, SpaceView is
  keyed by `channelId` (`routes/_workspace/channels.$channelId.tsx:9`), and the store re-announces after subscribe. It
  stays active while the Tệp or Công việc tab is shown, however, so push for that conversation is held back while its
  messages are not on screen. DESIGN.md:463 says "đang mở đúng cuộc trò chuyện", which is arguable. Pass
  `tab === 'chat'` into the effect if the intent is "messages visible".
- **L2.** `unreadStartId` falls back to placing the divider by count (`stream-model.ts:98`). With more than 50 unread
  messages it lands on the first message of the loaded page, and because it is frozen for the visit (`ConversationStream.tsx:84-97`)
  it stays wrong after older pages load. Acceptable, but say so in the spec's "Screens" section.
- **L3.** `ThreadPanel.tsx:104-108` scrolls to every new reply even when the reader has scrolled up in a long thread.
  Follow only when the panel body is near its end, as the main stream does.
- **L4.** `ThreadsPanel.tsx:57-79` uses `role="radiogroup"` with `role="radio"` buttons but has no arrow-key or
  roving-tabindex handling, so every chip is its own Tab stop. Either add arrow keys or use `aria-pressed` buttons.
- **L5.** Older pages kept by `keepOlderPages` are never refreshed. An edit or delete of an old message made while
  disconnected stays stale until the cache is dropped.
- **L6.** `SpaceView.tsx` sends an extra `GET /threads?filter=unread` on every conversation open for the badge, and
  repeats it on each thread-state event through `threadsListOf` invalidation. The badge is capped at "9+", so the
  20-item page limit the implementer flagged is invisible. The fetch itself is the cost.
- **L7.** The implementer report says `docs/specs/README.md` was modified. Its two rows are already in commit `c2d4e3c`,
  and there is no diff in this tree. That is harmless, but the "Files" list is inaccurate.
- **L8.** Dev data was left in the shared dev DB: the demo users `*.demo@example.org` and the channels "Đối soát giao
  dịch" and "Lịch sử dài", inserted by SQL. Clean them up before anyone takes screenshots against that account again.

## Implementer deviations: verdict
| Deviation | Verdict |
|---|---|
| Phone bubble text 14px vs DESIGN 15px | Acceptable for now; track it (needs a token or a `MessageContent` size prop). |
| Read receipt at end of stream rather than per visible message | Acceptable and conservative (it never marks unseen messages), but see M1, which breaks the guarantee. |
| No FLIP reorder or fade in the thread list | Acceptable (polish). |
| No day separators or react/pin controls in the thread panel | Acceptable; the mockup has none. |
| Choosing a thread does not scroll the main stream to the root | Acceptable as a follow-up; the root is shown at the top of the panel. Add it to the spec's open items. |
| "Thêm" menu and sheet hold only the built actions | Acceptable for the desktop menu. **Not acceptable for the sheet without "Sao chép"** (H2). |
| Badge counts the first page of unread threads | Acceptable (the "9+" cap hides the limit); see L6. |
| Phone shell bars (open question 1) | **Needs a user decision** (M5). |
| Back from list-opened thread returns to list (open question 2) | Reasonable; matches the desktop "Quay lại". Fix the label (M6). |
| Header actions hidden under md/lg (open question 3) | Acceptable; members and search are still reachable. |
| Single-item "Thêm" button (open question 4) | Keep it. It is the only desktop path to follow or unfollow a thread without opening it. |

## Checklist results
- **Concurrency:** `useLoadOlderMessages` has a `busy` ref against overlapping loads. It reads the cache after its
  await, so it is safe against concurrent refetches. `applyState` orders by `as_of` (`thread-cache.ts:54-56`).
  `useFollowThread` does not `cancelQueries` before its optimistic patch, so a refetch that lands at that moment can
  briefly undo the toggle. The server's answer then settles it. Low.
- **Error boundaries:** `listThreads` normalises omitted arrays (the crash was found and fixed). Follow errors show a toast
  and roll back. A failed older-page load shows a retry pill. A thread-read failure is silent by design.
- **API contracts:** see Scope. `ThreadState.as_of` is parsed through `toMillis`, which accepts both the proto
  timestamp object and an ISO string.
- **Auth:** UI only. Every new endpoint is authorised on the server (`authorizeMessage` / `authorizeChannel`). No new
  trust boundary on the client.
- **Data leaks / identifiers:** no UUID is rendered. Names come from `displayName`, which falls back to the username
  or `UNKNOWN_PERSON`, never an id. Ids appear only in DOM `id` and `aria-labelledby` attributes and hue keys.
  `SpaceView.test` asserts no identifier on any panel.
- **State ownership:** server state stays in TanStack Query. `useLoadOlderMessages` keeps only request UI state
  (`loading` and `failed`) in local state, which is fine. Zustand is used only for the socket store.
- **Touch and inputs:** the 36 and 40 px controls (summary row, chips, jump pill, Follow) reach 44 px through the global
  `@media (pointer: coarse)` `::after` hit-area rule (`index.css:697-707`). Inputs are 16 px under the same media query.
- **Plan success criteria:** the screenshots exist (28 files). The gates are green apart from the known failure. No
  `TopicStream` import is left. Criterion 1 is ticked while M5 is still an open mockup deviation at 390 px.

## Recommended actions (in order)
1. H1: guard `keepOlderPages` against a fresh page with no overlap, and add the missing test.
2. H2: add "Sao chép" to the long-press sheet, or drop `pointer-coarse:select-none`.
3. M1: do not arm mark-read until the stream has reported its position.
4. M2: stop live-announcing older pages.
5. M3/M4: memoise `MessageBlock` and `MessageContent`, narrow the presence selector, and trim older pages on unmount.
6. M6: fix the back-arrow label. M5: get the user's decision on the phone shell before phase 05.

## Unresolved questions
- M5: should a phone conversation hide the app top bar and the tab bar, as the mockup does? That requires a change in `_workspace.tsx`.
- Should the "Tin mới" placement for more than 50 unread messages (L2) be fixed now, or recorded as a limitation in the spec?

Status: DONE_WITH_CONCERNS
Summary: Phase 04 builds clean and meets most criteria, with no identifiers on screen and server state kept in TanStack Query. Two high-priority defects should be fixed before commit: `keepOlderPages` silently drops messages (and can turn off "load older") when a refetch does not overlap the cache, and phones lost the ability to copy message text. Mark-read timing, live-region noise and re-render cost are medium-priority follow-ups.
