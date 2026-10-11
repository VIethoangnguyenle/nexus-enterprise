# Phase 04 — UI luồng in-line, panel Thread

Plan: `plans/261010-1559-chat-inline-threads/` · branch `claude/inspiring-cray-32y4n1` · not committed, git index untouched
(158 staged deletions intact; `TopicStream.*` removed from the working tree only).

## Result
Group and DM conversations now render as an in-line stream (two-sided bubbles, grouping, day and "Tin mới" dividers,
jump pill, older pages on scroll up, thread summary row under every message with replies), with ThreadPanel (follow
toggle, "Tin mới" inside the thread, opening marks it read) and ThreadsPanel (filters, pagination, open-from-list).
Built and checked at 390 first, then 768 / 1100 / 1440, light and dark, against the real stack with seeded data.

Gates (from `frontend/`): `npx vitest run` 1287 pass / 1 fail (the known `AssetsScreen` overview test, unrelated);
`npm run lint` clean; `npm run typecheck:diff` clean; `npm run build` ok.
Touch/overflow audit in a real browser at 390 and 768 (stream, thread list, thread): no horizontal overflow, no tap
target under 44px, no page errors.

## Files

Created (`frontend/src/`)
- `lib/stream-model.ts` (+ `.test.ts`): pure rules — grouping (same sender, ≤ 5 min, broken by day/unread/summary/system line),
  day labels, "Tin mới" placement (`unreadStartId`, `firstUnreadReplyId`), thread summary view + aria label, last-reply time
  format, RFC 3339 `before=` with full nanoseconds, `keepOlderPages`.
- `components/spaces/ConversationStream.tsx` (+ test): role="log" / aria-live polite; open at the end or at "Tin mới"
  (48px from top when the unread run is longer than a screen); jump pill (two states, never together); older pages with
  scroll anchor (`overflow-anchor: none` + manual); realtime wash for others only; "Thêm" menu; long-press sheet.
- `components/spaces/ThreadSummary.tsx`, `ThreadsPanel.tsx` (+ test), `MessageActionSheet.tsx` (bottom sheet, drag to close).
- `hooks/useThreads.ts` (follow/unfollow, mark thread read, threads infinite list, load older messages), `hooks/useLongPress.ts` (400ms).
- `components/chat/ThreadPanel.test.tsx`.

Modified
- `components/spaces/SpaceView.tsx`: ConversationStream, Thread header button + badge, `threads` panel, thread opened from the list
  has "Quay lại", read-at-end gating, phone header (Thêm người hidden < 768, search hidden < 1024 because the phone top bar
  carries it and clicks it), hint "Enter để gửi · Shift+Enter xuống dòng".
- `components/spaces/MessageBlock.tsx` (bubble layout, shared with the thread panel), `components/chat/ThreadPanel.tsx` (rewritten),
  `components/chat/HoverActionBar.tsx` ("Trả lời trong thread", Thêm gives its anchor, bar sits over the bubble corner for your own).
- `components/spaces/SidePanel.tsx`: `sheetBelow` ('lg' for the thread panels per DESIGN), `onBack`, `backOnSheet`, `bodyRef`;
  the lg sheet is `h-app` so the reply box rides above the keyboard. Other panels unchanged.
- `api/messaging.ts` (follow/unfollow/thread-read/listThreads, `before` URL-encoded, `ChannelUnread.thread_unread_count`),
  `hooks/useMessages.ts` (refetch keeps older pages), `hooks/useArrivals.ts` (+ `isHistory`), `lib/thread-cache.ts`
  (exports `patchRoot`, `applyThreadStateAnswer`), `test/chat-fixtures.ts` (thread fields, threads/follow/read endpoints).
- Docs: `docs/specs/chat-threads/spec.md` (new "Screens" requirements), `docs/specs/lark-messaging-layout/spec.md` (marked
  superseded; thread peek panel requirement kept), `docs/specs/README.md` (two rows), phase 04 and plan.md status.

Deleted: `components/spaces/TopicStream.tsx`, `TopicStream.test.tsx` (no import left).

## Tests added (TDD: stream-model, then components; 115 pass across the touched files)
- stream-model: grouping rules, day labels, divider placement, unread start with/without loaded read position, summary ("M mới" only when
  following, no ids in labels), RFC 3339 precision, `keepOlderPages`.
- ConversationStream (36): log semantics, own message without name/on the right, grouping, day separators, "Tin mới" position / frozen /
  gone after send, summary + "mới" rules + faces by name, hover actions, Thêm menu follow, long-press sheet (opens at 400ms, not on move
  or mouse, only existing actions, quick reaction), load-older trigger rules, scroll anchor, jump pills, at-end reporting, realtime wash
  (others yes, mine no, history/older pages no, summary wash only for followed threads).
- ThreadPanel (9): read on open after unread is known, divider position and persistence, no divider when not following, follow toggle
  with server calls + one aria-live announcement, reply posts `parent_message_id`, back button, error state.
- ThreadsPanel (9): list by name, filters hit the server, unread chip count, open, empty per filter (server sends no `messages` key), cursor
  = last reply time + root id, retry.
- SpaceView: log + hint, no identifier on any screen (stream, thread, thread list, members, files, tasks), badge, list → thread → back → Esc,
  channel read only at the end of the stream (positive and negative).
