# Code review: chat wiring before deploy (7fbefc6, df0a6f4, ce64a74, 491e3eb)

## Scope
- Commits: 7fbefc6 (pieces, 92 files, +8.9k), df0a6f4 (type scale), ce64a74 (FileViewer outside chat), 491e3eb (wiring, 105 files, +5.4k)
- Focus: production risk. I traced the send, draft, quote, star and level caches, the realtime handlers, route search, card access and the viewer. Paths below are relative to `frontend/src/`.
- Not done: a visual check at 390px (I could not run a browser here) and `npm run build` (see Gates).

## Gates (run by me)
| Gate | Result |
|---|---|
| `npx vitest run` | 2113 pass, 1 fail: `components/assets/AssetsScreen.test.tsx:103`. This is the failure the executors already reported. It is outside this diff. |
| `npm run lint` | clean. The errors the wiring-B report listed are gone. |
| `npm run typecheck:diff` | no file above baseline |
| `npm run build` | **NOT RUN.** The AgentKit scout-block hook rejects any Bash command containing "build". The executors reported it ok, but nobody has checked that since. Run it by hand before deploying. |

## Verdict
None of the findings below is a crash, an infinite loop or a server-side authorization hole. Route search values are checked against an id pattern. Cards are read live under the viewer's own access (403/404 shows the "không xem được" state). I found no raw IDs on screen. The vite proxy already covers the new REST prefixes (`/api/messages`, `/api/channels`, `/api/polls`, `/api/tasks`, `/api/drive`).

Two findings should be fixed before deploy. Both are small (H1, H2).

---

## Critical
None.

## High (deploy blockers)

### H1: Drafts and pending quotes survive sign-out in the same tab (leaks between users)
`stores/drafts.store.ts` (the whole store) and `stores/auth.store.ts:82-88`.
`logout()` clears `queryClient` but never calls `useDraftsStore.getState().reset()`. The store holds the unsent HTML of every composer the user typed in, and quote excerpts of up to 160 characters of message text. If a second person signs in on the same tab (a shared office PC, the PWA on a shared phone) and opens the same conversation, they see the first person's unsent draft and pending quote in the composer. If they press Enter, it is sent under their own name. The `editing` map has the same problem.
**Fix:** in `logout()` add `useDraftsStore.getState().reset()`. This needs a lazy import, or a small `onLogout` registry, so auth.store does not import the components graph. Add a test: "drafts are gone after logout". The same applies to any future in-memory per-user store.

### H2: A sent message comes back as the draft when the composer unmounts before the server answers
`components/chat/ChatEditor.tsx:392-403`, and `scopeRef` at :128.
`handleSend` awaits `onSend` (that is, `mutateAsync`) and only afterwards runs `editor.commands.clearContent()` and `setHtml(scope, '')`. During the await, the draft store still holds the sent text, because `onUpdate` wrote it on every keystroke. Here is how it goes wrong:
- The user presses Enter, then quickly opens another conversation. That is common on a slow phone network. SpaceView is keyed by `channelId`, and ThreadPanel by `thread-${id}`, so the editor is destroyed.
- When the send resolves, `editor.commands.*` on the destroyed tiptap v3 editor throws. The `catch {}` swallows it, so `setHtml(scope, '')` never runs.
- When the user comes back, the composer is refilled with a message that was already sent (`content: drafts[scope].html` at :160). One Enter sends it a second time.

The same thing happens when the thread panel is closed mid-send.
**Fix:** snapshot the scope and HTML at send time. Clear the draft in the store right away (`setHtml(scope, '')` before the await). On failure, restore it only if the draft is still empty. Guard the editor calls with `if (!editor.isDestroyed)`. Test: unmount the editor while `onSend` is pending, resolve it, remount, and expect an empty box.

## Medium

### M1: Any 400 on send marks the quote "deleted"
`hooks/useMessages.ts:57-66` (`settleQuote`).
The server answers 400 for many reasons: a bad attachment, an empty message, a bad parent (`services/messaging/internal/domain/messages.go:56-66`, plus the attachment checks after it). Each of these currently flips the strip to "Tin nhắn đã bị xoá", and the next send silently drops a quote that is in fact still valid.
**Fix:** mark the quote deleted only when the error message matches the quote refusal ("a quote names a message of this channel"). A distinct error code from the server would be better. Otherwise keep the quote. Test: a 400 caused by an attachment keeps the quote live.

