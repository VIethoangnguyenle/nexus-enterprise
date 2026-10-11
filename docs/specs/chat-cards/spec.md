# chat-cards

## Purpose
Polls, tasks, documents and approval requests created from the chat composer, each shown as a card in the
conversation. Cards are not NGAC nodes: polls and tasks are checked on the channel's content OA; a document or approval
card shows its details only through its own service, under the viewer's own access.

## Requirements

### Requirement: Creating a card needs write on the channel
Creating a poll or a task SHALL require `write` on the channel (403 otherwise).

### Requirement: Polls
A poll SHALL ask a non-empty question with 2–10 non-empty choices, single- or multiple-choice, optionally anonymous,
optionally ending at a future time (`ends_at`, RFC 3339; 400 otherwise). Voting and withdrawing a vote SHALL need `read`
on the channel; in a single-choice poll a vote replaces the voter's previous one. A poll closed early or past its end
takes no votes or withdrawals (400). It SHALL be closed early (`POST /api/polls/{id}/close`) only by its creator with
`write`, or by someone with `manage` on the channel (403 otherwise).

Each viewer sees a poll as it applies to them: their own choices, and the counts, totals and voters only once they have
voted or the poll is closed — until then those are left out of the answer. An anonymous poll never names its voters,
not even to its creator. A change to a poll reaches the channel's viewers as a frame naming the poll only, never its
counts; each client refetches the poll under its own view.

#### Scenario: Looking before voting
- **WHEN** a member opens an open poll they have not voted in
- **THEN** they see the question and the choices, without counts or voters

#### Scenario: Closing someone else's poll
- **WHEN** a member without manage closes a poll another member created
- **THEN** the answer is 403 and the poll stays open

### Requirement: Tasks
A task's assignee SHALL be someone who reads the channel (400 otherwise). Changing a task's status (done, reopened)
SHALL be for its assignee, its creator, or someone with `manage` on the channel; any other writer is refused (403).
The assignee and the creator need only `read` to change the status, so a read-only assignee can mark their own task
done. Handing a task to another person SHALL be for its creator or someone with `manage` (403 otherwise), and the new
assignee must read the channel (400) — otherwise a writer could make themselves the assignee and then mark the task
done. Changing the title or due date SHALL require `write`.

### Requirement: Document and approval cards
A message MAY link a document or an approval request (`linked_entity_type` `document` / `approval`). Its card's details
are fetched from the owning service under the viewer's own authorization, so a viewer without access to the object sees
no details. Approval cards are not offered in a personal workspace, and the server refuses one there: sending a
message with `linked_entity_type` `approval` in a personal workspace fails with 403 and stores nothing (a direct message is
judged by the workspace the sender is signed in to; an unknown workspace is refused). See `tenant-identity`.
