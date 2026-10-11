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

An edit MAY carry `mentions` (user ids, as a send does): the mention list after the edit, replacing the stored one;
absent, the list is left as it was, and `[]` clears it. Only the people the edit **adds** — mentioned now and not
before — are notified: a `mention` push (see `web-push`), and, when the message is a reply, they follow its thread
(a root's mentioned already follow it). People already mentioned are not notified again, and removing a mention
notifies nobody. The mention push runs after the edit has answered.

A person an edit adds also has an **unread mention** from the time of the edit — in the mentions feed and badge
(`chat-shortcuts`) and in `mention_count` of `GET /api/channels/unread` (`notifications`) — until they read past it,
although the message is older than their read position. The time is kept per person and message
(`message_mention_times`); a person mentioned when the message was sent has none, and their mention dates from the
message. A later edit that removes the mention drops it; one that adds the person back dates it afresh. A deleted
message never counts. Where the unread mention appears follows the existing rules: a main message against the
channel read position, a reply against its thread's (and only main messages in `mention_count`), whatever the
conversation's notification level.

#### Scenario: Someone else's message
- **WHEN** a member with write on the channel edits a message another member sent
- **THEN** the answer is 403 and the text is unchanged

#### Scenario: An edit adds a mention
- **WHEN** a sender edits their message so it mentions Lan, who was not mentioned before, and Minh, who was
- **THEN** only Lan is notified

#### Scenario: An edit adds an unread mention
- **WHEN** a sender edits a message that Lan has already read past, so that it mentions Lan
- **THEN** Lan's mention badge, the message's `unread` in her mentions feed and the channel's `mention_count` count it,
  until she reads past the edit

#### Scenario: A later edit removes the mention
- **WHEN** the sender edits the message again without Lan
- **THEN** it no longer counts for Lan and is not in her mentions feed

#### Scenario: An edit mentions someone who cannot read
- **WHEN** an edit adds a mention of a user without `read` on the channel
- **THEN** the mention is stored and that user receives no push

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

That refusal SHALL carry the machine-readable reason `quoted_message_unavailable` in its body
(`{"message": …, "reason": "quoted_message_unavailable"}`), and no other refusal of a send SHALL carry it, so a client
tells a quote that is gone from any other bad request without reading prose. The reason is the same whether the message
was deleted, never existed, or lives in another channel: the answer must not reveal what another channel holds. It is
judged after the caller's `write` on the channel, so a caller without write gets 403 and no reason.

#### Scenario: The quoted message was deleted
- **WHEN** a send names a message of this channel that has been deleted
- **THEN** the answer is 400 with reason `quoted_message_unavailable` and nothing is stored

#### Scenario: The quoted message is in another channel
- **WHEN** a send names a message of another channel
- **THEN** the answer is 400 with the same reason, and the body does not contain that message's text

#### Scenario: Another reason for a bad request
- **WHEN** a send that quotes a live message has no text and no file
- **THEN** the answer is 400 without the reason

#### Scenario: No write access
- **WHEN** a caller without `write` on the channel sends a message quoting a deleted message
- **THEN** the answer is 403 without the reason

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

### Requirement: A quote waits in the composer of the conversation or thread it will be sent in
"Trích dẫn khi trả lời" SHALL set the pending quote of the composer the message is answered in: the conversation's for a
message of the stream, the thread's for a reply, and, inside the thread panel, the thread's for the thread root too. A
quote and the text typed beside it are kept per conversation and per thread, for the session in memory only: they
survive leaving the conversation and coming back, and they are dropped when the person signs out. Sending takes the
quote off the composer at once, so a second message sent before the first is answered does not repeat it. When the send
fails the text and the quote come back, unless something new was typed or quoted meanwhile; the quote then shows as
"Tin nhắn đã bị xoá" (the message is sent without it) only when the server answered 400 with the reason
`quoted_message_unavailable` — any other failure leaves a quote that is still good. A quote taken from a message deleted
while it was pending shows the same way. A card posted as a message (a document or an approval request) never carries a
quote. On a phone, a composer that is editing a sent message takes no quote until the edit is saved or cancelled.

The text leaves the draft when the send starts, not when the server answers: a composer closed before the answer (another
conversation, a closed thread panel) comes back empty. If the send fails, the text returns to the composer it was typed
in, and only when that composer holds no newer text.

#### Scenario: Sending from a thread panel while quoting its root
- **WHEN** a reader chooses "Trích dẫn khi trả lời" on the thread root inside the thread panel
- **THEN** the quote appears in the thread's reply box, not in the conversation's

#### Scenario: A send fails for another reason
- **WHEN** a message with a pending quote is refused with 400 because of a bad attachment
- **THEN** the text and the quote return to the composer and the quote is not marked deleted

### Requirement: A thread stays unread after "Đánh dấu chưa đọc" until its panel is left
When a reader marks a message of an open thread unread, the thread panel SHALL NOT read the thread again as part of the
same visit; the next time the thread is opened it is read as usual. A failed mark-unread holds nothing.

### Requirement: An edit sends the whole mention list
The client SHALL send, with an edit of a message written as rich text, the full list of people mentioned in the edited
text (`[]` when it mentions nobody), and the `message_edited` frame SHALL carry the whole list after the edit, so
that a viewer holding the message replaces its mentions with it, an empty list included. A viewer's "Được nhắc tới" count
SHALL be asked for again when an edit adds or removes the viewer, when a message that named the viewer is deleted (or may
have, when it is not held), when a conversation or a thread is read here or on another device, and when the unread
counts change.

#### Scenario: An edit removes every mention
- **WHEN** a sender edits a message so that it mentions nobody
- **THEN** every viewer who holds it shows no mention on it, without reloading

