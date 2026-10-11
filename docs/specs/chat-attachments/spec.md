# chat-attachments

## Purpose
Files sent in a conversation the way Google Chat sends them: text and several files in one message, each file kept
in the channel's own drive, so the drive's permissions keep governing who can open it.

## Requirements

### Requirement: Text and files travel as one message
`POST /api/channels/{id}/messages` (and the gRPC `SendMessage`) SHALL accept up to 20 `attachments`, each naming a
file of the channel's drive by id, with the pixel width and height the client measured for an image. The message is
stored with its attachments in one transaction, in the order given; name, type and size are copied from the file.
A message SHALL carry text, files, or both; one with neither is refused (400).

#### Scenario: Three files and a line of text
- **WHEN** a member sends "báo cáo tuần" with two images and a PDF
- **THEN** one message is stored and returned with the three attachments, images with their pixel size

### Requirement: Only a confirmed file of the channel's own drive can be attached
Every attachment SHALL be a confirmed (`active`) file whose chain of parent folders reaches the channel's drive root;
what the file's row claims about its context does not count. A file of another channel, of the workspace drive, an
upload not yet confirmed, a folder, an unknown id or the same file twice SHALL refuse the whole message (400) and
store nothing. Write on the channel is checked first, so a caller without it learns nothing about which files exist.

#### Scenario: Another channel's file
- **WHEN** a writer of channel A attaches a file that lives in channel B's drive
- **THEN** the answer is 400 and no message is stored

### Requirement: An old-style file link is an attachment
A message sent with `linked_entity_type = drive_file` SHALL be treated as an attachment of that file: the same check,
and stored as an attachment, not as a link. A message from before attachments that linked one drive file reads as that
one attachment only when the file is a live file of that message's own channel drive.

### Requirement: Attachments are shown with the message, everywhere it appears
The message list, a thread, the thread list and the live `chat_message` / `thread_reply` frames SHALL carry each
message's attachments. A message from before attachments that linked one drive file (`linked_entity_type =
drive_file`) SHALL read as that one attachment. A deleted message carries none.
