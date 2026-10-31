# search-suggestions

## Purpose
What the search box of Chat, Danh bạ, Tài liệu, Tài sản and Phê duyệt suggests before anything is typed (and, in
Chat, while words are typed), how it opens and closes, and how a person's recent searches are kept and forgotten.
Design: DESIGN.md "Gợi ý trong ô tìm", mockup `design/mockups/search-gchat.html` (approved 2026-10-18; it replaced
`search-suggestions.html` of 2026-10-17). Suggestions are worked out in the browser from lists the screen already asks
for (Chat's messages are the one exception, asked of `GET /api/messages/search`, see `docs/specs/chat-search`); they add
no endpoint, no NGAC node and no operation. Each module's spec names its groups.

## Requirements

### Requirement: Suggestions show while the box is empty
Focusing an empty search box SHALL show "Tìm gần đây" and the module's own groups; a group with nothing in it is left
out with its label. Typing a letter SHALL give way to the results as before; emptying the box again SHALL bring the
suggestions back, and so SHALL a click into a box that kept the focus after a choice or Esc. A group still loading
SHALL keep its place with skeleton rows; a group that fails to load SHALL be left out without an error.

#### Scenario: Nothing to suggest
- **WHEN** there is no recent search and every group is empty
- **THEN** a desktop opens no popover; a phone shows one line saying what can be searched

### Requirement: Desktop surface, phone screen
At 1024px and wider the box SHALL be one surface that opens out of the box and holds the suggestions under it: the
box is a `combobox`, the suggestions a `listbox` grouped by labelled `group`s; the input is one element that does not
move when the surface opens, and focus stays in it, except by Tab to "Xoá tất cả", which belongs to the surface. ↑/↓ move
through every row, Enter opens the marked row (and only a marked one, otherwise it is the screen's), Delete removes a
marked recent search while the box is empty, Esc closes the surface alone, and leaving the box closes it. A recent
search's ✕ is for the pointer only. At most 5 recent searches and 3 rows of any other group SHALL show. The surface
SHALL NOT reach past the window's right edge (640px wide in Chat, at least 440px and as wide as the box elsewhere), its
rows SHALL scroll inside the room left under the box, and it SHALL open even when there is nothing to suggest, to say
what can be searched. It is revealed with `clip-path` over 280ms (`--ease-out-expo`) and folds back over 210ms
(`--ease-in`); rows slide in 4px and fade, 20ms apart; with reduced motion nothing slides, folds or is clipped over time:
the surface changes at once and rows fade in 120ms.
Below 1024px the suggestions SHALL cover the list under the box, which stays laid out beneath them so going back keeps
its scroll; the box stays a plain search box; each row is a button and a recent search has its own named "Xoá “…” khỏi
Tìm gần đây" button (a 44px target); putting the keyboard away SHALL NOT close them; Quay lại (and Esc from a hardware
keyboard) SHALL. At most 8 recent searches and 5 rows of any other group SHALL show. Filters that would change the
covered list step aside while the suggestions are up. The panel rises 8px as it fades in.

#### Scenario: Esc inside a dialog
- **WHEN** the surface is open in a box inside a dialog and Esc is pressed
- **THEN** only the surface closes

#### Scenario: Reduced motion
- **WHEN** the person asked for reduced motion (the device, or Cài đặt → Giao diện)
- **THEN** no `transform` and no clip transition is applied, and rows and the panel only fade

### Requirement: Only what the caller may read is suggested
A suggestion SHALL come only from a list the server returned for the caller (already filtered by read access) and be
worked out again each time; nothing is remembered about a conversation, file, person, asset or request. A thing the
caller can no longer read SHALL therefore never be suggested. Rows SHALL name things and people, never show an id.

#### Scenario: Read withdrawn
- **WHEN** the caller loses read on a conversation, a file or a request that was suggested before
- **THEN** it is not suggested again

### Requirement: Recent searches per person and module
A search SHALL be kept when the box is submitted with Enter (not the Enter that ends a word an input method is
composing) or when a result found by it is opened: at least 2 characters, newest first, at most 8, one differing only
in case or accents replacing the older one, per person and per module, on this device, under
`ngac-recent-searches:<module>:<user>`. Choosing a recent search SHALL run it again. Chat's searches kept before under
`ngac-chat-recent-searches:<user>` SHALL be moved to the new key on the first read, the old key removed only after the
new one is written; unreadable old data is dropped without hiding a list already in the new place. Signing out SHALL
remove every key of both prefixes. Storage that is blocked SHALL leave the lists empty without an error.

#### Scenario: Matching ignores case and accents
- **WHEN** "Tam ung" is searched after "tạm ứng"
- **THEN** only the newer one is kept

#### Scenario: Sign-out on a shared device
- **WHEN** a person signs out
- **THEN** no recent search of any person or module remains on the device

### Requirement: Recent searches can be forgotten
Each recent search SHALL have a ✕ (the pointer on a desktop, on hover or when the row is marked; always on a phone) and,
on a desktop, Delete when its row is marked and the box is empty. Forgetting one SHALL fold its row away (fade and
height, 160ms), let the rows below glide up, move the mark to the next row (the one before it when it was the last), and
take the group's label with the last one; it SHALL NOT show a toast and SHALL tell a screen reader what was forgotten.
From two searches on, the group SHALL offer "Xoá tất cả" beside its label (reachable by Tab from the box on a desktop).
It SHALL forget all of them at once with no confirmation, return the focus to the box, and show the toast "Đã xoá N lần
tìm gần đây" with **Hoàn tác** for 5 seconds; Hoàn tác SHALL bring them back in their old order, behind anything searched
since, without repeating one differing only in case or accents.

#### Scenario: Delete moves on
- **WHEN** the second of three recent searches is marked and Delete is pressed
- **THEN** it goes, the third is marked, and no toast appears

#### Scenario: Clear all and undo
- **WHEN** "Xoá tất cả" is pressed with three recent searches and then Hoàn tác within 5 seconds
- **THEN** all three are gone, the toast says "Đã xoá 3 lần tìm gần đây", and Hoàn tác restores them in order

### Requirement: Chat suggests while words are typed
Once the box of Chat holds a character that is not a space, the groups SHALL give way to **Người** (at most 3 people of the
directory the caller may browse whose name or username holds every word, never the caller), **Nhóm và trò chuyện** (at
most 3 of the conversations the caller reads whose name holds every word; a direct message with someone already under
Người is not repeated), **Tin nhắn** (from 2 characters, at most 3, asked of the server after 250ms without typing, with
two skeleton rows meanwhile, left out without a word when the server fails) and last, always, **Xem tất cả kết quả cho
“…”**. Matching ignores case and accents and marks the match in the row. Choosing a person opens the direct message
that exists or creates it; a conversation opens; a message opens its conversation at that message (its thread when it is
a reply); each keeps the typed words as a recent search. Enter with no row marked, and the last row, open the results
(`docs/specs/chat-search`). With nothing found the last row stays with the quiet line "Không có gợi ý. Nhấn Enter để tìm
trong mọi tin nhắn." The other modules' suggestions fold away as soon as a letter is typed and their screen filters in
place.

#### Scenario: Person already listed
- **WHEN** "lan" is typed and a direct message with Nguyễn Thu Lan exists
- **THEN** she is listed under Người and that direct message is not repeated under Nhóm và trò chuyện

### Requirement: Chat's box on a desktop
The conversation list of Chat SHALL begin with the search box (a 40px pill, "Tìm trong chat", the key hint `/`) above
"Trò chuyện mới". `/` anywhere in Chat, except while typing in another control or with a dialog open, SHALL focus it and
remember where the focus was. **Esc** SHALL step back: first it closes the suggestions and keeps the words; then it
clears the words (the suggestions stay shut); then it leaves the box and returns the focus to where it was before `/` (a
box opened with the pointer keeps the focus). With words typed the key hint is replaced by ✕ ("Xoá chữ đã nhập"). The
words in the box follow the address of the results page.

#### Scenario: Three Esc
- **WHEN** the focus was on a button, `/` and "lan" are typed and Esc is pressed three times
- **THEN** the suggestions close with "lan" kept; "lan" is cleared; the focus is back on the button

### Requirement: Lists asked for only when needed
Groups whose data the screen does not already load (Tài liệu's "Được chia sẻ với tôi", Danh bạ's direct messages,
Tài sản's "Bạn đang giữ", Phê duyệt's "Đề nghị của bạn") SHALL be asked for only once the box has been focused.

## Known limits
- Android's back gesture leaves Danh bạ, Tài liệu, Tài sản and Phê duyệt rather than closing the phone suggestions;
  only Tìm trong chat owns a history entry.
- On a phone the box of those four screens does not become the 52px accent pill of the mockup; only Chat's pill turns
  into its search box (☰ to ←, accent ring, the list fading, the panel rising).
- A phone's search layer cannot focus the box inside the tap that opens it (the layer mounts after the tap), so some
  phones raise the keyboard a moment late.
- Closing the phone layer is instant for Danh bạ, Tài liệu, Tài sản and Phê duyệt; only Chat's runs its opening backwards.

## Planned change (mobile-ux-overhaul, 2026-10-19)
Phase 08: the search box of every module becomes one search with scopes (module, Tất cả, other modules), per-module filters and actions. See `unified-search`. Plan: `plans/261018-1742-mobile-ux-overhaul`.
