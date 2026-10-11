---
title: "Execution plan — pending quote in the composer + mark-unread rule + mentions on edit (frontend)"
owner: Claude (planning, review) · executor: agy/Gemini
status: ready (chat lane, after exec-shortcuts-ui)
---

# Execution plan

APPROVED design: `design/mockups/composer-quote.html` + report `plans/reports/ui-ux-designer-261011-1030-composer-quote.md`
(apply its decisions). User decisions 2026-10-11:
1. A pending quote is kept WITH THE DRAFT per conversation: leave and come back → the quote strip and the typed text
   are still there (store drafts per conversation id in a small Zustand store, not localStorage-only; survive route
   changes within the session).
2. Editing a sent message and adding @people notifies only the newly added people — the backend handles it; the
   frontend must send the edited message's `mentions` (ids from the mention nodes, like a send) on edit. Check
   `plans/reports/fullstack-developer-261011-1110-edit-mentions.md` for the exact request shape.
3. After "Đánh dấu chưa đọc" in a thread, reopening it right away does NOT mark it read; it is marked read the next
   time it is opened after the person has left it (track "marked unread this visit" per thread in client state;
   clear it when the thread panel closes or the conversation changes).

## Steps (tests first)
1. "Trích dẫn" action in the "Thêm" menu and long-press sheet (it was withheld in phase 08) → sets the pending quote
   for the composer where the message lives (thread panel composer for a reply).
2. Quote strip per mockup (region labelled "Đang trích dẫn tin của <tên>", ✕ "Bỏ trích dẫn", Esc order: popovers
   first, then the quote; ↑ edit-last disabled while a quote is pending; attachments/cards summarised); send with
   `quoted_message_id`; deleted-while-pending state ("Tin nhắn đã bị xoá … gửi không kèm trích dẫn"); phone edit bar
   replaces the strip and the quote returns after Lưu/Huỷ.
3. Drafts store (decision 1) incl. the quote; restore on return.
4. Edit sends `mentions` (decision 2).
5. Mark-unread rule (decision 3).
6. Gates: vitest (only known AssetsScreen failure), lint, typecheck:diff, build; Playwright 1440 + 390 screenshots →
   `plans/261010-1559-chat-inline-threads/reports/quote-screens/`.

## Rules
PROGRESS LOG after EVERY step: `plans/reports/agy-progress-quote.log`; read it first when resuming. No backend edits,
no commits, no git index; NEEDS_CONTEXT instead of inventing UI. Report: `plans/reports/agy-<date>-quote-composer.md`.