### M2: A failed star does a whole-list rollback that wipes realtime updates
`hooks/useShortcuts.ts:154-182` (message star) and `:205-224` (conversation star).
On failure, the code restores snapshots of the entire message list, the thread and the unread list taken before the request. Any message, edit, reaction or unread count that arrived by WebSocket while the request was in flight is lost until the next refetch. Neither function calls `cancelQueries` first. The unread list is invalidated on every `chatMessage` frame, so an in-flight refetch can land after the optimistic patch with `starred=false`. The row then jumps back to its old section, and because nothing is invalidated after success it stays wrong until the next frame. `changeNotificationLevel` already does this properly with a per-item patch.
**Fix:** roll back by applying the inverse patch (`setMessageStarred(old, id, !starred)` / `setUnreadListStarred(old, id, !starred)`) instead of restoring snapshots. Call `await queryClient.cancelQueries` on the list keys before patching. Invalidate `unreadCounts` when a conversation star settles.

### M3: The "Được nhắc tới" pill and feed go stale after reading
Only two things touch the pill: a new mention frame (`stores/websocket.store.ts:364`) and opening an item from the feed (`hooks/useShortcuts.ts:117-122`). Reading the conversation from the navigator does not (`hooks/useMessaging.ts:64`). Neither does the thread mark-read, the `unreadCount` frame (`websocket.store.ts:471`), reading on another device, or `messageEdited`/`messageDeleted` (`:506`, `:510`). The spec says a mention added by an edit is unread (`docs/specs/chat-shortcuts/spec.md:47-48,62-64`), but the client never hears about it live. Result: the badge keeps counting mentions the user has already read, and misses mentions added by an edit.
**Fix:** invalidate `keys.shortcuts.mentionsUnreadAll()` (and `mentionsAll()`) in `useMarkRead.onSuccess`, in the thread mark-read success, on the `unreadCount` frame, and in `messageEdited`/`messageDeleted` when the old or new mentions include me.

### M4: For other viewers, removing every mention in an edit does not take effect live
`lib/chat-cache.ts:89`: `mentions: m.mentions?.length ? m.mentions : cur.mentions`.
491e3eb now sends `mentions: []` when an edit removes all mentions (`hooks/useMessageLifecycle.ts`). But the realtime patch treats an empty list as "keep the old one". Other viewers keep the old mentions, and with them the @you wash from `SelfMentionStyle`, until they refetch. The line predates this diff, but the new feature is what makes it reachable.
**Fix:** use `m.mentions ?? []`. If the frame cannot tell "unchanged" apart from "empty", add a presence flag.

### M5: The notification level can come from a stale cache, so a choice is ignored
`hooks/useNotificationLevel.ts:23-28` (`currentLevel`) and `:121`.
SpaceView passes `fetchLevel: false` and displays `conv.level` from the unread list. But `currentLevel` reads the level query cache first. An earlier optimistic `setQueryData(levelKey, …)` leaves a value there that is never refetched, because that query is disabled. After the level changes on another device or tab, the header shows the new level. Picking the old level again then returns early (`level === currentLevel`), so nothing is sent, and the undo/toast "prev" is wrong.
**Fix:** read the unread list first when it exists, or invalidate/remove `levelKey` in the `finally` of `changeNotificationLevel`.

### M6: Spec and plan follow-ups the wiring left open
- Wiring-B says `docs/specs/chat-cards` (thread placement, client differences) and `docs/specs/chat-message-lifecycle` (quote draft, mark-unread hold) were not updated. CLAUDE.md §4 requires the spec update in the same piece of work.
- The W5 Playwright screens (exec-03 step 5) and the exec-quote step 6 screenshots were not run by either executor. Nobody has visually checked the 390px layouts (title menu `w-86`, quote strip, level sheet, create sheet).

