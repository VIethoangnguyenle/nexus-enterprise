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
