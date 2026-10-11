# chat-shortcuts

## Purpose
The navigator shortcuts of Google Chat: "Được nhắc tới" (every message that mentions me) and "Đã gắn sao" (what I
starred — conversations and single messages). Both are personal views over conversations the person can read; neither
is an NGAC node and neither grants anything.

## Requirements

### Requirement: Stars are personal and grant nothing
A star SHALL belong to one user and mark a conversation (`starred_conversations`) or a single message
(`starred_messages`). Starring requires `read` on the conversation's channel (403 otherwise) and is idempotent; so is
unstarring, which also requires `read`. A star never widens access: every list of stars re-checks the graph, so a
person removed from a conversation no longer sees what they starred there. A deleted message cannot be starred (400),
though a star set earlier can be removed. Stars disappear with their channel, message or user.

#### Scenario: A removed member's stars
- **WHEN** a member who starred a message loses `read` on its conversation
- **THEN** the message is gone from their starred list, and returns if they regain `read`

#### Scenario: Starring without read
- **WHEN** a person without `read` stars or unstars a conversation or one of its messages
- **THEN** the answer is 403 and nothing is stored

### Requirement: Starred conversations
`GET /api/channels/unread` SHALL carry `starred` on each item and `GET /api/channels/{id}` SHALL carry `starred`,
both for the caller (omitted when false).

### Requirement: Starred messages feed
`GET /api/messages/starred?before=&before_id=&limit=` SHALL list the messages the caller starred, newest star first,
as `{messages, has_more}`; each message is the usual message payload plus `starred: true` and `starred_at`. Only
messages in conversations the caller can still read — those of the workspace they are signed in to and their direct
messages — appear; deleted messages do not. `before` is the `starred_at` of the last message of the previous page
(RFC 3339) and `before_id` its id, which breaks a tie between stars set at the same instant. `limit` defaults to and
is capped at 50.

#### Scenario: Paging through stars set together
- **WHEN** several messages were starred at the same instant and the list is paged one at a time
- **THEN** every message appears once, ordered by id descending within the instant

### Requirement: Mentions feed
`GET /api/messages/mentions?before=&before_id=&limit=` SHALL list every message that mentions the caller — main
messages and thread replies — sent by someone else, not deleted, in the conversations the caller can read (the same
scope as the starred feed), newest first, as `{messages, has_more}`. Each message carries `unread` (omitted when
false): a main message is unread when sent after the caller's read position in the channel; a reply when sent after
the caller's read position in its thread (the root's creation time when never read). A conversation the caller muted
is included: a mention always counts. A mention dates from the message, except for a person an edit added to it
(see `chat-message-lifecycle`): for them it dates from that edit, so it is unread when the edit came after their read
position even though the message is older; a mention the edit removed is neither listed nor counted. Listing order
stays by the message's `created_at`. `before` is the `created_at` of the last message of the previous page, with
`before_id` as above; `limit` defaults to and is capped at 50.

#### Scenario: A mention in a thread reply
- **WHEN** someone replies in a thread and mentions the caller, who has not read the thread
- **THEN** the reply is listed with `unread`, and after the caller reads the thread it is listed without

#### Scenario: Muted conversation
- **WHEN** the caller set a conversation's notification level to off and is mentioned there
- **THEN** the mention is still listed

#### Scenario: Added to an old message by an edit
- **WHEN** a sender edits a message the caller already read past, so that it now mentions the caller
- **THEN** the message is listed with `unread`, the badge counts it, and after the caller reads past the edit it is
  listed without `unread` and no longer counts

#### Scenario: The mention is removed again
- **WHEN** a later edit removes the caller from the message's mentions
- **THEN** the message is no longer listed and no longer counts

### Requirement: Mentions badge
`GET /api/messages/mentions/unread-count` SHALL answer `{"count": n}`: the number of unread messages of the mentions
feed, counted in one query over exactly the feed's scope (the conversations the caller can read in the workspace they
are signed in to, and their direct messages), with exactly the feed's `unread` rule — thread replies included, muted
conversations included. A conversation the caller can no longer read contributes nothing.

#### Scenario: A removed member's old mentions
- **WHEN** a member who has unread mentions in a conversation loses `read` on it
- **THEN** the count no longer includes them

### Requirement: Stars leave sections alone
Starring or unstarring a conversation SHALL NOT change its place in the caller's chat sections.

### Requirement: API
`PUT|DELETE /api/channels/{id}/star`, `PUT|DELETE /api/messages/{id}/star` (204), `GET /api/messages/starred` and
`GET /api/messages/mentions`, `GET /api/messages/mentions/unread-count`, routed to messaging by the existing `/api/channels` and `/api/messages` rules of the dev
proxy, Docker (Traefik) and production (nginx). The static `starred` and `mentions` paths are not read as a message id.
