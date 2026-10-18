# conversation-settings

## Purpose
Gives each group and each direct message a settings page reached from its name, with what a person needs to manage their
own view of it and, in groups, its members.

## Status
Specified ahead of code (2026-10-19, plan `plans/261018-1742-mobile-ux-overhaul`, phase 06; design DESIGN.md §6 "Cài đặt
cuộc trò chuyện").

## Requirements

### Requirement: The conversation's name opens its settings
On a phone, tapping the conversation's name in its top bar SHALL open its settings; on a desktop the same content is the
Thông tin panel. Group: picture, name, description, the quick actions Tìm, Thông báo, Gắn sao and Thêm người, members,
files/images/links, pinned messages, notification level, section, Quản lý nhóm (managers only, `chat-group-management`),
Rời nhóm. Direct message: the person, the quick actions Hồ sơ, Tìm, Thông báo and Gắn sao, files, pinned messages,
notification level, section, "Tạo nhóm với <tên>". "Gắn sao" is the existing star; there is no pin-to-top of a
conversation, and the star appears once (as the quick action, not again as a row).

#### Scenario: Tapping the name
- **WHEN** a person taps the conversation's name in its top bar on a phone
- **THEN** its settings open, and the top bar keeps only Gọi and Thread

### Requirement: Only permitted actions are offered, and the server decides
Each action SHALL be shown only when the reader may do it, and the server SHALL refuse it regardless. No identifier
appears on screen.

#### Scenario: Member who cannot rename
- **WHEN** a member who may not rename the group opens its settings
- **THEN** no rename action is offered, and a crafted rename request is refused

### Requirement: Moving a conversation into a section uses one picker
Long-press on a row, swipe → Thêm, and the Mục row of the settings SHALL open the same section picker (`chat-sections`).

#### Scenario: Same picker from three places
- **WHEN** a person moves a conversation into a section from a long-press, from swipe → Thêm and from the Mục row
- **THEN** each opens the same picker and the conversation ends up in the chosen section
