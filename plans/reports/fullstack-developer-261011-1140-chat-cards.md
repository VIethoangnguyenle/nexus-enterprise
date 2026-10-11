# Chat cards: component half (exec-11)

Status: DONE_WITH_CONCERNS (approval create dialog skipped: NEEDS_CONTEXT)

## Files (all new, no existing file edited, no git index touched)
- `frontend/src/api/chat-cards.ts`: `pollApi` (create with `ends_at`, get, vote, unvote, close) and `PollView` (adds `results_visible`, `my_option_ids`, `closed`, `closed_at` that `Poll` in messaging.ts lacks). Tasks reuse `messagingApi.createTask/updateTask/listTasks` and `hooks/useChannelTasks`.
- `frontend/src/hooks/keys/chat-cards.ts`: `chatCardKeys.poll(id)` = `['messaging','poll',id]`, `channelFolder(ws,ch)`. NOT yet merged into `hooks/keys/index.ts` (existing file); import it directly, or have the owner of that file add `chatCards: chatCardKeys`.
- `frontend/src/hooks/usePolls.ts`: `usePoll`, `useVotePoll`, `useUnvotePoll`, `useClosePoll`, `useCreatePoll`, `invalidatePollOnVote(queryClient, frame)`.
- `frontend/src/components/chat/`: `ChatCard.tsx` (shared frame, `UnavailableCard`, `CardSkeleton`), `CreateMenuItems.tsx`, `PollDialog.tsx` (+ exported `validatePoll`), `TaskDialog.tsx`, `DocDialog.tsx`, `PollCard.tsx`, `TaskCard.tsx`, `DocCard.tsx`, `ApprovalCard.tsx`.
- Tests: one `*.test.tsx` per component above (except ChatCard), `hooks/usePolls.test.tsx`, fixtures `frontend/src/test/chat-cards-fixtures.ts`.

## Gates
- vitest on my 9 files: 59/59 pass.
- `npx eslint` on all my files: clean (one `eslint-disable-next-line` with reason in PollDialog for the unlabeled choice input, precedent in AppSidebar).
- `npm run typecheck:diff`: no errors in my files. Reported errors are others' files: AttachmentGrid(.test).tsx, ImageViewer.test.tsx, spaces/MessageBlock.test.tsx.

## Skipped / not invented (need design or a call)
1. **Đề nghị phê duyệt create flow (NEEDS_CONTEXT).** `components/approval/CreateRequestDialog` holds its own mutation and `onCreated()` carries no request id, so the message `linked_entity_id` cannot be obtained; `CreateRequestInput` also has no "Gửi vào: nhóm" field. Reuse needs either `onCreated(request)` in that dialog or a channel link on the backend. `CreateMenuItems` still emits `'approval'` (hidden in personal workspaces); the composer must not open anything for it until this is settled. `ApprovalCard` (read side) is done.
2. Doc dialog: no "Bảng tính" type (documents API only creates text) and no "Đổi thư mục" picker; folder shown read-only. Both are in the mockup, deferred.
3. Poll card: "N người đã bình chọn" only shows once results are visible, because the API omits totals before the viewer votes (spec: never computed client side). Mockup shows it before voting. Manager closing via the message "Thêm" menu is not built; the card button is creator-only as the plan says.
4. Task card: the check is the design's `role=checkbox` (aria-label "Đánh dấu xong: …" / "Mở lại: …"), no separate "Xong"/"Mở lại" buttons (plan text vs mockup; mockup followed). "Mở trong Công việc" renders only if `onOpenTasks` is passed (no route known).
5. Approval card: no inline Duyệt / Từ chối (design has them; needs the reject-comment flow). Not in the plan.
6. Poll dialog: separate dialogs per kind, no Bình chọn/Công việc/Tài liệu segmented switch from the mockup. "Cuối ngày hôm nay" = 23:59 (mockup's fixed 17:00 can be in the past). Validation copy is mine (mockup has none): "Nhập câu hỏi.", "Cần ít nhất 2 lựa chọn.", "Tối đa 10 lựa chọn.", "Lựa chọn không được để trống.", "Hạn đóng phải ở tương lai."

## WIRING NOTE

### MessageBlock (components/spaces/MessageBlock.tsx)
Render by `message.linked_entity_type` with `linked_entity_id`, outside the bubble, aligned to the message's side (card is `max-w-105` already). Keep `isSystemMessage` task notes as they are.
- `poll` -> `<PollCard pollId={linked_entity_id} />`
- `task` -> `<TaskCard taskId={linked_entity_id} channelId={message.channel_id} onOpenTasks={...optional} />`
- `document` -> `<DocCard docId={linked_entity_id} />`
- `approval` -> `<ApprovalCard requestId={linked_entity_id} />`
Cards use `useActiveWorkspace`/`usePeople`, so they need the normal router + QueryClient context (already there). A `document`/`approval` message has empty `content`: make sure the bubble is skipped when content is empty and a card exists.

### ChatEditor (components/chat/ChatEditor.tsx), and the thread panel composer
- Put `<CreateMenuItems onSelect={setCreateKind} />` as the second group inside the "+" popover (desktop) and the phone bottom sheet, after the attach group. It renders its own leading separator and handles `isPersonal` and 44px phone rows.
- Keep `createKind: 'poll'|'task'|'doc'|'approval'|null` state and mount, when set:
  - `<PollDialog open onClose channelId />`
  - `<TaskDialog open onClose channelId />`
  - `<DocDialog open onClose workspaceId channelId />` (workspaceId from `useActiveWorkspace()`)
  - `'approval'`: nothing yet (see skipped item 1).
- Dialogs create the object and the message themselves (poll/task: the server posts the message; doc: `useSendMessage` with `linkedEntity`). The composer must not send anything. Inside a thread the card should land in the thread: the dialogs have no `parent_message_id` parameter; if cards must go into threads, that is an extension (tell me).

### websocket.store (stores/websocket.store.ts)
In the envelope switch, add `case 'pollVote':` calling `invalidatePollOnVote(queryClient, envelope.payload.pollVote)` (import from `hooks/usePolls`; generated frame is `PollVoteEvent { pollId, channelId, optionId, voteCount, totalVotes }`, only `pollId` is used; counts in the frame are ignored on purpose). Add the store test the plan lists (frame invalidates `['messaging','poll',id]`).

### Other
- `hooks/keys/index.ts` merge optional (see Files).
- Spec `docs/specs/chat-cards/spec.md`: client-side differences above (items 3, 4) are the only deviations worth noting there.

## Unresolved questions
- Approval create: change `CreateRequestDialog.onCreated` to pass the created request, and does the backend get a channel/"Gửi vào" field?
- Should dialogs support posting into a thread (composer in the thread panel)?
- Route for "Mở trong Công việc"?

Status: DONE_WITH_CONCERNS
Summary: All components, hook, API, keys, fixtures and tests for polls, tasks, documents and the approval card are in and green (59 tests, lint and typecheck clean for my files). The approval create dialog is skipped because the existing form returns no request id, and MessageBlock, ChatEditor and websocket.store still need the wiring described above.
