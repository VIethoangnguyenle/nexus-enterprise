# Wiring B: cards (W3), quote composer (W4), message star, chat attachments in FileViewer (W5 chat part)

Status: DONE_WITH_CONCERNS (concerns = items under "ASK OTHER AGENT" and the design calls below)

## Gates
- Full `npx vitest run`: 2113 pass, 1 fail = AssetsScreen (known). Other agent's files: no failures at the time of the run.
- `npm run typecheck:diff`: no file above baseline. `npm run build`: ok.
- eslint: my files clean. Remaining errors are in the other agent's files: `ConversationTitleMenu.tsx`, `NotificationLevelSheet.tsx`, `PhoneChatList.tsx:161`, `useFocusMessage.test.tsx:69`.
- TDD: tests written first for each behaviour (see lists below). No commits, git index untouched, no backend edits, nothing restarted.

## Files changed
New
- `stores/drafts.store.ts` (+test): per-conversation / per-thread draft (text + pending quote), `editing` flag, `quoteMessage`, `quoteFromMessage` (summary: "3 ảnh", file name, "2 tệp", "Bình chọn: …").
- `components/chat/QuoteStrip.tsx` (+test), `components/chat/use-quote-watch.ts` (+test), `components/chat/ComposerCreate.tsx`, `components/chat/MessageCard.tsx`.
- `lib/message-actions.ts` (+test): `canStar` / `canQuote`.
- Tests: `components/chat/ChatEditor.compose.test.tsx`, `components/spaces/{MessageMenu,MessageActionSheet,MessageBlock.cards,ConversationStream.cards}.test.tsx`.

Modified
- `components/chat/`: `ChatEditor.tsx` (+test), `ThreadPanel.tsx` (+test), `AttachmentGrid.tsx` (+test), `CreateMenuItems.tsx` (+test), `PollDialog.tsx` (+test), `TaskDialog.tsx` (+test), `DocDialog.tsx` (+test), `chat-extensions.ts` (+test: `mentionsFromHtml`).
- `components/spaces/`: `MessageBlock.tsx` (+test), `MessageMenu.tsx`, `MessageActionSheet.tsx`, `ConversationStream.tsx` (+test: 2 menu lists updated, PollCard mock).
- `hooks/`: `useMessages.ts` (+test), `usePolls.ts`.
- Outside my list, minimal and needed: `api/chat-cards.ts` (`parent_message_id` on `CreatePollInput`), `hooks/useMessageLifecycle.ts` (+test: edit sends `mentions`), `hooks/useMessageActions.ts` (+test: `onMarkedUnread` option).
Deleted: `components/chat/ImageViewer.tsx`, `ImageViewer.test.tsx`.
Not touched: `lib/chat-cache.ts`, `hooks/useAttachmentUploads.ts`, `components/files/**`, `components/approval/**`, routes, other stores, SpaceView.

## What was done
W3 cards
- Desktop: a separate "+" button "Tạo trong chat" (menu: Bình chọn, Công việc, Tài liệu mới, Đề nghị phê duyệt), next to Đính kèm, as `design/mockups/spaces.html` draws it (the wiring note said "second group of the same popover"; the mockup has two buttons, so I followed the mockup). Phone: one sheet "Tạo và đính kèm", attach row then the four rows (44px). Approval hidden in personal workspaces (both). Hidden while a message is being edited.
- Poll/Task/Doc dialogs take `parentMessageId`; thread panel passes the root: polls and tasks send `parent_message_id`, document/approval link messages are sent as replies (`useSendCardMessage`). Approval: `CreateRequestDialog.onCreated(request)` then a message `{content:'', linked_entity_type:'approval', linked_entity_id}`; failed post offers "Thử lại". No "Gửi vào" field.
- `MessageBlock` renders Poll/Task/Doc/Approval card by `linked_entity_type`, outside the bubble; the fallback text of poll/task messages ("📊 Poll: …") is not printed. No "Mở trong Công việc", no inline Duyệt/Từ chối.
- `ConversationStream`: poll/task messages arrive as `message_type: system`, which the stream drew as "đã tạo công việc" notes; now a card of its sender (other system lines stay notes).

