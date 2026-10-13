# chat-threads

## Purpose
In-line threads in groups and direct messages, as in Google Chat: every message in the main stream can carry
a thread of replies, and each user follows the threads that concern them and sees how much of each they have
not read. A thread is a root message plus the messages whose parent is that root; threads are not NGAC nodes,
so every check is made on the content OA of the channel the root message lives in.

## Requirements

### Requirement: A reply belongs to a root message of the same channel
Sending a message with a parent SHALL require that the parent exists, lives in the channel the reply is sent
to, and is itself a root message (has no parent). Otherwise the request SHALL be refused with 400 and nothing
is stored: no message, no change to the parent's reply count.

#### Scenario: Reply into another channel's thread
- **WHEN** a member with write on channel A sends a reply whose parent is a message of channel B
- **THEN** the answer is 400 and channel B's message gains no reply

#### Scenario: Reply to a reply
- **WHEN** a reply names another reply as its parent
- **THEN** the answer is 400; replies are made to the thread's root message

### Requirement: Each root message carries the caller's thread summary
The message list of a channel and the thread view SHALL return, for every root message, the number of replies,
the time of the last reply (absent until the first reply), the users of the three most recent distinct repliers
(newest first), whether the caller follows the thread, and how many replies the caller has not read. The summary
of a whole page SHALL be read in one query, however many root messages the page holds.

#### Scenario: Starter sees the replies of others as unread
- **WHEN** B and C reply to a message A sent, and A opens the channel
- **THEN** A's summary of that message shows the replies, B and C as repliers, following, and the replies as unread

### Requirement: Who follows a thread
A user SHALL follow a thread when they started it, when they were mentioned in its root message or in a reply,
and when they reply to it. Following SHALL change only through the user's own actions: replying follows the
thread again after an unfollow; a later reply by someone else SHALL NOT undo an unfollow. Following a thread for
the first time starts with nothing unread.

#### Scenario: Unfollow, then reply
- **WHEN** B unfollows a thread and later replies to it
- **THEN** B follows the thread again and has nothing unread in it

#### Scenario: Others keep replying after an unfollow
- **WHEN** A unfollows the thread A started and B replies afterwards
- **THEN** A still does not follow the thread and it raises no unread for A

### Requirement: Unread replies are counted only in followed threads
A reply SHALL count as unread for a user when it was sent by someone else after the user's read position in the
thread, and only while the user follows the thread. With no read position the thread is read up to its root
message; replying moves the replier's read position to their own reply. Marking a thread read SHALL move the
caller's read position to now and SHALL NOT change whether they follow it.

### Requirement: Thread endpoints require read on the channel
`POST /api/messages/{id}/follow`, `DELETE /api/messages/{id}/follow`, `POST /api/messages/{id}/thread/read` and
`GET /api/channels/{id}/threads` (and the gRPC `FollowThread`, `UnfollowThread`, `MarkThreadRead`,
`ListThreads`) SHALL require `read` on the channel's content OA, and nothing more: following and reading change
only the caller's own state. A caller without it SHALL get 403 and no state SHALL be written. A message id that
is not a message is 404; a reply's id, or an unknown filter or a malformed `before`, is 400.

#### Scenario: Stranger follows a thread
- **WHEN** a user with no read on the channel follows one of its threads
- **THEN** the answer is 403 and no follow is recorded

### Requirement: Thread changes are live
A reply SHALL update every open view of the channel with the thread's new summary, and each follower's own unread
count, without a refetch; following, unfollowing and reading a thread SHALL update the caller's other sessions. The
delivery rules are in `realtime-event-delivery`.

### Requirement: The thread list of a channel
`GET /api/channels/{id}/threads?filter=all|following|unread&before=&limit=` SHALL return the root messages of the
channel that have at least one reply, most recently active first, each with the caller's thread summary. `following`
keeps the threads the caller follows; `unread` keeps those with unread replies for the caller. `before` (an RFC 3339
last-reply time) pages to older threads; `has_more` says whether another page exists.

