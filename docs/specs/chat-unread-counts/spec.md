# chat-unread-counts

## Purpose
How a conversation's unread main messages are counted, and how a person's counts stay current without asking the
server after every message, at a cost that does not grow with the length of a conversation's history or with the
number of messages on the platform.

## Requirements

### Requirement: Main messages are numbered in their conversation
Every main message SHALL be given the next number of its conversation (1, 2, 3…) when it is stored, in the same
transaction, so a number is never given twice and never skipped (a rolled-back send gives its number back). A reply
is numbered instead in its thread. Messages are numbered by the database (`data/migrations/072`), so every path that
stores a message numbers it. The same announcement stored twice (`InsertMessageOnce`) is one message with one number.

#### Scenario: Two people send at the same moment
- **WHEN** fifty messages are sent to one conversation at once
- **THEN** they are numbered 1 to 50, each once

### Requirement: The unread count is a subtraction
A person's unread main messages in a conversation SHALL be the conversation's last number, less the number they have
read up to, less the deleted main messages numbered after it; never below zero; 0 under the `off` level. Reading a
conversation (`POST /api/channels/{id}/read`) moves the read number to the conversation's last number at that moment;
marking a message unread moves it to just before that message (`chat-message-lifecycle`). A sender who had read up to
the message before theirs has read their own message; one still behind counts it with the rest. A conversation never
read counts every live main message. An unread mention is a mentioning main message numbered after the read number,
or one an edit made to mention the reader after their read time. Replies and thread counts are as `chat-threads`
and `notifications` specify.

A write that moves only the read time (code of the release before the numbers, which keeps serving during a deploy and
again after a rollback) SHALL bring the read number with it: the number of the last main message at or before that
time. A message stored twice under one id takes no second number.

#### Scenario: An unread message is deleted
- **WHEN** someone has two unread messages and one of them is deleted
- **THEN** their count is 1; deleting a message they had already read changes nothing

### Requirement: The counts carry their numbers
Each entry of `GET /api/channels/unread` (and of the chat state) SHALL carry `last_seq`, the conversation's last main
number when counted, and `last_read_seq`, the caller's read number, so a client that hears of later messages can count
on from them.

### Requirement: Members hear of a conversation's activity
Every main message sent, reply sent, or message deleted SHALL be told as one `conversation_activity` frame to every
session of every member of the conversation (its `channel_members`), whether or not the conversation is open, and to
no one else: not to a person of the workspace who is not a member, not to the same person signed in to another
workspace. A person removed from the conversation, or who left its workspace, SHALL stop hearing of it at once. The
frame carries the conversation id, the kind (`sent`, `deleted`), the main number (0 for a reply), the thread root of a
reply, the sender's id and the ids of the people mentioned — never any text. One frame per message goes over Redis,
to the conversation's workspace; each instance hands it on from the members it holds in memory (read from the
directory at most every 30 seconds; a list read while someone was being removed is not kept). Publishing never
reads the database on the sender's request. `MESSAGING_ACTIVITY_EVENTS=0` turns the frames off.

Members are those of the `channel_members` cache that messaging keeps beside the graph, as push notifications are: a
frame names who spoke and who was mentioned, never what was said, which is read only through the conversation and its
`read` check. Where the cache outlives a membership (a removal whose cache delete failed, a change made to the graph
outside messaging), the person keeps hearing of activity until the cache row goes.

#### Scenario: Someone removed from a group
- **WHEN** a person is removed from a group and a message is then sent to it
- **THEN** none of their sessions receives a frame for it

### Requirement: The client counts from the frames
A client SHALL keep its unread counts from these frames without asking the server:
- a main message from someone else, in a conversation not on screen, adds one (none under `off`), and one mention when
  it names the reader, at every level;
- the reader's own message, or one in the conversation on screen, adds nothing;
- a deleted main message numbered after the reader's read number takes one off;
- a repeated frame (a number already counted) changes nothing.

It SHALL ask for its counts again when it cannot follow: a number skipped (a frame that never arrived, or counts held
without a number), a conversation
its list does not know, a reply in a conversation set to hear every reply (`all`), or a deletion while a mention is
shown. A message arriving in the open conversation no longer asks for the counts.

#### Scenario: A frame is lost
- **WHEN** the client knows number 4 and hears of number 6
- **THEN** it counts 6 and asks the server for its counts once

### Requirement: Sessions dropped together come back spread out
After a dropped socket, a client SHALL wait a random time spread over the whole backoff step before reconnecting:
up to 5 seconds the first time, doubling each time up to 30 seconds, never less than 250 ms. A deploy that drops every
socket at once then sees them return over seconds, not together.
