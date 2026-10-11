# Wiring A (W1 focus + shortcuts, W2 notification levels)

Gates: full vitest 2113 pass, 1 fail (AssetsScreen, known). eslint clean on my files. tsc: 0 errors outside generated/. `npm run build` ok. typecheck:diff ok.

## Done
- Decision 1: route search `thread` + `message` (id-shaped only); SpaceView `initialThreadId`/`focusMessageId`; `hooks/useFocusMessage.ts` (centre, 2.4 s `data-flash`, waits for render, loads older pages, reduced motion). Flash in thread panel is found via `#reply-<id>-text` / `#thread-root-<id>-text` (no edit to ThreadPanel).
- Decision 2: `starred` on Message/Channel/ChannelUnread; `keys.shortcuts`, `keys.chatCards` registered; local patch types dropped.
- Decision 3: websocket.store: mention -> mentionsAll + mentionsUnreadAll (others' messages/replies naming me only); threadState/threadReply -> unread threads; pollVote -> `invalidatePollOnVote`. Test per frame.
- Routes `/channels/mentions`, `/channels/starred` (+ `hooks/useShortcutScreen.ts`); phone: full screen, tab bar steps aside (matches `isConversationPath`), Quay lại = history back or /channels.
- Navigator: shortcut rows + pill (99+, fades), "Đã gắn sao" section (bare, collapsible, starred excluded elsewhere), muted look (bell-off, "@" pill, no count), row menu Gắn sao/Bỏ gắn sao + Tắt/Bật thông báo (starred row: no section items). Phone: tiles, starred group, sheet items, UnreadThreadsSection (max 2) mounted.
- Header: name button menu (radio group, Mặc định tag, Gắn sao, Thông tin), status icon, star, "Đang lưu…" after 300 ms; DM Bật/Tắt only. Phone: name opens info; info row Thông báo + level sheet.
- Totals by level: `unreadOf/totalUnread/unreadByLevel`; sidebar badge, Trang chủ, folded headers (now also Nhóm/DM on desktop).
- Decision 8: unstar collapse 220 ms, new mention grows in + sender wash (history/older pages excluded), @you-only wash (`SelfMentionStyle` + index.css), reduced motion = fade.
- Muted rows: no wash, no slide, excluded from burst line (Home, phone).
- Coordinator items: `EditMessageInput.mentions?`, `createTask` data `parent_message_id?` added (B may drop its local types in hooks/useMessageLifecycle.ts, api/chat-cards.ts). `useOpenStarred()` exported from `hooks/useShortcutScreen.ts` for the "Xem" toast.

## Files changed (frontend/src)
New: hooks/{useFocusMessage,useDelayedFlag,useShortcutScreen}.ts(+tests for first two), lib/{feed-motion,notification-level-copy}.ts(+tests), test/chat-state-server.ts, routes/_workspace/channels.{mentions,starred}.tsx, routes/channels-shortcuts.test.tsx, components/spaces/{ConversationBadges,ConversationTitleMenu,NotificationLevelSheet,SelfMentionStyle}.tsx(+tests for menu, self-mention), ChatNavigator.shortcuts.test, PhoneChatList.shortcuts.test, HomeView.muted.test, SpaceView.levels.test.
Edited: api/messaging.ts, hooks/keys/index.ts, hooks/{useShortcuts,useNotificationLevel(+test),useConversations}.ts, lib/{conversations,chat-sections,shortcut-feed}.ts(+tests), stores/websocket.store.ts(+test), routes/_workspace.tsx, routes/_workspace/channels.$channelId.tsx, routes/channels-thread.test.tsx, routeTree.gen.ts (plugin), index.css, components/spaces/{SpaceView,ChatNavigator,PhoneChatList,ConversationRow,HomeView,FeedParts,MentionsView,StarredView,SectionOverlays,SpaceInfoPanel}.tsx and tests (SpaceView, ChatNavigator.sections, PhoneChatList, MentionsView, StarredView).
Outside my listed set but unowned by B: SectionOverlays, SpaceInfoPanel, index.css, lib/conversations, lib/chat-sections, hooks/useConversations. Did not touch ConversationStream or useMessageActions.

## ASK OTHER AGENT / follow-ups
- MobileNav (components/patterns) has no Tin nhắn badge; mockup shows one. Use `unreadByLevel(useUnreadCounts().data?.channels)`.
- Push URL (backend push.go:336) carries `?thread=` only; add `&message=<reply id>` to flash the reply.
- NEEDS_CONTEXT: starred section order by starred time (unread list has no starred_at; list follows activity order); no copy for "target message not found" (silent).
- Tiles precede "Thread có tin mới" on phone (mockups disagree).

Status: DONE_WITH_CONCERNS
All of W1+W2 is wired and tested; concerns are the unowned phone-tab badge, the push `message` param and starred ordering. Playwright screens were not run.