### Requirement: Unread counts cover the channels the caller may read
`GET /api/channels/unread` SHALL list exactly the channels the sidebar shows the caller: the channels of the
workspace they are signed in to and the direct messages, each kept only when the caller holds `read` on its
content OA (one batch check). Being a recorded member is not required: a workspace channel the caller can read but
never joined still has its counts. Each entry carries the caller's unread message count, last read message and
`thread_unread_count`: the unread replies of the threads in that channel the caller follows. A channel the caller
may not read SHALL never appear, not even with a zero count; when the policy service cannot answer, no channel is
named.

#### Scenario: Someone else's channel
- **WHEN** a member asks for unread counts in a workspace where some channels are closed to them
- **THEN** the answer lists the channels they can read, joined or not, and none of the others

#### Scenario: Policy service down
- **WHEN** the batch read check fails
- **THEN** the answer is an empty list, never every channel

### Requirement: Followed threads across conversations
`GET /api/threads/followed?limit=&before=&before_id=` SHALL return the threads the caller follows, read or not, across
every conversation the navigator lists for them (the channels of the workspace they are signed in to that they read,
and their direct messages), most recently active first, each with the caller's thread summary — the same shape as
`GET /api/threads/unread`, plus `has_more`. `before` (an RFC 3339 last-reply time) and `before_id` (the root id of the
last thread of the previous page) page to older threads; `limit` is at most 50. A muted conversation still
contributes its followed threads; a thread whose root message was deleted does not. A conversation the caller cannot
read SHALL never contribute a thread, whether they follow it or were removed from it; when the policy service cannot
answer, none is named. Trang chủ's "Đang theo dõi" and the tab Thêm read it.

#### Scenario: Followed and read
- **WHEN** the caller has read every reply of a thread they follow
- **THEN** it is still listed here, and absent from `/api/threads/unread`

#### Scenario: Removed from the group
- **WHEN** the caller follows a thread in a group they are then removed from
- **THEN** the thread is no longer listed

## Screens

The stream and the thread panels (`ConversationStream`, `ThreadSummary`, `ThreadPanel`, `ThreadsPanel`) draw what
the requirements above return; they add the rules below. Layout and motion are in DESIGN.md, "Chat: luồng in-line,
thread, ô soạn" and "Chat trên điện thoại".

### Requirement: The stream opens where the reader stopped
A conversation SHALL show its messages oldest first, grouped by sender within five minutes, and SHALL place a
"Tin mới" divider before the first message from someone else after the reader's read position. The divider is
decided once per visit and stays until the reader leaves or sends a message. When the unread run fits in one screen
the stream opens at its end; when it is longer the stream opens with the divider 48px below its top. Messages older
than the loaded page SHALL load when the reader scrolls to the top, without moving the line being read.

#### Scenario: Long unread run
- **WHEN** a reader with more unread messages than fit on a screen opens the conversation
- **THEN** the stream opens at "Tin mới", and the conversation is not marked read until the reader reaches the end

### Requirement: A thread is read by opening it
Opening a thread panel SHALL mark the thread read once its unread replies are known, and again for any reply that
arrives while it is open. Its "Tin mới" divider SHALL keep the position it had on opening until the panel closes.
Only a followed thread has unread replies, so only a followed thread shows "M mới" on its summary row or in the
thread list.

#### Scenario: Reply to an unfollowed thread
- **WHEN** someone replies to a thread the reader does not follow
- **THEN** its summary row shows the new reply count and no "mới", and nothing is washed

### Requirement: The thread list
The Thread button of a conversation SHALL open its threads, filtered Tất cả / Đang theo dõi / Chưa đọc by the server,
a page at a time (the last thread's reply time and root id are the cursor). Its badge SHALL count the conversation's
threads with unread replies for the reader. Every thread, face and author on these screens is shown by display name;
no identifier is rendered or typed.

### Requirement: Followed threads with news, across conversations
`GET /api/threads/unread?limit=` SHALL return the caller's followed threads that have unread replies, across every
conversation the navigator lists for them (the channels of the workspace they are signed in to that they read, and
their direct messages), newest reply first, at most 50; a conversation set to `off` contributes none. Trang chủ's
"Thread có tin mới" reads it. A conversation the caller cannot read never contributes a thread.

#### Scenario: A thread read is a thread gone
- **WHEN** the caller opens a thread with unread replies and reads it
- **THEN** it no longer appears in the list
