# Shortcuts UI, steps 1 and 3 (data layer + two views)

Status: DONE_WITH_CONCERNS. New files only; no existing file, git index or backend touched.

## Files (all under frontend/src)
- api/shortcuts.ts: `shortcutsApi` (star/unstar conversation + message, listStarred, listMentions, mentionsUnreadCount), `FeedMessage`, `FeedPage`, `FeedCursor`.
- hooks/keys/shortcuts.ts: `shortcutKeys` (NOT yet in hooks/keys/index.ts; hooks import it directly).
- lib/shortcut-feed.ts (+test): `feedCursor` (RFC3339 nanos via timestampToRfc3339), `groupByDay`, `cardTime`, `messageSegments` (flat runs, `span[data-mention=<userId>]` -> self/other), `openTarget`.
- hooks/useShortcuts.ts (+test): `useMentionsFeed`, `useStarredFeed` (25/page), `useMentionsUnreadCount`, `useThreadRoots`, `useOpenMention`, `useStarMessage`, `useStarConversation`; pure patches `setMessageStarred`, `setUnreadListStarred`, `setChannelRecordStarred`, `dropFromFeed`, `markFeedRead`.
- components/spaces/FeedParts.tsx (shared card, skeleton, footer, error/empty, BackBar), MentionsView.tsx, StarredView.tsx (+ tests each).

## Gates
- vitest: 5 new test files, 46 tests pass (+ api dir 122 total in the run). 
- eslint on all 11 files: clean.
- tsc: zero errors in my files. `typecheck:diff` reports new errors only in other files (chat/*, MessageBlock.test.tsx: other agents).

## Behaviour notes
- Star toggles are plain async functions (not useMutation) so toasts can call their own undo/retry. Optimistic: stream list, thread, starred feeds, unread list (`starred`), channel record. Rollback + `toast.error("Không gắn sao được. Kiểm tra kết nối rồi thử lại.")` + Thử lại.
- Toasts: star message "Đã gắn sao tin" + Xem (only if `onView` given); unstar "Đã bỏ gắn sao tin" + Hoàn tác (restores the card to its old feed position, then PUTs; no refetch so position holds); conversation "Đã gắn sao/Đã bỏ gắn sao “name”" + Hoàn tác. Undo/retry are quiet (no second toast).
- Viewing a list marks nothing read. Clicking a mention card calls `useOpenMention` (dot off, badge -1, no request) then `onOpen`.
- Reply context line needs the root: feed items carry only `parent_message_id`, so `useThreadRoots` fetches `GET /messages/{root}/thread` (shared `keys.messaging.thread` cache). Until loaded, line reads "Trả lời trong thread".
- Conversation name/icon per card comes from the caller's `conversationOf(channelId)`; the feed payload has no channel name.

## WIRING NOTE (steps 2, 4, 5)
Props:
- `MentionsView({ workspaceId, conversationOf, onOpen(item: FeedMessage), onHome, onBack?, hrefOf?, phone? })`
- `StarredView({ workspaceId, starredConversations: Conversation[], conversationOf, onOpen(item), onOpenConversation(c), onHome, onBack?, hrefOf?, phone? })`
- `workspaceId` = `useActiveWorkspace().workspaceId`. `conversationOf` = lookup over `useChatNavigation(ws).conversations`. `phone` = `useBottomBarLayout()`; pass `onBack` only on phone (renders Quay lại, hidden at lg).
- `onHome`: navigate to Trang chủ (desktop) / Tin nhắn tab (phone). `onOpenConversation`: navigate `/channels/$channelId`.
- `onOpen(item)`: `const t = openTarget(item)` from lib/shortcut-feed ({channelId, messageId, rootId?}). Navigate to channel t.channelId focusing t.messageId (flash 2.4 s); if t.rootId, also open the thread panel at t.rootId flashing t.messageId. This is step 4's search param (`thread` + `message`).
- Route: both views fill the main pane; suggest `/spaces/mentions` and `/spaces/starred` (follow Trang chủ); phone full screen with onBack = history back.
- Navigator (step 2): pill = `useMentionsUnreadCount(ws).data` (99+ via `formatCount`). Starred section rows = conversations whose unread-list item has `starred` (ChannelUnread type lacks the field: add `starred?: boolean` to ChannelUnread, Channel and Message in api/messaging.ts; my hooks already patch it). Row menu "Gắn sao/Bỏ gắn sao" -> `const star = useStarConversation(); star({channelId, name, starred})`.
- Step 5: message menu/sheet -> `const star = useStarMessage({ onView: () => navigate(starred route) }); star({ message, starred: !message.starred })`.
- Also needed: register `shortcutKeys` in hooks/keys/index.ts (`shortcuts: shortcutKeys`) if the "no literal keys" convention requires; realtime for a new mention should invalidate `shortcutKeys.mentionsAll()` and `mentionsUnreadAll()`; the badge refetches on mount/focus only otherwise.
- Not done / gaps: the 220 ms collapse animation on unstar and the realtime "new mention grows in with person wash" (need realtime hook, step outside my files); the `@you` wash inside the stream itself (index.css `.message-html [data-mention]` already washes every mention, not only the reader's).

Status: DONE_WITH_CONCERNS
Data layer, hooks and both views are built test-first and pass vitest, eslint and tsc. Concerns: reply root lines cost one thread fetch per distinct root, and `starred` is absent from the shared Message/Channel/ChannelUnread types until api/messaging.ts is edited (my code patches it via local types).
