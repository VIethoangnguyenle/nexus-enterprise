# chat-channel-integrity

## Purpose
What the messaging service guarantees about the things that hang off a channel (messages, pins, polls, votes) and about the difference between "not there" and "could not be read".

## Requirements

### Requirement: An unknown channel or message is not found
A request that names a channel or message that does not exist SHALL answer 404 (gRPC `NotFound`) with the fixed words `channel not found` / `message not found`, before the policy service is asked anything. A lookup that fails for a reason of the server's SHALL answer a generic 500 and SHALL NOT be reported as "not found", whether the thing looked up is a channel, a message, a poll or a task. A malformed `before` cursor on the message list is 400, not an empty page.

#### Scenario: Unknown channel
- **WHEN** a signed-in member opens `GET /api/channels/{id}` for an id that is not a channel
- **THEN** the answer is 404 `channel not found`

#### Scenario: Denied is not "not found"
- **WHEN** the caller holds no read on an existing channel and asks for it over gRPC or REST
- **THEN** the answer is `PermissionDenied` / 403, never `NotFound`

#### Scenario: Database unreachable
- **WHEN** the channel lookup itself fails
- **THEN** the answer is a generic 500, not 404

### Requirement: Members and reactions are not silently shortened
Listing a channel's members SHALL fail when the policy service cannot answer, rather than return an empty room. Loading the reactions or pin state of a page of messages SHALL fail the request when the database cannot answer, rather than send the page without them. A member whose display name cannot be looked up is still listed, under the name the graph holds.

### Requirement: A pin names a message of its own channel
`POST /api/channels/{id}/pins` SHALL require write on the channel and SHALL refuse (400) a message that is not in that channel. A listing of pins SHALL show a pin only when its message is in the channel being listed, so that a pin can never carry another channel's text to this channel's readers.

#### Scenario: Pinning another channel's message
- **WHEN** a member with write on channel A pins the id of a message that lives in channel B
- **THEN** the answer is 400, nothing is pinned, and channel B's text is not shown in A's pins

### Requirement: A vote names an option of its own poll
`POST /api/polls/{id}/vote` SHALL require write on the poll's channel and SHALL refuse (400) an option that does not belong to that poll, so a vote never lands in another channel's tally.

#### Scenario: Voting with another poll's option
- **WHEN** a member authorised on poll A sends the option id of poll B
- **THEN** the answer is 400 and poll B's counts are unchanged
