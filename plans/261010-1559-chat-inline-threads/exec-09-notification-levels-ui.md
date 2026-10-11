---
title: "Execution plan — chat phase 09 notification levels + Trang chủ threads UI"
owner: Claude (planning, review) · executor: agy/Gemini
status: ready (after chat 07/08 + 10 UI are committed)
---

# Execution plan: phase 09 — frontend only

Backend DONE (commits 5e924df, c9829c8, dae31b2):
- `GET|PUT /api/channels/{id}/notification-level` → `{level}`; levels `all` | `main` (default) | `off`;
  a DM accepts only `main` (shown as "Bật") / `off` — `all` on a DM is 400.
- `GET /api/channels/unread` items now carry `notification_level` and `mention_count` (unread main messages
  mentioning me, at every level; under `off` unread_count = thread_unread_count = 0).
- `GET /api/threads/unread?limit=` → `{messages: [...root messages with thread_unread_count, last_reply_at,
  reply_participant_ids, channel_id]}` newest reply first; muted conversations excluded.
Specs: `docs/specs/notifications/spec.md`, `docs/specs/chat-threads/spec.md`, `docs/specs/web-push/spec.md`.
Design (decided; render it, do not redesign): `design/mockups/notification-levels.html` §1–§5 — read its notes
column for every rule (row muted look, "@" pill, menu in the group header, phone info screen + sheet, Trang chủ
"Thread có tin mới" section: max 3 desktop / 2 phone + "Xem thêm N thread", hidden when empty, collapsible).

## Steps (do them in order; each with tests first)
1. Types/API: `api/messaging.ts` — `notification_level?: 'all'|'main'|'off'` and `mention_count?: number` on the
   unread item type; `getNotificationLevel(chId)`, `setNotificationLevel(chId, level)`, `unreadThreads(limit)`.
   Query keys in `hooks/keys/messaging.ts`: `notificationLevel(chId)`, `unreadThreads()`.
2. Hook `hooks/useNotificationLevel.ts`: query + optimistic mutation that also patches the unread-counts cache
   (`notification_level`, zeroing counts for `off`), rollback on error, toast "Đã … thông báo của “<tên>”" with
   Hoàn tác (restores the previous level), error toast per mockup. Tests: optimistic, rollback, Hoàn tác.
3. Group header menu (`components/spaces/SpaceView.tsx` header name button): the "Thông báo" radio group
   (menuitemradio, aria-checked, explanation line as aria-describedby), "Mặc định" tag on `main`, status icon
   after the name (bell-ring for all, bell-off for off, none for main) with hidden text. DM: two items Bật/Tắt.
   Phone: "Thông tin" → info screen row "Thông báo" → bottom sheet radiogroup (§5). Tests: menu roles, choose
   level calls PUT, DM shows two items only.
4. Navigator rows (`ChatNavigator.tsx`, `PhoneChatList.tsx`, `ConversationRow`): muted look + bell-off + "@" pill
   from `mention_count`; row ⋯ menu / long-press sheet item "Tắt thông báo" ↔ "Bật thông báo" (Bật = `main`).
   Totals (sidebar Tin nhắn badge in `routes/_workspace.tsx`, tab, folded section header) sum by level: muted
   conversations add only `mention_count`. Tests for each.
5. Trang chủ "Thread có tin mới" (`HomeView.tsx` desktop / `PhoneChatList.tsx` phone) from `unreadThreads`:
   rows per mockup, tap → navigate to the channel and open its thread panel (use the existing thread-open route
   param / handler used by ThreadsPanel), "Xem thêm N thread", collapsible (remember per device in localStorage
   via try/catch), hidden when empty. Invalidate `unreadThreads()` on WS `threadState` / `threadReply` frames
   (add to `stores/websocket.store.ts` with tests).
6. Gates: `cd frontend && npx vitest run` (only the known AssetsScreen failure), `npm run lint`,
   `npm run typecheck:diff`, `npm run build`.

## Rules for the executor
- PROGRESS LOG (mandatory): after EVERY step append `<time> | step | files | result` to
  `plans/reports/agy-progress-chat-09.log`; read it first when resuming and skip what is done.
- Tests first for every behaviour; no UUID/ID on screen; 44px targets on phones; Vietnamese copy from the mockup.
- Do NOT commit; do NOT touch the git index. Do not edit backend/.
- If the mockup does not cover something you need, stop that item and write NEEDS_CONTEXT in the report.
- Report: `plans/reports/agy-<date>-chat-09-ui.md` — files, tests, gate output, skipped items.
