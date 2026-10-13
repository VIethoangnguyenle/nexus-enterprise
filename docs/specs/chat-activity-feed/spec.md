# chat-activity-feed

## Purpose
What each conversation last said, available the moment the app opens, so the conversation list can show
"Sender: opening of the message" and the latest thread reply without waiting for live events. It is a read-only
summary over messages; it grants nothing and adds no NGAC node or operation. Previews carry message text, so they
follow exactly the read access that unread counts follow.

## Requirements

### Requirement: One entry per readable conversation with a message
`GET /api/channels/activity` SHALL return `{"channels": [...]}`, one entry for every conversation the caller reads
that has at least one live main-stream message, in the workspace of the caller's session (tenant from the token) and
including their direct messages. A conversation with no live message has no entry. Each entry carries `channel_id`,
`latest` and, only when present, `latest_reply`. The result SHALL be read with one statement for the main stream and
one for the replies, however many conversations there are, each costing one ordered index probe per conversation
(`idx_messages_latest_main`, `idx_messages_latest_reply`).

#### Scenario: Cold start of the list
- **WHEN** a member opens the app and asks for activity
- **THEN** every conversation they read that has a message appears with its newest main-stream message, and none without

### Requirement: Latest message and newer latest thread reply
`latest` SHALL be the conversation's newest main-stream message (no parent) that is not deleted; `latest_reply` SHALL
be the newest thread reply that is not deleted, and SHALL be present only when it was sent after `latest`. An edited
message appears with its current text; a deleted message appears nowhere, so the previous one takes its place.

#### Scenario: A reply newer than the last main message
- **WHEN** a thread gets a reply after the conversation's last main-stream message
- **THEN** the entry carries both, and `latest_reply.parent_message_id` names the thread's root message

#### Scenario: A main message newer than the reply
- **WHEN** a main-stream message is sent after the latest thread reply
- **THEN** the entry has `latest` only

#### Scenario: Deleted message
- **WHEN** the newest message of a conversation is deleted
- **THEN** the entry shows the message before it, or no entry if there is none

### Requirement: What a message preview carries
Each of `latest` and `latest_reply` SHALL carry `id`, `sender_id`, `sender_name`, `preview`, `kind`, `mentions` (the
users mentioned), `created_at`, and — on a reply — `parent_message_id`. `preview` is the plain text of the message
(formatting removed, lines joined by a space) of at most 140 characters, counted in characters not bytes; a longer
text is cut at a word boundary when it has one, else at the limit, and ends with an ellipsis within the 140.
`kind` is `attachment` when the message has files (and `attachment_name` is the first file's name), else `poll`,
`task` or `card` (a document or approval) from the object it links, else `text`.

#### Scenario: Formatted message
- **WHEN** the newest message is `<p>xem <strong>kế hoạch</strong></p>`
- **THEN** its preview is `xem kế hoạch`

#### Scenario: Files without text
- **WHEN** the newest message is two files sent without text
- **THEN** its `kind` is `attachment`, `attachment_name` is the first file's name and `preview` is empty

### Requirement: Previews follow read access
A conversation SHALL appear only when the caller holds `read` on its content OA, decided by the same batch check as
`GET /api/channels/unread`, unread threads and mentions. Being a recorded member is neither required nor enough:
a member whose read was withdrawn, a stranger, and someone else's direct message SHALL never appear, with no text of
theirs. When the policy service cannot answer, no conversation is named. The route needs a session (401 without).

#### Scenario: Removed from a group
- **WHEN** a member is removed from a group and asks for activity
- **THEN** the group is not in the answer and none of its message text is returned

#### Scenario: Someone else's direct message
- **WHEN** a member asks for activity while other people's direct messages exist
- **THEN** only the direct messages they may read appear

#### Scenario: Policy service down
- **WHEN** the batch read check fails
- **THEN** the answer is an empty list, never every conversation

### Requirement: Followed threads across conversations
`GET /api/threads/followed?limit=&before=&before_id=` is specified in `chat-threads`; it shares the read-access rule
above. Trang chủ's "Đang theo dõi" and the tab Thêm read it.

### Requirement: Trang chủ and Trực tiếp rows on a phone
Below 1024px Trang chủ (`/channels`) lists every conversation, newest activity first (a thread reply counts), and Trực
tiếp (`?tab=direct`) lists the direct messages the same way. A row shows the avatar (44; thread badge when a reply is
newer than the main message), the name and a relative time ("1 phút", "09:02", "Hôm qua"), "Người gửi: đoạn đầu" of the
newest main-stream message ("Bạn:" for the caller's own; two lines when there is no reply line) and, when `latest_reply`
is present, an indented line "Người trả lời: đoạn đầu" (two lines). A file reads "📎 tên tệp", a poll "Bình chọn: …",
a task "Công việc: …". A person named in the text as "@<display name>" is drawn as a chip, only on the exact name of
someone in `mentions`. The preview of a conversation is the server's answer unless a live event is newer than both
`latest` and `latest_reply`: a live thread reply becomes the reply line, a live main message replaces the main line and
drops an older reply. While the answer has not loaded, or when it failed, rows show what live events carried and the
rest stay without a preview. The answer is asked for again when the socket reconnects, when a message is edited or
deleted, and on pull-down.

Tapping the row opens the conversation; tapping the thread line opens it on that thread (`?thread=<root>`). Unread rows
are bold with a count pill (or "@" when the newest reply names the caller); a muted row is faded, counts nothing and
shows "@" only for a mention. Long-pressing a row opens its sheet, ending with Đánh dấu đã đọc (reads up to the message
the row quotes) when the conversation has unread messages and a message is known. The unread-only pill is remembered on
the device per person and per tab; with nothing unread the tab says "Bạn đã đọc hết". Pulling down from the top reloads
the lists.

#### Scenario: Cold start
- **WHEN** a member opens the app and no live event has arrived
- **THEN** every row with a message shows its preview, and a conversation whose last activity is a thread reply shows
  that reply on its own line and sorts by it

#### Scenario: A live reply
- **WHEN** a reply arrives live in a thread newer than anything the server returned
- **THEN** the row's thread line shows it, the main line keeps the server's message, and the row moves to the top

#### Scenario: Open the thread
- **WHEN** the person taps the thread line of a row
- **THEN** the conversation opens on that thread, not on its stream

### Requirement: The Thread screen on a phone
The Thread button beside the title of Trang chủ shows how many followed threads have unread replies and opens a
full-screen list over the tabs, with the chips Có tin mới (`GET /api/threads/unread`) and Đang theo dõi (this
document's `GET /api/threads/followed`, a page at a time with "Xem thêm thread"). A row names the conversation, the
author and opening of the thread, its replies and last reply, and how many are new; choosing it opens the conversation
on that thread. A thread of a conversation the list does not hold is left out, and an empty chip says so in words.
The screen is part of the address of `/channels` (`?threads=new` for Có tin mới, `?threads=followed` for Đang theo dõi;
any other value is dropped, and the desktop ignores it) and owns a history entry: opening it pushes one, and the system
back gesture or Quay lại closes it back to the tab it covered instead of leaving Chat. When the page was opened on that
address there is no entry of its own to go back to, so closing replaces the address with the same one minus `threads`.

#### Scenario: From Thêm
- **WHEN** the person chooses Thread đang theo dõi in Thêm
- **THEN** the Thread screen opens with Đang theo dõi chosen

#### Scenario: Back closes the screen
- **WHEN** the Thread screen is open and the person uses the system back gesture
- **THEN** the screen closes and the tab it covered is shown; Chat is not left

## Status
Matches code.
