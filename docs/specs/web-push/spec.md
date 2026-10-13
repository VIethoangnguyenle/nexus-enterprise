# web-push

## Purpose
Notifications on a person's devices, through OneSignal, when a chat message concerns them and they are not looking
at it — without ever telling someone about a conversation they can no longer read.

## Requirements

### Requirement: Who a message is pushed to
A stored chat message (not a system line) SHALL be pushed to: the other side of a direct message (`direct`), the
users it mentions (`mention`), and the followers of the thread it replies in (`thread_reply`); never to its sender.
Someone concerned for several reasons is pushed once, as the strongest: mention, then direct, then thread reply.

A message edited to mention people it did not mention before SHALL be pushed as a `mention` to those people only
(`chat-message-lifecycle`): not to anyone already mentioned, not to the direct-message or thread-follower audience,
and not for a removed mention. The same recipient checks apply, and the same limit of 20.

### Requirement: Every recipient is checked when the push is sent
A recipient SHALL be pushed only when, at that moment, they hold `read` on the channel's content OA (a removed member
or a mentioned non-member gets nothing; a policy error sends nothing), they have that kind of push on, the time is
outside their quiet hours (in their time zone; a window may wrap midnight), and none of their live sessions has the
conversation on screen. A session says what it shows with a `view` frame on the WebSocket (the conversation open while
the page is visible; empty when hidden or none), accepted only for a conversation it is subscribed to (403 otherwise).
Being subscribed is not viewing: the navigator subscribes to every listed conversation, and a tab merely left open must
not hold back pushes to the person's phone.

#### Scenario: Mentioned without access
- **WHEN** a member mentions someone who cannot read the channel
- **THEN** that person receives no push

#### Scenario: Looking at it already
- **WHEN** a direct message arrives while its recipient has the conversation open
- **THEN** no push is sent

### Requirement: Content and grouping
A push SHALL be titled by the sender (direct message) or the group's name (group, with the sender before the text),
carry the opening of the message's plain text (140 characters at most; "📎 Tệp đính kèm" for files alone), and open the
conversation — or the thread — in the app: the link of a thread reply carries `?thread=<root id>&message=<reply id>`, that of a
mention in the main stream `?message=<message id>`, so the app brings that message into view and outlines it. The first message of a conversation is pushed at once; further messages
of that conversation to the same person within 30 seconds become one follow-up, "N tin mới", that replaces it on the
device (`web_push_topic` on the web, `collapse_id` on mobile). The link is absolute, under `APP_BASE_URL`. Every
request carries an `idempotency_key` kept across its retries, so a retry after a timeout does not deliver twice.
The recipient's notification level for the conversation (`docs/specs/notifications/spec.md`) shapes this: under
`all` every message of a group is pushed to them (no per-kind switch applies — the level is the wish); under `off`
nothing is pushed, except a mention in a group; a direct message under `off` is silent, mention or not. Quiet hours
and the read check still apply to all of it.
One message (or one edit) pushes to at most 20 mentioned people (the mention list comes from the client), and the delivery of one
message ends with its 30-second deadline, however many recipients remain.

#### Scenario: A reply or a mention names its message
- **WHEN** a thread reply is pushed to a follower, or a mention in the main stream is pushed
- **THEN** the link opens the thread with `message` set to the reply, or the conversation with `message` set to the mention; a plain group or direct message links to the conversation alone

#### Scenario: A tab left open elsewhere
- **WHEN** a person has Nexus open on a desktop, on another conversation or hidden, and a direct message arrives
- **THEN** the push is sent

### Requirement: Preferences
`GET|PUT /api/me/push-preferences` SHALL read and store the caller's own preferences: each kind on or off, and quiet
hours (`HH:MM` start and end, time zone). Without stored preferences every kind is on and there are no quiet hours.
Invalid hours or an unknown time zone are refused (400).

### Requirement: Device switch
Settings › Notifications shows a per-device push switch whose state is read from the OneSignal SDK. The SDK is
initialised at most once per page; a caller that stops waiting (20 s) leaves that init running and a later call
receives its result. Turning the switch on while permission is undecided asks for permission inside the same tap.
The worker OneSignal registers is `/OneSignalSDKWorker.js`, which runs the app worker (`/sw.js`); both are served
`no-cache`, so a device gets one worker for the offline shell and for push.

#### Scenario: Turning push on
- **WHEN** a person turns the switch on and allows notifications
- **THEN** the worker registers and the device is subscribed on OneSignal

#### Scenario: The notification service cannot be reached
- **WHEN** the SDK fails to load or does not finish starting while the switch is being read
- **THEN** the switch is disabled with "Chưa kết nối được dịch vụ thông báo" and a "Thử lại" action, never an endless "Đang kiểm tra…"; "Thử lại" reads the state again and succeeds once the SDK has started

### Requirement: Configuration
With neither `ONESIGNAL_APP_ID` nor `ONESIGNAL_REST_API_KEY` set nothing is sent; with only one set the service
refuses to start. The REST API key is never logged, and neither is a message's text; a failed push is logged with the
number of recipients and the error only.
