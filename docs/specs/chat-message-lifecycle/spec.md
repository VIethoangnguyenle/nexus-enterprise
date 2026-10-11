# chat-message-lifecycle

## Purpose
What can happen to a message after it is sent — edited, deleted, quoted, marked unread — and who may do it.
Messages are not NGAC nodes: every check is on the content OA of the message's channel.

## Requirements

### Requirement: Only its sender edits a message
`PATCH /api/messages/{id}` (gRPC `EditMessage`) SHALL change the text of a live message only for its sender, who must
hold `write` on the channel; anyone else is refused (403), and so is the sender without write. The new text is
filtered like a send (see `chat-rich-text`); an edit to no text is refused (400) — removing the text is a delete; a
card (poll, task…) is not edited as text (400). The message records when it was edited and the channel's viewers
receive a `message_edited` frame.

#### Scenario: Someone else's message
- **WHEN** a member with write on the channel edits a message another member sent
- **THEN** the answer is 403 and the text is unchanged

### Requirement: The sender, or a manager, deletes a message
`DELETE /api/messages/{id}` (gRPC `DeleteMessage`) SHALL delete a live message for its sender holding `write`, or for
anyone holding `manage` on the channel; anyone else is refused (403). Deletion is soft: the row remains, and records
who deleted it, but its body, files, mentions and quote are never returned again. A root message with replies stays
in the stream as a "deleted" placeholder so its thread survives; any other deleted message leaves the stream. A
deleted reply leaves its thread, no longer counts in the thread's reply count or anyone's unread, and the thread's
last reply time moves back to the newest reply left. The channel's viewers receive a `message_deleted` frame that
carries ids only. Deleting (or editing) a message already deleted is 404, including when two requests race. When a
manager deletes someone else's message, the conversation SHALL receive a system line naming who deleted whose message
(display names, never ids); deleting one's own message adds no line. No one can reply to a deleted message (400),
even if the delete lands while the reply is being stored.

#### Scenario: A writer deletes a colleague's message
- **WHEN** a member without manage deletes a message another member sent
- **THEN** the answer is 403 and the message stays

#### Scenario: The root of a thread
- **WHEN** its sender deletes a message that has replies
- **THEN** the message shows as deleted, without its text, and its replies remain

### Requirement: A quote names a message of the same channel
A message MAY quote one live message of the same channel (`quoted_message_id`); any other target is refused (400), so a
quote never carries another channel's text. A message is returned with what its quote shows: the quoted message's
sender name and the opening of its text, or that it was deleted, without text.

### Requirement: Marking a message unread
`POST /api/messages/{id}/unread` (gRPC `MarkMessageUnread`) SHALL, for a caller with `read` on the channel, move their
read position to just before the message: in the channel for a root message, in its thread for a reply, so that
message and everything after it count as unread again. Following the thread does not change.

### Requirement: The client offers only what the server would accept
The conversation SHALL list an action on a message only when the reader may take it: "Sửa tin" on the reader's own text
messages (not cards, files or system lines), "Xoá tin" on their own messages or on any message when they manage the
channel, "Đánh dấu chưa đọc" on other people's messages. Deleting SHALL always be confirmed first, in a dialog that names
the sender when the message is not the reader's, says the group's log records it in that case, and says a thread stays
when the message has one. A deleted root with replies SHALL show as "Tin nhắn đã bị xoá" without files or reactions and
keep its thread summary; any other deleted message SHALL leave the stream for everyone watching, as the
`message_deleted` frame says. An edited message SHALL end with "đã sửa". The log line a manager's deletion leaves SHALL
read as a quiet system line, not a bubble. No message id reaches the screen.

#### Scenario: A manager deletes a colleague's message
- **WHEN** a manager chooses "Xoá tin" on a colleague's message
- **THEN** the dialog names the colleague, and after confirming the message is gone for everyone and a system line says who deleted whose message

