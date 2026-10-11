# Chat phase 10: sections UI (frontend)

Status: DONE_WITH_CONCERNS

## Built
- `lib/chat-sections.ts` (+test): arrange, name validation, optimistic reducers.
- `api/chat-sections.ts`, `hooks/useChatSections.ts` (+test, 12): optimistic edits with rollback; create/rename report errors inline (409 -> "Bạn đã có mục tên này"); delete -> toast "Hoàn tác" 5s -> recreate with name+position+channel_ids.
- `hooks/useChatNavigation.ts`: conversations + sections. Sections load failure leaves default lists.
- `stores/nav-collapse.store.ts`: fold state per device (persisted, Zustand).
- `ChatNavigator.tsx`: user sections above DM/Nhóm, "Tạo mục", row ⋯ menu (menuitemradio), section ⋯ menu (rename/up/down/delete), name popover, drag row onto section header, unread pill when folded, empty hint.
- `PhoneChatList.tsx` + `ConversationRow.tsx` (extracted from HomeView): phone Tin nhắn tab, 64px-class rows, long press / Menu key / Shift+F10 -> `NavSheet` (bottom sheet), section ⋯ 44px -> sheet, name form in sheet.
- `NewChatMenu`: "Tạo mục" item (optional prop).
- Tests: ChatNavigator.sections (27), PhoneChatList (13), HomeView.phone (3), plus helper `test/chat-sections-server.ts`.

## Left out (no backend)
Gắn sao, Tắt thông báo (long-press sheet / row menu); shortcuts Được nhắc tới, Đã gắn sao; Duyệt nhóm.

## Shared files touched (minimal)
- `primitives/Popover.tsx`: arrow keys also visit `menuitemradio` and skip disabled items.
- `hooks/useLongPress.ts`: optional 3rd param `ignore` (rows that are links).
- `hooks/keys/messaging.ts`: `sections(wsId)`.
- `test/chat-fixtures.ts`: fixtureApi answers GET chat-sections with empty list.
- `HomeView.tsx`: below 1024px renders PhoneChatList (no filter chips, as in chat-mobile.html); desktop feed unchanged.
- `ChatNavigator.test.tsx`: reset fold store between tests.
- vite proxy: verified, chat-sections already routed (regex block line 40); not edited.

## Design notes / deviations
- Phone avatar 40 not 44 (Avatar scale has no 44).
- Section ⋯ on phone is a sheet per DESIGN.md text ("menu ⋯ thành sheet đáy"); chat-mobile.html does not draw it.
- Create is not optimistic (needs server id); everything else is.
- Desktop ⋯ 28px per design; phone targets 44.

## Gates
- vitest: 1479 pass, 1 fail (known AssetsScreen overview).
- lint: 1 error in QuoteBlock.tsx (other agent, not mine). Mine clean.
- typecheck:diff: only `lib/message-model.test.ts` (other agent). Mine clean.
- build: ok.
- Browser check at 390/1440 light/dark and screenshots: NOT done (subagent has no browse tool; Vite :5173 is up but no saved session). Screenshots dir not created.
- Spec unchanged (behaviour matches). Phase-10 status line and plan.md row updated, not marked completed.

## Open
- Browser verification + screenshots to `plans/261010-1559-chat-inline-threads/reports/phase-10-screens/`.
- ConversationStream tests failed intermittently mid-run from the other agent's in-progress work; full run was green except AssetsScreen.

Status: DONE_WITH_CONCERNS
Summary: Sections UI (desktop navigator, phone list with long-press sheet, Undo) is built test-first and all gates are green for my files; browser verification and screenshots are not done.
