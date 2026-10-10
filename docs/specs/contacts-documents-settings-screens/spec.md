# contacts-documents-settings-screens

## Purpose
The screens for finding people (Danh bạ), for the documents written in the app (Văn bản, a group inside Tài liệu) and for a person's own settings (Cài đặt). Everything is shown by name and in words; no identifier reaches the screen and nothing is chosen by typing one.

## Requirements

### Requirement: Danh bạ lists the workspace's people
Danh bạ SHALL show everyone in the workspace as a hairline table (name with title, department, email, status in words) or as cards, the choice remembered for the signed-in person. It SHALL filter by name, title or email ignoring accents, by department (a chip listing the departments people belong to, with head counts) and by who is online. Choosing a person SHALL open a profile beside the list with their name, role, status, the details that exist (empty ones are left out), the colleagues of the same department, "Nhắn tin" (opens, or creates, the direct message) and "Gửi email". There SHALL be no button for a feature that does not exist (no call).

#### Scenario: A person has no title or location
- **WHEN** their profile opens
- **THEN** those rows are absent rather than shown as dashes

#### Scenario: One's own profile
- **WHEN** the signed-in person opens themselves
- **THEN** there is no "Nhắn tin"

#### Scenario: The directory fails to load
- **WHEN** the directory request errors
- **THEN** the screen says so with "Thử lại"; it never lists members with their names missing

### Requirement: Văn bản is a group of Tài liệu
The list panel of Tài liệu SHALL carry a Văn bản group above the folder tree: Tất cả văn bản, Bản nháp của tôi (with a count) and Được chia sẻ. The group is `?view=texts` (and `&group=drafts|shared`) of `/drive`; the former `/documents` address SHALL land on it, keeping the workspace. The body SHALL be the hairline table of documents (title, owner, status in words, last edit) with "Văn bản mới", a row menu (Mở; Xoá only where the caller may write, after a confirmation that says it cannot be undone) and loading, empty and error states. A document SHALL open at its own address with the list panel hidden.

### Requirement: The editor is a page for one author
A document SHALL open fresh each time. Someone who may only read SHALL see it as text with the reason, without toolbar or title field. Someone who may write SHALL have a toolbar (paragraph style, bold, italic, underline, strike, lists, quote, undo, redo) in which every control works, a status menu (Bản nháp, Đang dùng, Lưu trữ) saved at once, and Ctrl/Cmd+S. A document that cannot be opened (deleted or forbidden, answered alike) SHALL say so and point back to the list.

### Requirement: Cài đặt has three tabs, the open one in the URL
Cài đặt SHALL have Hồ sơ, Workspace and Giao diện, with `?tab=workspace` and `?tab=giao-dien` for the last two and Hồ sơ by default; an unknown value opens Hồ sơ.

#### Scenario: Hồ sơ saves only what changed
- **WHEN** a person changes their title and saves
- **THEN** the request carries only the title, so a field set meanwhile by an administrator is not overwritten, and the new value shows in the sidebar, chat and tables

#### Scenario: Fields that cannot be changed here
- **WHEN** Hồ sơ is shown
- **THEN** email and department are text in a sunk box with the reason beside it; there is no photo upload offered

#### Scenario: Workspace without Quản lý
- **WHEN** a member without `manage` opens the Workspace tab
- **THEN** the name and description are plain text with the reason, there is nothing to save and the manager-only member list is not requested

#### Scenario: Leaving
- **WHEN** a member (manager or not) chooses "Rời workspace" and confirms the consequence
- **THEN** they leave, the cached data of that workspace is dropped, and they land on the workspace picker
- **AND** the last Owner is told to hand over ownership first, the dialog stays, and opening it again starts clean

#### Scenario: Workspace with Quản lý
- **WHEN** a manager changes the name
- **THEN** only the name is sent, the switcher and the sidebar show the new name, and a link points to Quản trị for people, roles and departments

### Requirement: A person is never shown by a username
A missing display name SHALL read "Thành viên" in the directory, the sidebar, the profile and the name field; a username is a login handle.

### Requirement: Leaving a document never loses text silently
Saving SHALL be held while a delete is in flight and carry on if the delete is refused. After the editor has gone, nothing SHALL be rescheduled; text that could not be saved at that point (a conflict, an error, no network) SHALL be reported in a toast, and no save SHALL be attempted while the browser reports it is offline.

### Requirement: Appearance is applied before the first paint
A script in the page SHALL put the stored theme and motion choice on the document before the application loads, reading the signed-in person's own choice and else the device's last; any failure leaves the defaults. `motion-reduce:` styles SHALL follow "Luôn bật" as well as the system setting. The sidebar rail's link names SHALL carry the unread count.

### Requirement: Giao diện is kept on this device for this person
The Giao diện tab SHALL offer the theme (Theo hệ thống, Sáng, Tối) as radio cards that apply at once and without a colour cross-fade, reduced motion (Theo hệ thống, Luôn bật) and a switch to collapse the sidebar to a rail of icons with tooltips. The choices SHALL be stored under the signed-in person in the browser and never sent to the server. Realtime colour SHALL NOT be a setting: it is information (DESIGN.md §7).

#### Scenario: Two people on one browser
- **WHEN** one chooses Tối and another signs in
- **THEN** the second sees their own choice, not the first's

## Status
- Hồ sơ cannot change the photo (no upload exists) and saves Nơi làm việc where the mockup shows a phone number (the profile API has no phone). Department is the profile's free text, not the organisation's department.
- Workspace cannot change the icon and shows no organisation field: neither has a server behind it.
- Presence "đang xem" and live cursors are not drawn: see `text-documents`.
