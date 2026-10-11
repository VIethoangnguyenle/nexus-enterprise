# Chat phases 08 + 07 — frontend

Status: DONE_WITH_CONCERNS (one design gap, see NEEDS_CONTEXT below). No commits, git index untouched.

## Delivered
Phase 08 (lifecycle)
- API `editMessage / deleteMessage / markMessageUnread`, `quoted_message_id` on send; types `edited_at`, `deleted`, `quoted`, `message_type`, `Channel.ngac_oa_id`.
- WS `message_edited` / `message_deleted` -> cache patches in `lib/chat-cache.ts` (`applyMessageEdited`, `applyMessageDeleted`, `patchMessage`), wired in `websocket.store.ts`. Edit keeps reactions/thread state, no new-message preview; delete: placeholder for root with thread, removal otherwise, reply leaves thread + root count from server, quotes of it become "deleted", idempotent.
- `lib/message-model.ts` (abilities, delete copy, quote summary, text/HTML helpers), `hooks/useMessageLifecycle.ts` (optimistic edit + rollback, delete, mark unread), `hooks/useMessageActions.ts` (editing/deleting state, spoken errors, empty save -> asks delete, refused save reopens editor with typed text), `hooks/useCanManageChannel.ts` (policy `manage` on channel content OA).
- UI: "đã sửa" label, "Tin nhắn đã bị xoá" placeholder (keeps thread summary), `QuoteBlock` (click scrolls when loaded), in-place `MessageEditBox` (>=768), composer edit mode "Đang sửa tin" + Huỷ + Lưu (phones), ↑ edits last own message, `DeleteMessageDialog` (`alertdialog`, 420, Huỷ focused), hover pencil, `MessageMenu` ("Thêm"), long-press sheet items (Sửa tin / Đánh dấu chưa đọc / divider / Xoá tin danger), manager-deletion log as quiet system line. Same actions in `ThreadPanel` replies.
Phase 07 (rich text)
- `chat-extensions.ts` (StarterKit v3 already has Underline + Link: no new deps; quick syntax `*b*` `_i_` `~s~`, Ctrl+Shift+X, links limited to http/https/mailto, no trailing empty paragraph), `FormattingToolbar` ("A" toggle remembered per user, `aria-pressed` follows selection, roving arrows, tooltips with Ctrl/⌘, 44px on phones, horizontal scroll), `LinkDialog`, Ctrl/Cmd+K, Enter = new line in list/code block (Ctrl/Cmd+Enter sends), mention as `span[data-mention]` node + ids sent in `mentions`, `MessageContent` no longer regex-highlights `@word` in HTML, design CSS for message/editor markup. Client sanitiser (DOMPurify) kept and re-tested.

## Files
New: `lib/message-model.ts(+test)`, `hooks/{useMessageLifecycle,useMessageActions,useCanManageChannel,useMessages}.test/ts`, `components/spaces/{DeleteMessageDialog,MessageEditBox,MessageMenu,QuoteBlock,copy-message}`, `components/chat/{FormattingToolbar,LinkDialog,chat-extensions}` (+tests, `ChatEditor.test`).
Modified: `api/messaging.ts`, `lib/{chat-cache,stream-model}`, `stores/websocket.store.ts`, `hooks/useMessages.ts`, `components/chat/{ChatEditor,HoverActionBar,MessageContent,ThreadPanel}`, `components/spaces/{MessageBlock,MessageActionSheet,ConversationStream,SpaceView}`, `components/composites/Dialog.tsx` (additive `role`/`descriptionId`/`width`), `index.css` (message/editor markup), `test/setup.ts` (jsdom range stubs).
Shared-file touches kept minimal: `SpaceView.tsx` only (lifecycle hook, send mentions, composer `edit`/`onEditLast`, stream `lifecycle`); not the search-button line or anything the coordinator owns. Untouched: `_workspace.tsx`, `components/app/**`, `api/client.ts`, `vite.config.js`, `package.json`, navigator/sections files, MobileNav/MobileTopBar.
Docs: `docs/specs/chat-message-lifecycle` and `chat-rich-text` (client requirements added); phase-07/08 status lines; plan.md rows.

## Gates
- `npx vitest run`: 1535 pass, 1 fail = known AssetsScreen overview.
- `npm run lint`: clean. `npm run typecheck:diff`: no file increased. `npm run build`: ok.
- TDD: tests written first for each behaviour (api, cache patches, WS, hooks, model, dialog, edit box, stream, toolbar, link dialog, composer, extensions, MessageContent).

## Browser (Vite :5173 up, real backend, smoke account)
Screens in `plans/261010-1559-chat-inline-threads/reports/phase-07-08-screens/` — 1440 light+dark: toolbar open, rich text sent (bold/italic/strike/code/list), hover bar with pencil, in-place edit, "đã sửa", Thêm menu, delete dialog; 390 light+dark: stream (no horizontal overflow, 390/390), toolbar scrolling, long-press sheet of own message (Sửa tin, divider, Xoá tin). No page/console errors.
Not browser-verified: phone composer edit mode (bar "Đang sửa tin"), mark-unread, quote block, deleted placeholder, manager deletion line, ↑ edit — covered by jsdom tests only. The OTP limit (5/15 min) was hit before the phone-edit step and no session had been saved to disk.
Fixed after seeing screens: "A" button had no pressed look; in-place editor at the foot of the stream sat behind the composer (now scrolls into view).

## NEEDS_CONTEXT (exact gap)
"Trích dẫn khi trả lời" action is NOT built. DESIGN.md and both mockups define the quote block inside a bubble and list the menu item, but define no composer state for a pending quote (chip/bar above the input: label, who/what is shown, remove control, behaviour on mobile and in thread panel). Per brief I did not design it; the item is hidden (DESIGN: unbuilt items are not listed). Rendering of quotes sent by other clients/API works; `quoted_message_id` is already in the send type.

## Notes / unresolved
- Mark-unread leaves the open stream's "Tin mới" divider where the visit began (it is fixed per visit); badges/counts refresh. Say if the divider should jump.
- The manager-deletion line has no mockup; it reuses the quiet task-line style.
- `useCanManageChannel` costs one `GET /channels/:id` + one batch-access per opened channel (cached).
- Phone message edit lives in the composer; if the thread panel and main composer both exist, each edits its own messages.
- I used a few Python replace scripts and one `sed -i` on frontend files (Serena tools are not available to this agent); no generated files touched.

Status: DONE_WITH_CONCERNS
Summary: Phase 08 UI (edit, delete, mark unread, quote rendering, WS patches) and phase 07 rich-text composer are built test-first with all gates green and desktop/phone screens captured; the "Trích dẫn khi trả lời" action is withheld because no design defines the composer's pending-quote state.
