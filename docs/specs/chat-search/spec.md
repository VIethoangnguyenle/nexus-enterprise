# chat-search

## Purpose
Finding a message by its words, in every conversation the caller reads at once (the Search screen of the phone chat)
and inside one conversation. Search is read-only over messages; it grants nothing and adds no NGAC node or operation.
A hit is message text, so it follows exactly the read access that unread counts, mentions and previews follow.

## Requirements

### Requirement: Search across every readable conversation
`GET /api/messages/search?q=&limit=&before=&before_id=&has_file=` SHALL return `{"messages": [...], "has_more": bool}`:
the live (not deleted) messages whose text matches `q`, newest first, in the conversations the caller reads in the
workspace of their session (tenant from the token) and in their direct messages. Main-stream messages and thread
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
matches as a prefix so a word still being typed finds its message.

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

### Requirement: The Tìm trong chat screen on a phone
Below 1024px the search pill of Chat opens a full-screen dialog "Tìm trong chat" (from any Chat screen). The box takes
focus at once; Quay lại, Esc and the system back gesture close it and focus returns to the pill. The screen is part of
the address of `/channels` (`?search=1`) and owns a history entry: opening pushes one, and closing goes back when the
screen pushed it, otherwise replaces the address with the same one minus `search`; an unknown value is dropped and the
desktop ignores it. Chips Tất cả · Người · Nhóm · Tin nhắn · Có tệp filter what is shown. Groups and people are matched
in the browser from the loaded lists, ignoring case, accents and đ ("doi soat" finds "Đối soát giao dịch"), every word
of the query required; the person using the app is never listed. A group opens its conversation; a person opens the
direct message that exists, or creates it. Messages come from `GET /api/messages/search` once typing pauses for 250 ms
and the words reach 2 characters, 25 a page with "Tải thêm"; a row shows the sender (and the group for a group message),
the opening of the text with the words found marked, and the time, and opens the conversation on that message (its
thread when it is a reply). Before anything is typed the screen lists the person's recent searches (at most 8, newest
first, kept per person on the device in `ngac-chat-recent-searches:<user>`, removable one by one); a search is kept when
a result is opened or the box is submitted. No id reaches the screen.

#### Scenario: Group without accents
- **WHEN** a member types `doi soat` on the screen
- **THEN** the group "Đối soát giao dịch" is listed with the words marked, without waiting for the server

#### Scenario: Nothing found
- **WHEN** the words match no message
- **THEN** the screen says "Không tìm thấy “…” — thử từ khác." and, for words typed without accents, "Thử gõ có dấu."

#### Scenario: Search failed
- **WHEN** the server cannot answer
- **THEN** the screen says "Chưa tìm được tin nhắn." and Thử lại asks again

#### Scenario: Back closes the screen
- **WHEN** the screen is open and the person uses the system back gesture
- **THEN** it closes and the Chat list underneath is shown; Chat is not left

#### Scenario: Recent searches
- **WHEN** the box is empty
- **THEN** the person's recent searches are listed, a tap puts one in the box, and storage that is blocked leaves the list empty without an error

## Known limits
- Search without diacritics does not work: `doi soat` does not find `đối soát`. It would need the `unaccent`
  extension wrapped as an IMMUTABLE function in the generated column; it is out of scope for now.
- No stemming or synonyms: `họp` and `họp,` match, `họp` and `cuộc họp` do not.
- A plain message that quotes `<...>` text has that text left out of the index, as a tag.

## Schema
`data/migrations/051_message_search_simple.sql` rebuilds `messages.search_vector` (and its index) and so rewrites the
`messages` table; see `docs/deployment.md` (Migrations). Rollback SQL is in the migration header.
