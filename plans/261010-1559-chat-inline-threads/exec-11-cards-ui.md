---
title: "Execution plan — chat phase 11 in-chat cards UI"
owner: Claude (planning, review) · executor: agy/Gemini
status: ready (start after exec-06 attachments UI is committed — same composer files)
---

# Execution plan: phase 11 cards — frontend only

Backend is DONE: polls `POST /api/channels/{ch}/polls {question, options[], is_multi, is_anonymous, ends_at}`,
`POST|DELETE /api/polls/{id}/vote {option_id}`, `GET /api/polls/{id}`, `POST /api/polls/{id}/close`;
tasks `POST /api/channels/{ch}/tasks {title, assignee_id, due_date}`, `PATCH /api/tasks/{id} {status|assignee_id|title|due_date}`,
`GET /api/channels/{ch}/tasks`. A created poll/task arrives as a message with `linked_entity_type` `poll` / `task`
and `linked_entity_id`. WS frame `pollVote {poll_id, channel_id}` means "refetch this poll" (it carries no counts).
Contract and permission rules: `docs/specs/chat-cards/spec.md` (read all of it). Design (approved — render it,
do not redesign): `design/mockups/spaces.html` section `#cards` (`#poll-demo`, `#task-demo`, the create dialogs),
`design/mockups/chat-mobile.html` composer "+" sheet second row (Bình chọn / Công việc / Tài liệu mới /
Đề nghị phê duyệt), `DESIGN.md` cards paragraphs.

## Decisions (fixed)
1. Entry: the composer "+" menu (desktop popover, phone bottom sheet) gets a second group: Bình chọn,
   Công việc, Tài liệu mới, Đề nghị phê duyệt. "Đề nghị phê duyệt" is not shown when
   `useWorkspaceType().isPersonal` (hooks/useWorkspaceType.ts) — the server refuses it there anyway.
2. Poll card (`PollCard.tsx`): question, choices; before the viewer votes and while open → no counts (the
   API omits them; never compute them client-side); single choice = radio behaviour, multi = checkboxes;
   "Rút phiếu"; "Đóng bình chọn" shown to the creator (`created_by === me`) — the server decides manage, so on
   403 show toast "Bạn không đóng được bình chọn này"; closed / past `ends_at` → results, no voting. Anonymous
   → never show voter names. Data: `useQuery(['messaging','poll',pollId])` (add a key in `hooks/keys/messaging.ts`);
   on WS `pollVote` invalidate that key (add the case in `stores/websocket.store.ts` with a test).
3. Create poll dialog: question, 2–10 choices (add/remove rows), Nhiều lựa chọn, Ẩn danh, Hạn đóng (optional,
   date-time, must be in the future) — validation messages as in the mockup; on success the message arrives via
   the normal send/refetch path.
4. Task card (`TaskCard.tsx`): title, assignee (display name via the people directory, never an id), due date,
   status; "Xong" / "Mở lại" visible to the assignee or creator (server is the authority; on 403 toast). Assignee
   picker in the create dialog = channel members (`useChannelMembers`), searchable; no typed ids.
5. Tài liệu mới: dialog asks the title → `documentsApi.create(wsId, {title, folder_id})` with the channel drive's
   folder (same lookup as attachments: `driveApi.channelDrive` → folder) → send a message
   `{content: '', linked_entity_type: 'document', linked_entity_id: <doc id>}`. Doc card (`DocCard.tsx`) fetches the
   document under the viewer (existing documents API `get`); 403/404 → card "Tài liệu không xem được" with no details.
6. Đề nghị phê duyệt: reuse the existing approval create flow (search `components/approval` for the new-request
   dialog/form used by Phê duyệt; open it as a dialog), then send a message with `linked_entity_type: 'approval'`.
   Approval card (`ApprovalCard.tsx`) reads the request via the existing approval API under the viewer; no access
   → "Đề nghị không xem được". If the approval create flow cannot be reused as a dialog without redesign, stop and
   report NEEDS_CONTEXT for this item only (do the others).
7. `MessageBlock.tsx` renders the card for `linked_entity_type` poll/task/document/approval (task notes rendered
   as system lines today stay as they are — check `lib/stream-model.ts isSystemMessage`).
8. No UUID/ID on screen; 44px targets on phones; Vietnamese copy from the mockup.

## Files
- Create: `frontend/src/components/chat/{CreateMenu,PollDialog,TaskDialog,DocDialog,PollCard,TaskCard,DocCard,ApprovalCard}.tsx` (+ tests)
- Create: `frontend/src/hooks/usePolls.ts` (+ test) — poll query, vote, unvote, close, create
- Modify: `components/chat/ChatEditor.tsx` (+ menu), `components/spaces/MessageBlock.tsx`, `api/messaging.ts`
  (poll/task API fns if missing), `hooks/useChannelTasks.ts` (update mutation), `hooks/keys/messaging.ts`,
  `stores/websocket.store.ts` (pollVote → invalidate), `test/chat-fixtures.ts`.
- Spec: note client behaviour in `docs/specs/chat-cards/spec.md` only if something differs.

## Tests FIRST (vitest)
1. PollCard: not voted → no counts rendered; vote → refetch shows counts; multi allows two; anonymous shows no
   names; closed shows results and no vote controls; "Đóng bình chọn" only for the creator.
2. Poll dialog validation: <2 or >10 choices, empty choice, past end date → errors, no request.
3. TaskCard: assignee shown by name; "Xong" sends PATCH status done; 403 → toast, state unchanged.
4. CreateMenu: approval item hidden in a personal workspace; present in an organization.
5. websocket.store: `pollVote` frame invalidates `['messaging','poll',id]`.
6. Doc/Approval card: 403 → "không xem được", no title leaks.

## Gates, rules, report
Same as `exec-06-attachments-ui.md`: vitest (only the known AssetsScreen failure), lint, typecheck:diff, build;
no commits, no git index; report to `plans/reports/agy-<date>-chat-11-cards-ui.md`.
PROGRESS LOG (mandatory): after EVERY step append one line to `plans/reports/agy-progress-chat-11.log`
(`<time> | step | files | result`); when resuming, read it first and skip what is done.