## Low
- **L1:** `components/chat/ChatEditor.tsx:398` together with `hooks/useMessages.ts:72`. The quote stays pending until the server answers. If a second message is sent during that window, it carries the same `quoted_message_id`. Clear the pending quote from the strip optimistically and put it back on error.
- **L2:** `stores/drafts.store.ts` (`scopeOf`). Choosing "Trích dẫn" on the thread root inside the thread panel sends the quote to the channel composer. On a phone that composer is hidden behind the full-screen panel, so nothing visible happens. Use the thread scope when the action comes from the panel.
- **L3:** `routes/_workspace/channels.$channelId.tsx` with `components/chat/ThreadPanel.tsx`. The `thread` param is not checked against the `channelId` of the URL. A crafted or old link opens a thread of another channel inside this view, and replies are then posted with the wrong `channelId`. The server refuses them (`checkReplyParent`), but the user only sees an error. Close the panel when `root.channel_id !== channelId`.
- **L4:** `hooks/useFocusMessage.ts`. The effect is keyed on `target`. Clicking the same mention link twice, or closing and reopening the same `?thread=` link, does not flash or reopen anything, because the search params stay in the URL. Clear `thread`/`message` from the search once the flash has run (`navigate({ search: …, replace: true })`).
- **L5:** `hooks/useShortcuts.ts:165`. The "Đã gắn sao tin" toast shows before the server answers. On failure the user sees a success toast and then an error toast.
- **L6:** `lib/conversations.ts:151-152`. The "Chưa đọc" filter uses `c.unread`, which is 0 for a muted conversation. A muted conversation with an unread @mention is therefore hidden from the filter while still counted in the totals (`unreadOf`). Decide with design; then use `unreadOf(c) > 0`.
- **L7:** Layering. `stores/drafts.store.ts:3` imports `components/chat/AttachmentGrid`, and `lib/message-actions.ts:2` imports `components/chat/MessageCard`, which pulls in every card component. Move `isImageAttachment` and `cardKindOf` into `lib/`.
- **L8:** `hooks/useShortcutScreen.ts` `back()`. `history.length > 1` is true even when the previous entry belongs to another site, so "Quay lại" can leave the app. Keep an in-app "came from" flag instead.
- **L9:** Messages created before rich text whose mentions are not `data-mention` spans lose all their mentions on edit, because `mentionsFromHtml` returns `[]`. This is an edge case.

## Edge cases traced and found to be fine
- `validateChannelSearch` and `useFocusMessage`/`SelfMentionStyle` use `/^[\w-]{1,64}$/`, so no selector or CSS injection.
- `useFocusMessage`: the MutationObserver stops after 15 s and after at most 12 older pages, and is cleaned up on unmount. No loop.
- `FileViewer` clamps an out-of-range index and handles a shrinking `files` list (`components/files/FileViewer.tsx:66-69`).
- `isQuoteGone` with "in thread, not found means gone" is safe: `GET /messages/{id}/thread` is not paginated, and `injectReply` never creates a partial thread cache.
- WebSocket `chatMessage` carries `quoted` and `attachments`, so the dedup that replaces the optimistic copy keeps the quote.
- Feeds are scoped server-side to the signed-in workspace and to channels the user can read, and the keys carry `wsId`.
- The push URL already sends `&message=` (`backend/services/messaging/internal/domain/push.go:340-342`), so the wiring-A follow-up is done. The MobileNav badge now uses `unreadByLevel`.

## Recommended order
1. H1, H2: block deploy until fixed and tested.
2. Run `npm run build` by hand.
3. M1 to M5, as a follow-up the same day. M2 and M3 are what users will notice first.
4. M6: update the specs and take the 390px screenshots before calling the plan done.

## Plan status
Waves W1 to W4 look complete in code. The non-Playwright part of W5 is done. Still open: the W5 Playwright screens, the spec updates (M6), and the H/M fixes above.

## Unresolved questions
- L6: should a muted conversation that has an @mention appear under the "Chưa đọc" filter?
- M1: can messaging return a distinct error code for a refused quote, so the client does not have to match on the message text?

Status: DONE_WITH_CONCERNS
Summary: The four commits have no crash, loop or authorization defect, and lint, typecheck:diff and vitest are green apart from the AssetsScreen failure the executors already reported. Two data-safety bugs should block the deploy: drafts are not cleared on logout (H1), and a send that finishes after the composer is gone leaves the sent text as the draft (H2); the build gate also still has to be run by hand.
