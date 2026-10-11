---
title: "Execution plan — navigator shortcuts UI (Được nhắc tới, Đã gắn sao)"
owner: Claude (planning, review) · executor: agy/Gemini
status: ready (chat lane, after exec-type-scale is committed — shares navigator/message files)
---

# Execution plan — shortcuts UI

APPROVED design: `design/mockups/shortcuts.html` (+ its notes) and DESIGN.md "Lối tắt Được nhắc tới / Đã gắn sao"
(under "Chat: cột điều hướng và mục"). Decisions: starred conversations show ONLY in the navigator's "Đã gắn sao"
section (they keep their section membership on the server and return there when unstarred); mentions count only
direct @you, muted conversations included; deleted/unreadable starred messages simply don't appear; starred
section ordered by starred time.

Backend (LIVE, `docs/specs/chat-shortcuts/spec.md`): `PUT|DELETE /api/channels/{id}/star`,
`PUT|DELETE /api/messages/{id}/star` (204), `GET /api/messages/starred?before=&before_id=&limit=`,
`GET /api/messages/mentions?before=&before_id=&limit=` (items carry `unread`), `GET /api/messages/mentions/unread-count`
→ `{count}` (being added now — if missing when you start, build the badge against it and mock it in tests),
`starred` on each item of `GET /api/channels/unread` and on `GET /api/channels/{id}`. JSON timestamps are
`{seconds,nanos}` objects; the `before` cursor must be RFC3339 with nanoseconds built from seconds+nanos (use
`lib/stream-model.ts timestampToRfc3339`); `unread`/`starred`/`has_more` are omitted when false.

## Steps (tests first)
1. API + hooks: `api/messaging.ts` star/unstar fns, feeds, unread-count; `hooks/useShortcuts.ts` — infinite queries
   (25/page, "Tải thêm"), optimistic star toggles patching the unread list, message caches and feeds, with rollback
   + toasts (Hoàn tác; "Xem" after starring a message). Keys in `hooks/keys/messaging.ts`.
2. Navigator (`components/spaces/ChatNavigator.tsx`, `PhoneChatList.tsx`, `ConversationRow.tsx`): shortcut rows
   "Được nhắc tới" (count pill, 99+) and "Đã gắn sao"; system section "Đã gắn sao" under the shortcuts (hidden when
   empty, collapsible, no ⋯/+), starred conversations excluded from their own section/defaults; row ⋯ for a starred
   conversation = Bỏ gắn sao · Tắt thông báo; others get "Gắn sao". Phone: two 64px tiles at the top of the list.
3. Views: routes for the two screens (desktop main pane, phone full screen with Quay lại) — follow how Trang chủ
   is routed; `MentionsView.tsx`, `StarredView.tsx` per the mockup (day dividers, unread dot + bold, @you highlight,
   reply context line, skeleton, empty, error + Thử lại). Viewing does NOT mark read.
4. Opening an item: navigate to the conversation with the message centered and flashed 2.4 s; a reply also opens the
   thread panel and flashes there. Add the minimal props/route search param to SpaceView/ConversationStream/ThreadPanel
   for "focus message X" (also used by `?thread=` links from push: accept `thread` + optional `message`).
5. Star a message: "Thêm" menu (after Theo dõi thread) + long-press sheet; small accent star after the time on starred
   messages. Conversation star: row ⋯, conversation header name menu (star 16 after the name when starred), sheet.
6. Gates: vitest (only known AssetsScreen failure), lint, typecheck:diff, build; Playwright on the dev stack at 1440
   and 390 (light) → `plans/261010-1559-chat-inline-threads/reports/shortcuts-screens/`.

## Rules
PROGRESS LOG after EVERY step: `plans/reports/agy-progress-shortcuts.log`; read it first when resuming. No backend
edits; no commits; no git index; NEEDS_CONTEXT instead of inventing UI. Report: `plans/reports/agy-<date>-shortcuts-ui.md`.
