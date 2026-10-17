# search-suggestions

## Purpose
What the search box of Chat (Tìm trong chat on a phone), Danh bạ, Tài liệu, Tài sản and Phê duyệt suggests before
anything is typed, and how a person's recent searches are kept. Design: DESIGN.md "Gợi ý trong ô tìm", mockup
`design/mockups/search-suggestions.html` (approved 2026-10-17). Suggestions are worked out in the browser from lists
the screen already asks for; they add no endpoint, no NGAC node and no operation. Each module's spec names its groups.

## Requirements

### Requirement: Suggestions show while the box is empty
Focusing an empty search box SHALL show "Tìm gần đây" and the module's own groups; a group with nothing in it is left
out with its label. Typing a letter SHALL give way to the results as before; emptying the box again SHALL bring the
suggestions back, and so SHALL a click into a box that kept the focus after a choice or Esc. A group still loading
SHALL keep its place with skeleton rows; a group that fails to load SHALL be left out without an error.

#### Scenario: Nothing to suggest
- **WHEN** there is no recent search and every group is empty
- **THEN** a desktop opens no popover; a phone shows one line saying what can be searched

### Requirement: Desktop popover, phone screen
At 1024px and wider the suggestions SHALL be a popover under the box: the box is a `combobox`, the suggestions a
`listbox` grouped by labelled `group`s; focus stays in the box; ↑/↓ move through every row, Enter opens the marked row
(and only a marked one, otherwise it is the screen's), Delete removes a marked recent search, Esc closes the popover
alone, and leaving the box closes it. A recent search's ✕ is for the pointer only. At most 5 recent searches and 3 rows
of any other group SHALL show. The popover SHALL stay inside the window and be at least as wide as the box.
Below 1024px the suggestions SHALL cover the list under the box, which stays laid out beneath them so going back keeps
its scroll; the box stays a plain search box; each row is a button and a recent search has its own named "Xoá “…” khỏi
Tìm gần đây" button; putting the keyboard away SHALL NOT close them; Quay lại (and Esc from a hardware keyboard) SHALL.
At most 8 recent searches and 5 rows of any other group SHALL show. Filters that would change the covered list step
aside while the suggestions are up.

#### Scenario: Esc inside a dialog
- **WHEN** the popover is open in a box inside a dialog and Esc is pressed
- **THEN** only the popover closes

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

#### Scenario: Sign-out on a shared device
- **WHEN** a person signs out
- **THEN** no recent search of any person or module remains on the device

### Requirement: Lists asked for only when needed
Groups whose data the screen does not already load (Tài liệu's "Được chia sẻ với tôi", Danh bạ's direct messages,
Tài sản's "Bạn đang giữ", Phê duyệt's "Đề nghị của bạn") SHALL be asked for only once the box has been focused.

## Known limits
- Android's back gesture leaves Danh bạ, Tài liệu, Tài sản and Phê duyệt rather than closing the phone suggestions;
  only Tìm trong chat owns a history entry.
- On a phone the box of those four screens does not become the 52px accent pill of the mockup.
- Desktop Chat has no module-wide search box, so it has no suggestions.
