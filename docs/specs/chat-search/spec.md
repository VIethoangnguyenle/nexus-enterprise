# chat-search

## Purpose
Finding a message by its words, in every conversation the caller reads at once (the search box and results of Chat on a
desktop, the search screen on a phone) and inside one conversation. Search is read-only over messages; it grants nothing and adds no NGAC node or operation.
A hit is message text, so it follows exactly the read access that unread counts, mentions and previews follow.

## Requirements

### Requirement: Search across every readable conversation
`GET /api/messages/search?q=&limit=&before=&before_id=&has_file=` (and the narrowing parameters below) SHALL return `{"messages": [...], "has_more": bool}`:
the live (not deleted) messages whose text matches `q`, newest first, in the conversations the caller reads in the
workspace of their session (tenant from the token) and in their direct messages of that workspace. Main-stream messages and thread
replies are both searched; a reply carries `parent_message_id`, the root of its thread. A page holds 25 messages
unless `limit` (at most 50) asks for another size. `before` (RFC 3339 send time) and `before_id` (id of the last message of
the previous page) continue after that message; messages sent at the same instant are ordered by id, so a page
neither repeats nor skips one. A malformed `before` or non-numeric `limit` is 400; the route needs a session (401
without). Hits carry what the mentions feed carries: sender name, reactions, attachments and the caller's thread state.

#### Scenario: A word in a reply
- **WHEN** a member searches for a word that only a thread reply contains
- **THEN** the reply is returned with `parent_message_id` set to the thread's root message

#### Scenario: Paging
- **WHEN** more than one page matches and the client sends `before` and `before_id` of the last message it has
- **THEN** the next page follows without repeating or skipping a message, and `has_more` is false on the last

### Requirement: Short queries find nothing
A `q` that is missing, empty or shorter than 2 characters once trimmed, or that holds no letter or digit, SHALL answer
an empty list (200), without reading messages.

#### Scenario: One letter
- **WHEN** the caller searches for `n`
- **THEN** the answer is `{"messages": []}`

### Requirement: Attachment filter
With `has_file=true` only messages that carry at least one attachment (or the legacy single drive-file link) SHALL be
returned; any other value leaves the filter off.

#### Scenario: Files only
- **WHEN** two messages match and one has a file
- **THEN** `has_file=true` returns only that one

### Requirement: Search follows read access
A message SHALL be returned only when the caller holds `read` on the content OA of its conversation, decided by the
same batch check as `GET /api/channels/unread`, mentions and activity. Being a recorded member is neither required nor
enough: a stranger, a member whose read was withdrawn, and someone else's direct message SHALL never yield a hit, in
the main stream or in a thread, however exact the words. A conversation of another workspace is not searched. When the
policy service cannot answer, nothing is found.

#### Scenario: Removed from a group
- **WHEN** a member whose read on a group was withdrawn searches for words that group's messages and replies contain
- **THEN** none of that group's messages is returned

#### Scenario: Someone else's direct message
- **WHEN** a member searches for words that only a direct message between other people contains
- **THEN** nothing is returned; the same direct message is found by a participant who may read it

#### Scenario: Policy service down
- **WHEN** the policy service cannot be reached
- **THEN** the answer is an empty list

### Requirement: What is matched
Matching SHALL use the PostgreSQL `simple` text-search configuration (words kept as written, case ignored, no stemming
and no stop words) over the message's readable text with any `<...>` tag removed, held in the generated column
`messages.search_vector` (GIN index `idx_messages_search`). Vietnamese words match with their diacritics. The tag names
of a formatted message are not words: searching `strong` does not find `<strong>kế hoạch</strong>`. A deleted message is
never found. All words of `q` must occur (any order); the last word, when `q` does not end in a space or punctuation,
matches as a prefix so a word still being typed finds its message — only when it has at least two letters; a one-letter last word must match whole. `q` is read up to its first 200 characters.

#### Scenario: Vietnamese with diacritics
- **WHEN** a member searches for `đối soát`
- **THEN** a message `Báo cáo đối soát tháng 9` is found, whatever its case