W4 quote composer
- "Trích dẫn khi trả lời" in the "Thêm" menu and the long-press sheet (label from `shortcuts.html`); goes to the composer where the message lives (reply -> thread composer). Hidden while a phone composer edits.
- Strip as in `composer-quote.html`: region "Đang trích dẫn tin của …", ✕ "Bỏ trích dẫn", press = go to original, summary line for files/cards, deleted state ("Tin nhắn đã bị xoá", "Tin của bạn sẽ được gửi không kèm trích dẫn.", no link), fade + 4px rise (fade only for reduced motion), live announcements, focus moves to the editor on choose (not for a quote restored with a draft).
- Esc: attach/emoji/"+"/mentions first, then the quote, then (nothing left) bubbles to the panel. ↑ edit-last off while a quote is pending. Gửi needs text or files.
- Send: `quoted_message_id` added by `useSendMessage` / `useSendReply` from the drafts store (so SpaceView needed no change); card links never carry it; cleared on success; a 400 marks the quote deleted and keeps the text (no auto-resend); optimistic bubble shows the quote.
- Deleted while pending: `useQuoteWatch` reads the cached list/thread (never fetches) and marks the quote deleted.
- Drafts store per conversation id / `thread:<root>`: text and quote survive leaving and coming back and switching conversations; sent -> forgotten; failed send -> kept. Memory only (session).
- Phone edit: edit bar replaces the strip, note "Trích dẫn tin của X sẽ quay lại sau khi sửa xong.", draft and quote set aside and restored after Lưu/Huỷ.
- Edit sends `mentions` (full list from the mention spans of the edited html; `[]` when all removed; same list when none added); optimistic patch and rollback include `mentions`.
- Mark-unread rule: `ThreadPanel` holds off the auto-read after a successful "Đánh dấu chưa đọc" on a message of this thread until the panel is left. Interpretation: plan says the flag is cleared when the panel closes, so "reopening it right away" = the read that the unread update would otherwise trigger inside the same visit; the next opening reads it.

Message star
- "Gắn sao tin" / "Bỏ gắn sao tin" after Theo dõi thread in the menu, after the follow row in the sheet, using `useStarMessage` (not edited). Accent star (role img "Bạn đã gắn sao") after time/pin in `MessageBlock` (grouped lines too), not on deleted ones.

W5 chat part
- `AttachmentGrid.onOpen(index, files, opener)`: pictures first then files as one list; file cards open too (name area is a button "Mở tệp …", download stays separate). `MessageBlock` opens `FileViewer` (origin chat, sender and time in the header). No `onOpenInDrive` and no `parentFolderId`: message attachments carry no folder, so "Mở trong Tài liệu" never shows (needs a folder id in the API).
- ImageViewer tests' intent: open/Esc/focus return -> `MessageBlock.test` and `ThreadPanel.test` (Esc closes only the viewer); navigation/focus trap/download failure/no ids are already in `FileViewer.test`; one-list order and file open -> `AttachmentGrid.test`, `MessageBlock.test`.

## Tests added (negative cases included)
approval hidden in personal workspace (menu and sheet); poll counts not shown before voting (real PollCard inside MessageBlock); deleted-while-pending quote (strip state, send without it, 400 race keeps text); edit with no added mentions sends the same list, all removed sends `[]`, refused edit restores mentions; card link never carries the quote; quote not offered on deleted/temp/system-line messages or while a phone composer edits; Esc leaves the panel alone with nothing to close; failed mark-unread does not hold the read; no ids on screen (strip, menu, file card).

## ASK OTHER AGENT
1. `api/messaging.ts`: add `mentions?: string[]` to `EditMessageInput` (I use a local `EditMessageInput & { mentions: string[] }` in `useMessageLifecycle.ts`), and `parent_message_id?: string` to the `createTask` data type and `useCreateTask` input (local `TaskRequest` type in `TaskDialog.tsx`). After that the two local types can go.
2. Star toast "Xem": `useStarMessage({ onView })` is not passed from the menu/sheet (the Đã gắn sao route is theirs). Pass `onView` from the owner of the route, or tell me the path and I wire `useNavigate`.
3. `hooks/keys/index.ts`: `chatCards` keys still merged only by direct import (`hooks/keys/chat-cards.ts`).
4. `websocket.store.ts`: `pollVote` -> `invalidatePollOnVote` (their wave, noted in the wiring note).
5. Backend/API (not frontend): message attachments have no `parent_folder_id`, so "Mở trong Tài liệu" cannot appear from chat.

## Notes / unresolved
- Desktop "+" is a separate button (mockup) rather than a second group in the attach popover (wiring note); say if you want the single popover.
- Docs: `docs/specs/chat-cards` could note the client differences listed in the cards report (items 3, 4) and the thread placement; `docs/specs/chat-message-lifecycle` could note the quote draft and the mark-unread hold. Not edited (outside my files).
- No Playwright screenshots in this run (exec-03 step 5 and exec-quote step 6 are not in my scope list).
- The `ConversationStream.tsx` edit (one line + import) and its test edits sit in a file the other agent may also touch.

Status: DONE_WITH_CONCERNS
Summary: Cards, the quote composer with per-conversation drafts, message star and the FileViewer for chat attachments are wired and tested (full suite green except the known AssetsScreen, typecheck:diff and build clean). Concerns are the type widenings and the star "Xem" link that belong to the other agent, plus the desktop "+" menu following the mockup instead of the wiring note.
