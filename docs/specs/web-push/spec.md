# web-push

## Purpose
Notifications on a person's devices, through OneSignal, when a chat message concerns them and they are not looking
at it — without ever telling someone about a conversation they can no longer read.

## Requirements

### Requirement: Who a message is pushed to
A stored chat message (not a system line) SHALL be pushed to: the other side of a direct message (`direct`), the
users it mentions (`mention`), and the followers of the thread it replies in (`thread_reply`); never to its sender.
Someone concerned for several reasons is pushed once, as the strongest: mention, then direct, then thread reply.

### Requirement: Every recipient is checked when the push is sent
A recipient SHALL be pushed only when, at that moment, they hold `read` on the channel's content OA (a removed member
or a mentioned non-member gets nothing; a policy error sends nothing), they have that kind of push on, the time is
outside their quiet hours (in their time zone; a window may wrap midnight), and none of their live sessions has the
conversation open.

#### Scenario: Mentioned without access
- **WHEN** a member mentions someone who cannot read the channel
- **THEN** that person receives no push

#### Scenario: Looking at it already
- **WHEN** a direct message arrives while its recipient has the conversation open
- **THEN** no push is sent

### Requirement: Content and grouping
A push SHALL be titled by the sender (direct message) or the group's name (group, with the sender before the text),
carry the opening of the message's plain text (140 characters at most; "📎 Tệp đính kèm" for files alone), and open the
conversation — or the thread — in the app. The first message of a conversation is pushed at once; further messages
of that conversation to the same person within 30 seconds become one follow-up, "N tin mới", that replaces it on the
device.

### Requirement: Preferences
`GET|PUT /api/me/push-preferences` SHALL read and store the caller's own preferences: each kind on or off, and quiet
hours (`HH:MM` start and end, time zone). Without stored preferences every kind is on and there are no quiet hours.
Invalid hours or an unknown time zone are refused (400).

### Requirement: Configuration
With neither `ONESIGNAL_APP_ID` nor `ONESIGNAL_REST_API_KEY` set nothing is sent; with only one set the service
refuses to start. The REST API key is never logged, and neither is a message's text; a failed push is logged with the
number of recipients and the error only.