- useArrivals `isHistory`, thread-cache `applyThreadStateAnswer` (incl. not undoing a newer socket state).

## Found only against the real stack (fixed, covered by tests)
1. The server omits empty arrays: an empty thread list is `{"has_more":false}`. `getNextPageParam` crashed the whole conversation
   (error boundary) the moment the unread list became empty — i.e. right after opening a thread. `listThreads` now normalises.
2. A WebSocket resync refetches `messages` and replaced the cache with the newest page, deleting older pages the reader had loaded and
   moving the scroll. The messages query now keeps older pages (`keepOlderPages`).
3. A jump to the latest that starts at the top used to ask for an older page mid-flight and stop half way; loading now ignores downward scroll,
   and opening near the top fetches the page above once so "Tin mới" has context. Passing the divider during a jump no longer clears the
   "skipped" state, so "Nhảy tới tin chưa đọc" appears afterwards. Scroll anchor verified: pages 50 → 100 → 130 with the top line held to the pixel.

## Screenshots — `plans/261010-1559-chat-inline-threads/reports/phase-04-screens/`
`{390,768,1100,1440}-{light,dark}-1-stream.png`, `-2-thread.png`, `-3-threads.png` (24 files; thread opened from the list).
Extra at 390 light: `-4-sheet` (long-press), `-5-jump` (jump pill), `-6-older` (after pagination), `-7-after-jump` (long unread run, after jump).
Seeded data: channel "Đối soát giao dịch" (Đức/Vinh/Lan/Yến + me; a followed thread with 2 unread replies, an unfollowed thread, a
long wrapping message) and "Lịch sử dài" (130 messages). Compared against `spaces.html` §3–§4 and `chat-mobile.html` §2–§3 rendered side by side.

## Deviations from the mockup
- Phone bubble text is 14px, mockup says 15px (no 15px token; `MessageContent` owns the size).
- Read receipt: DESIGN says it follows the newest message *on screen*; implemented as "newest persisted message once the end of the stream is
  on screen" (nothing is marked while the reader is mid-run). Simpler and never marks unseen messages; progressive marking not done.
- Thread list: no FLIP reorder / realtime wash when a thread moves up (the list refetches and re-sorts); "M mới" pill leaves without the
  160ms fade; "Tin mới" divider fade is 220ms only for the stream.
- Thread panel has no day separators (mockup has none); replies and the root have no react/pin controls (as before this phase).
- Thread list "luồng chính cuộn tới tin gốc" on choosing a thread is not done.
- Desktop "Thêm" menu holds only "Theo dõi / Bỏ theo dõi thread" (Sao chép, Trích dẫn, Đánh dấu chưa đọc, Sửa, Xoá are later waves; DESIGN says
  unbuilt items are not shown). Long-press sheet likewise: quick reactions + Trả lời trong thread + Theo dõi + Ghim.
- ↑ to edit (wave 3), the "+" button, rich composer: untouched (phases 06/07/11).
- Header Thread badge counts threads from the `unread` list (max one page, 20) — the unread API gives replies, not threads.

## States the mockup does not cover (open questions)
1. Phone conversation: the shell keeps the "Nexus · workspace" top bar and the bottom tab bar inside a conversation; the mockup's chat screens
   show neither (own top bar, no tab bar). That is `_workspace.tsx`, outside this phase. Which is intended?
2. Phone: back arrow in a thread opened from the thread list returns to the list; in the mockup "close" leaves everything. Kept "back to list".
3. Phone header: no mockup for where "Thêm người" and search go. Hidden under md / lg respectively (members panel and the top-bar search
   still reach them).
4. Desktop "Thêm" button with a single menu item: keep, or hide the button until a second item exists?
5. A thread whose root was deleted / unreadable, and a thread list entry whose root is a task or a file only: rendered as plain text / "1 tệp".

## Concerns (not mine, worth knowing)
- Global CSS: the touch rule `:where(button, …) { position: relative }` in `index.css` is unlayered, so it beats Tailwind's layered `absolute`/`fixed`
  on any *button* on touch screens (the jump pill was full width until wrapped in an absolutely placed div). Same cause: `h1 { text-wrap: balance }`
  beats `truncate`, so headings never truncate. Left alone (index.css off limits); my components avoid the trap.
- Dev data left behind: demo users `*.demo@example.org` and channels "Đối soát giao dịch" / "Lịch sử dài" in the smoke account's workspace,
  inserted with SQL (a single account cannot produce two-sided chat). Delete the two channels and the four users to clean up.
- The sign-in code request is limited to 5 per 15 minutes per identifier; the refresh cookie is single-use. Screenshot scripts live in the
  scratchpad (not the repo) and reuse a saved cookie.
- The TanStack devtools icon sits bottom-left in the screenshots (dev only).

Status: DONE_WITH_CONCERNS
Summary: In-line stream, thread summary, ThreadPanel and ThreadsPanel are built, tested (TDD) and checked in a real browser at 390/768/1100/1440 light and dark; vitest (bar the known AssetsScreen failure), lint, typecheck:diff and build are green.
Concerns/Blockers: Mockup gaps listed above (phone shell bars, back behaviour, header actions); read receipt is end-of-stream rather than per visible message; thread-list live reorder animation not done; global CSS defects found (unlayered touch rule, heading `text-wrap`) are untouched.
