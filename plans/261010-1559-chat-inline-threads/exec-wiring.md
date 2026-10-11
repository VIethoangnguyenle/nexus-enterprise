---
title: "Execution plan — wire the chat building blocks into the screens"
owner: Claude (decisions, review) · executor: one worker at a time per wave (shared files)
status: ready after chat 06 fixes + type scale are committed
---

# Wire everything built as standalone pieces

The pieces exist (tests pass) but nothing mounts them. Each piece's report has a WIRING NOTE — read it before the
wave that uses it:
- shortcuts: `plans/reports/fullstack-developer-261011-1140-shortcuts.md` + `exec-shortcuts-ui.md` steps 2, 4, 5
- notification levels: `plans/reports/fullstack-developer-261011-1140-notification-levels.md` + `exec-09-notification-levels-ui.md` steps 3, 4
- cards: `plans/reports/fullstack-developer-261011-1140-chat-cards.md` + `exec-11-cards-ui.md`
- file viewer: `plans/reports/fullstack-developer-261011-1140-file-viewer.md` + `plans/261011-1035-file-preview/exec-03-file-viewer.md` steps 4, 5
- cards in a thread (backend): `plans/reports/fullstack-developer-261011-1150-cards-in-thread.md`

## Decisions (fixed)
1. **Focus target in a conversation.** `routes/_workspace/channels.$channelId.tsx` search: `thread` (root id) and
   optional `message` (validated). SpaceView takes `initialThreadId` / `focusMessageId`: opens the thread panel and/or
   scrolls the message into the centre and flashes it 2.4 s (main stream or thread panel). Used by push links, the
   Trang chủ "Thread có tin mới" rows, Được nhắc tới and Đã gắn sao.
2. **`starred`** is added as an optional field to `Message`, `Channel`, `ChannelUnread` in `api/messaging.ts`; drop the
   shortcuts code's local patch types. Register `shortcutKeys` and the other new key files in `hooks/keys/index.ts`.
3. **Realtime:** `stores/websocket.store.ts` calls the exported helpers: pollVote → poll invalidation; threadState /
   threadReply → unread threads; a new message that mentions me → `mentionsAll()` + `mentionsUnreadAll()`. A test per
   frame.
4. **Approval card creation:** `components/approval` CreateRequestDialog's `onCreated` passes the created request
   (id + title); the chat opens that dialog, then sends a message `{content:'', linked_entity_type:'approval',
   linked_entity_id}`. No "Gửi vào" field. Hidden in personal workspaces.
5. **Cards in a thread:** the thread panel's "+" passes the root id: polls/tasks via the new `parent_message_id`
   (see the backend report), doc/approval link messages are sent as replies. Main composer passes nothing.
6. **"Mở trong Công việc"**: no tasks route exists → not shown. Inline Duyệt/Từ chối on the approval card: not in
   this round (card opens Phê duyệt instead).
7. **File viewer everywhere:** chat attachments (images AND files of the same message, as one list) open FileViewer;
   delete `components/chat/ImageViewer.tsx` and move its tests' intent; Tệp tab (`SpaceFiles.tsx`), Tài liệu
   (`components/drive/*`, DriveDetailPanel "Xem trước"), `patterns/{FilePreviewCard,ImagePreviewCard}.tsx`.
   "Mở trong Tài liệu" only with a parent folder id.
8. **Shortcuts animations left out by the first pass** (unstar collapse 220 ms, new mention grows in, @you-only wash
   in the stream): build them per `design/mockups/shortcuts.html`, honouring `prefers-reduced-motion`.

## Waves (in order; each: tests first, then gates; one executor per wave)
- **W1 focus + shortcuts:** decision 1, 2, the shortcuts routes (desktop main pane, phone full screen with Quay lại —
  follow how Trang chủ is routed), navigator rows + phone tiles + "Đã gắn sao" section, star actions in "Thêm" menu /
  long-press sheet / conversation header / row ⋯, decision 8, realtime mentions.
- **W2 notification levels:** exec-09 steps 3–4, PhoneChatList mount of UnreadThreadsSection, realtime threads,
  totals by level.
- **W3 cards:** composer "+" second group (desktop popover + phone sheet), MessageBlock renders Poll/Task/Doc/Approval
  cards, decisions 4–6, pollVote realtime.
- **W4 quote composer:** `exec-quote-composer.md` in full.
- **W5 file viewer:** decision 7; then exec-03 step 5 Playwright screens.

## Gates (every wave)
Touched tests, full `npx vitest run` (known not-yours: AssetsScreen), `npx eslint` touched files,
`npm run typecheck:diff` (no new errors anywhere), `npm run build`.

## Rules
PROGRESS LOG after EVERY step: `plans/reports/progress-wiring-<wave>.log` (`<time> | step | result`); read first when
resuming. No commits, no git index, no backend edits. NEEDS_CONTEXT instead of inventing UI.
Report per wave: `plans/reports/<executor>-261011-wiring-<wave>.md`.