#### Scenario: Unfinished word
- **WHEN** the query is `đối so`
- **THEN** the message above is found; `so đối` does not find it (only the last word is a prefix)

#### Scenario: Markup
- **WHEN** a member searches for `strong` and only a formatted message's tags contain it
- **THEN** nothing is found

### Requirement: User input never reaches query syntax
The query SHALL be built so that any input is only words: everything before the last word is read by
`plainto_tsquery`, and the last word, reduced to letters, digits and combining marks, is passed as one quoted prefix
term. Operators and punctuation (`& | ! : * ( ) < > ' \`) act as separators, produce no SQL or tsquery error, and
change nothing in the database.

#### Scenario: Operator characters
- **WHEN** the query is `a & | ! :*` or `'; DROP TABLE messages; --`
- **THEN** the answer is 200 with whatever words match, never an error

### Requirement: Search inside one conversation
`GET /api/channels/:chId/search?q=&limit=` SHALL use the same matching (above), require `read` on the channel
(403 otherwise) and return the best matches first, then the newest, at most 50.

#### Scenario: Same matching in a group
- **WHEN** a member searches a group for `đối so`
- **THEN** the group's message `Báo cáo đối soát` is found, as in the search across conversations

### Requirement: Narrowing a search
`GET /api/messages/search` SHALL accept these further parameters, each optional and combinable with `q`; a message is
returned only when it meets every one given. The client sends them under exactly these names (the only place that maps
them is `messageSearchParams` in `frontend/src/api/messaging.ts`). Until the server honours a parameter it ignores it.
- `from` — user id of the sender.
- `channel_id` — id of one conversation; ignored (nothing found) when the caller does not read it.
- `after`, `before_date` — first and last day (`YYYY-MM-DD`, both inclusive), days of the caller's zone, which travels as
  `tz` (an IANA name such as `Asia/Ho_Chi_Minh`, sent only with a day filter). A malformed day or `tz` is 400.
- `has_file=true` — as above; `has_link=true` — the text holds a link (`http://` or `https://`); `mentions_me=true` — the
  message names the caller. Any other value leaves the filter off.
Narrowing never widens read access: the read check applies first.

#### Scenario: Sender and days
- **WHEN** a member searches with `from` and a one-day range in `Asia/Ho_Chi_Minh`
- **THEN** only that sender's messages sent between 00:00 and 24:00 of that day in that zone are returned

### Requirement: Chat's results page
Enter (or "Xem tất cả kết quả") in the search box of a desktop SHALL open `/channels/search?q=…`, whose address carries the
words and the filters (`q`, `from`, `in`, `since`, `until`, `file`, `link`, `me`: user id, conversation id, two days, three
switches; a malformed value is dropped), so back, forward and reload keep them. The page has a header ("Kết quả tìm
kiếm", ← "Đóng tìm kiếm" back to the conversation it was opened from, "N tin nhắn" or "Hơn N tin nhắn" while another page
exists), the toolbar "Bộ lọc" with the chips **Người gửi · Trong · Ngày · Có tệp · Có liên kết · Nhắc đến tôi**, a block
"Nhóm và người" (at most 3 and 3, desktop only, only while no filter is on) and the messages, newest first, 25 a page,
more as the list is scrolled to its end (a "Tải thêm" button where the browser cannot tell). A chip that lists opens a
popover (a bottom sheet on a phone) of names: Người gửi — "Bạn (name)" first, then the directory the caller may browse;
Trong — the conversations the caller reads; Ngày — Hôm nay, 7 ngày qua, 30 ngày qua, Tuỳ chọn khoảng… (two days, Áp dụng),
Bất kỳ lúc nào. A choice applies at once and replaces the history entry (back still returns to the conversation); the chip
then says the value with a ✕ that removes it; "Xoá bộ lọc" removes every filter. No list asks for or shows an id; an id of
a person or conversation the caller can no longer name is shown as "Người đã rời" / "Cuộc trò chuyện đã đóng".
States: four skeleton cards while the first answer comes; while a changed search is answered the old list stays at half
strength and `aria-busy`; nothing found ("Không có tin nhắn nào khớp “…”. Thử từ khác hoặc viết ngắn hơn.", or "… với các bộ
lọc này." with Xoá bộ lọc); failure ("Chưa tìm được. Kiểm tra kết nối rồi thử lại." with Thử lại); fewer than 2 characters
("Gõ ít nhất 2 ký tự để tìm tin nhắn."). A card shows the conversation, the sender, the time (`09:12`, "Hôm qua 16:05",
"Thứ Năm 10:41", `02/10`), up to three lines around the match with the match marked, and a chip for each file and for the
site of each link; it opens the conversation at that message. Opening a result keeps the words as a recent search. Below
1024px the same address is handed to the phone screen.

#### Scenario: Reload
- **WHEN** the page is reloaded at `/channels/search?q=đối+soát&from=<id>&file=1`
- **THEN** the same results come, "Người gửi" says the sender's name and "Có tệp" is on

#### Scenario: No filters left
- **WHEN** "Xoá bộ lọc" is pressed
- **THEN** every filter key leaves the address, the words stay, and the request carries only `q` and `limit`

### Requirement: The Tìm trong chat screen on a phone
Below 1024px the search pill of Chat opens a full-screen dialog "Tìm trong chat" (from any Chat screen). The pill stays
where it is and turns into the box (☰ becomes ←, the avatar fades, an accent ring fades in, the list behind fades, the
suggestions rise; with reduced motion nothing turns). The box takes focus at once; ←, Esc and the system back gesture
close it and focus returns to the pill. The screen is part of the address of `/channels` (`?search=1`, with the words and
filters of the results page beside it) and owns a history entry: opening pushes one, searching and filtering replace
it, and closing goes back when the screen pushed it, otherwise replaces the address with the same one minus `search` and
the words and filters; an unknown value is dropped and the desktop ignores it. Before anything is typed the screen shows,
as it appears and without waiting for the box's focus, Tìm gần đây (each with its ✕, and Xoá tất cả from two on) and "Trò
chuyện gần đây" (at most 5 of the conversations something was said in lately, newest first, each with who said what and
when); with nothing to show it says "Tìm tin nhắn, người và nhóm." and "Sau khi tìm, lọc theo người gửi, cuộc trò chuyện
hay ngày.". Typed words suggest people, groups and messages at once (`docs/specs/search-suggestions`). Enter or "Xem tất
cả" shows the results page under the pill, with the filter chips as bottom sheets, and puts the keyboard away; using the
box again brings the suggestions back over them. A search is kept when a result is opened or the box is submitted. No id
reaches the screen.

#### Scenario: Back closes the screen
- **WHEN** the screen is open and the person uses the system back gesture
- **THEN** it closes and the Chat list underneath is shown; Chat is not left

#### Scenario: Back and forward
- **WHEN** a search with a filter was made on the screen, back is used and then forward
- **THEN** back closes the screen with no words or filters left in the address, and forward brings the results and the filter back

#### Scenario: Recent searches
- **WHEN** the box is empty
- **THEN** the person's recent searches are listed, a tap puts one in the box, and storage that is blocked leaves the list empty without an error

#### Scenario: Conversations lately
- **WHEN** the screen opens and something was said lately in "Hợp đồng đối tác" and earlier in "Đối soát giao dịch"
- **THEN** "Trò chuyện gần đây" lists them in that order, and a conversation the person can no longer read is not listed

## Known limits
- Search without diacritics does not work: `doi soat` does not find `đối soát`. It would need the `unaccent`
  extension wrapped as an IMMUTABLE function in the generated column; it is out of scope for now.
- No stemming or synonyms: `họp` and `họp,` match, `họp` and `cuộc họp` do not.
- A plain message that quotes `<...>` text has that text left out of the index, as a tag.

## Schema
`data/migrations/051_message_search_simple.sql` rebuilds `messages.search_vector` (and its index) and so rewrites the
`messages` table; see `docs/deployment.md` (Migrations). Rollback SQL is in the migration header.
