# Notification levels UI — steps 1, 2, 5

## Files
New: `frontend/src/api/notification-levels.ts` (type `NotificationLevel`, `DEFAULT_LEVEL`, `notificationLevelApi.{getNotificationLevel,setNotificationLevel,unreadThreads}`),
`hooks/keys/notification-levels.ts` (`level(chId)`, `unreadThreads()`), `hooks/useNotificationLevel.ts(+.test.tsx)`,
`hooks/useUnreadThreads.ts`, `lib/unread-threads-cache.ts(+.test.ts)`, `components/spaces/UnreadThreadsSection.tsx(+.test.tsx)`.
Edited: `api/messaging.ts` (+4 lines: optional `notification_level`, `mention_count` on `ChannelUnread`), `hooks/keys/index.ts` (+2: `keys.notificationLevels`),
`components/spaces/HomeView.tsx` (+12: mounts section, desktop only), `HomeView.test.tsx` (+1 test).
Keys are reached as `keys.notificationLevels.level(id)` / `.unreadThreads()` (not `keys.messaging.*`).

## Tests / gates
- vitest: useNotificationLevel 8, unread-threads-cache 5, UnreadThreadsSection 9, HomeView* (incl. new mount+navigate test) all green (30 total in the 5 files).
- eslint on all my files: clean. `typecheck:diff`: no errors in my files (existing errors elsewhere: chat/AttachmentGrid, files/SheetBody, useShortcuts.test, MessageBlock.test ...).
- Not run: full vitest, build.

## Behaviour
- `useNotificationLevel(channelId, name)` -> `{ level, isLoading, isSaving, setLevel(level) }`. Optimistic: patches level query + that channel's item in `unreadCounts` (off => unread_count 0, thread_unread_count dropped, mention_count kept). Rollback on error + error toast (8s, "Thử lại"). Success toast with "Hoàn tác" (restores prev, quiet = no second toast). Toast wording from mockup (`levelToast` exported). onSettled invalidates unreadCounts + unreadThreads. No-op if level unchanged. `isSaving` for the mockup's "Đang lưu…" after 300ms (consumer times it).
- `UnreadThreadsSection({conversations, people, onOpenThread(channelId, rootId), max=3})`: hidden when empty/error; skips roots whose channel is not in `conversations`; role=list/listitem, row = button with aria-label; "Xem thêm N thread" / "Ẩn bớt"; collapse "Thu gọn"/"Mở rộng", localStorage key `ngac-home-threads-collapsed` in try/catch; 44px (`max-lg:min-h-11`) on toggles.
- HomeView desktop: `navigate({to:'/channels/$channelId', params, search:{thread: rootId}})`. Route already validates `?thread=`, but SpaceView does not consume it yet (route file comment says so) — tapping lands in the group without opening the panel until SpaceView reads `search.thread`.

## WIRING NOTE
- Steps 3-4: call `useNotificationLevel(channel.id, channel.title)` for the header menu (radio group: all/main/off; DM: main "Bật"/off only; do not offer `all` on a DM) and for the row menu ("Tắt thông báo" -> setLevel('off'), "Bật thông báo" -> setLevel('main')). Row/total data: `useUnreadCounts()` items now have `notification_level` and `mention_count`; `useConversations` currently maps only `unread_count`, so extend it there (muted adds only `mention_count`). Row menu needs a name per row; one hook instance per row is fine (query is cached per channel) but prefer reading `notification_level` from unread counts to avoid N GETs.
- PhoneChatList mount: `<UnreadThreadsSection conversations={nav.conversations} people={people} max={2} onOpenThread={...same navigate as HomeView...} />` at the top of the list, wrapped with `px-3`; HomeView already hides desktop copy when `phone`. Phone header in the mockup uses `.lsec` style (small section label) rather than h2 — restyle if desired.
- WS: in `stores/websocket.store.ts` `handle` switch, for `threadState` and `threadReply` cases call `invalidateUnreadThreadsFor(envelope.payload.oneofKind)` from `lib/unread-threads-cache.ts` (returns boolean). Also consider adding `keys.notificationLevels.unreadThreads()` to `workspaceResyncKeys` in `lib/realtime.ts` (not mine).

Status: DONE_WITH_CONCERNS
Steps 1, 2 and 5 (desktop) are implemented with tests green; concerns: `?thread=` is not consumed by SpaceView yet, and WS/PhoneChatList wiring is left per the note. No git index touched, nothing committed.
