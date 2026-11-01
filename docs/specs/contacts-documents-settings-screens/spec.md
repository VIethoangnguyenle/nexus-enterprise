# contacts-documents-settings-screens

## Purpose
The screens for finding people (Danh bạ), for the documents written in the app (Văn bản, a group inside Tài liệu) and for a person's own settings (Cài đặt). Everything is shown by name and in words; no identifier reaches the screen and nothing is chosen by typing one.

## Requirements

### Requirement: Danh bạ is browsed by category
Danh bạ SHALL open on its categories and list nobody until one is opened. An organisation workspace has Phòng ban (the department tree, its root named after the organisation and holding the people in no department), Vai trò (the administrator's roles, with Quản trị viên first and no "Thành viên" entry), Nhóm của tôi (the private chat groups the person is in; absent when there are none; `#general` is not one) and the shortcut Phòng của tôi; a personal workspace has Thành viên (its people, listed at once) and Nhóm của tôi, and nothing about departments or roles. There SHALL be no "Tất cả thành viên" category. Each category SHALL ask for its own data when opened (a group's members when that group is opened) and show skeleton rows of the same shape meanwhile, never a spinner; counts are muted text, not accent pills. On a desktop the categories are the list panel, on a tablet a row of chips, on a phone the first screen of a stack (category, then department, then profile).

#### Scenario: Personal workspace
- **WHEN** a person opens Danh bạ in a personal workspace
- **THEN** the categories are Thành viên and Nhóm của tôi, and no row, profile line or filter mentions a department or a role

#### Scenario: Partial tree failure
- **WHEN** the department tree fails to load but the people did
- **THEN** an inline error under Phòng ban says "Không tải được cây phòng ban. Bạn vẫn tìm được người bằng ô tìm." with "Thử lại", and Vai trò, Nhóm của tôi and the search box keep working

### Requirement: Departments are browsed one level at a time
Opening a department SHALL show its sub-departments first (folder tile, name, head count including everything beneath, "N phòng, nhóm bên dưới" when it has children) and then the people placed directly in it. A breadcrumb SHALL lead from the organisation (a building icon) through every level to the current one, each segment going up to that level; when it is too long the middle segments shrink with "…" and the last is never cut. There is no depth limit. People are placed by their department, never by its name. People rows show the avatar, the name, the title and the status in words ("Trực tuyến", "Ngoại tuyến"); outside the tree (search, a role, a group) the second line is the title and the full department path. People are sorted by given name (the last word), then by full name, the Vietnamese way.

#### Scenario: Same-named departments
- **WHEN** the organisation has a "Đối soát" under Khối Vận hành › Vận hành thanh toán and another under Khối Tài chính
- **THEN** each person appears under the right one in the tree, in the breadcrumb and in their profile path, and search rows tell them apart by the path

#### Scenario: An empty department
- **WHEN** a department has nobody
- **THEN** it says "<Phòng> chưa có ai" with a way back to its parent

### Requirement: Danh bạ search covers everything the person may see
The search box SHALL search names, titles and emails across the person's whole visible scope, not only the open department, ignoring case and accents, highlighting the match; it finds people, not departments. The header SHALL read "Kết quả trong cả <tổ chức>", or "Kết quả trong phạm vi bạn được xem" for a person limited to their own department, with the number of matches; ✕ or Esc returns to where the person was. A chip "Đang trực tuyến" filters the list on screen.

#### Scenario: Nothing matches
- **WHEN** the search finds nobody
- **THEN** the screen says "Không có ai khớp “…”" with "Xoá tìm kiếm"

### Requirement: The profile card acts for real
Choosing a person SHALL open a profile card: a cover in the person's colour, a 64 avatar, name, title, status in words, then "Nhắn tin" (opens, or creates, the direct message), "Gọi" (a one-to-one call) and "Gửi email" (`mailto:`). "Gọi" SHALL always be enabled, whether or not the person is online. The details are Email (with "Sao chép"), Phòng ban as the full path that opens that department, Vai trò (each opens its list) and Nơi làm việc; empty ones are left out, and in a personal workspace there is no Phòng ban, Vai trò or Cùng phòng. "Cùng phòng" shows up to four faces and "<tên> và N người khác", opening the department. There is no phone number. The card is a side panel on desktop, a sheet on a tablet and a full screen with Back on a phone.

#### Scenario: A person has no title or location
- **WHEN** their profile opens
- **THEN** those rows are absent rather than shown as dashes

#### Scenario: One's own profile
- **WHEN** the signed-in person opens themselves
- **THEN** the three actions are replaced by "Sửa hồ sơ", which opens Cài đặt › Tài khoản và hồ sơ

#### Scenario: Calling someone offline
- **WHEN** the person is shown "Ngoại tuyến" and "Gọi" is pressed
- **THEN** the call starts and rings their devices; the button was never disabled

#### Scenario: The directory fails to load
- **WHEN** the directory request errors
- **THEN** the screen says so with "Thử lại"; it never lists members with their names missing

### Requirement: Danh bạ shows only whom the person may see
Danh bạ SHALL show only the departments and people the server returns for the person (decided by NGAC, see `contacts-visibility`); the screen never filters by itself and never says that someone is hidden. A person limited to their own department sees "Chỉ phòng của bạn" under Phòng ban. An administrator sees everything, a hidden department carrying the label "Ẩn khỏi danh bạ".

#### Scenario: Hidden person card
- **WHEN** a person clicks the name or avatar of someone they may no longer see, in chat, an approval, a document or the calendar
- **THEN** a small card shows only that avatar, that name and "Hồ sơ này không còn hiển thị với bạn.", with no title, department, email, "Nhắn tin" or "Gọi"; replying in an existing conversation still works

### Requirement: Danh bạ and Tài liệu suggest before anything is typed
Their search boxes SHALL follow `docs/specs/search-suggestions`. Danh bạ SHALL suggest "Tìm gần đây" and "Nhắn gần
đây": up to 5 people of this workspace's directory the person has direct messages with, latest message first, never
the person themselves; choosing one opens their profile. The direct messages are asked for once the box is focused,
without a live subscription. Tài liệu SHALL suggest "Tìm gần đây", "Sửa gần đây trong <thư mục>" (the files, not
folders, of the open listing, latest edit first) and, outside the shared view, "Được chia sẻ với tôi" (asked for once
the box is focused); choosing a file opens the viewer, among the folder's files when it is one of them and on its own
otherwise. A search is kept on Enter or when a profile or file found by it is opened.

#### Scenario: Someone outside the workspace
- **WHEN** the person has a direct message with someone who is not in this workspace's directory
- **THEN** Danh bạ does not suggest them

### Requirement: Văn bản is a group of Tài liệu
The list panel of Tài liệu SHALL carry a Văn bản group above the folder tree: Tất cả văn bản, Bản nháp của tôi (with a count) and Được chia sẻ. The group is `?view=texts` (and `&group=drafts|shared`) of `/drive`; the former `/documents` address SHALL land on it, keeping the workspace. The body SHALL be the hairline table of documents (title, owner, status in words, last edit) with "Văn bản mới", a row menu (Mở; Xoá only where the caller may write, after a confirmation that says it cannot be undone) and loading, empty and error states. A document SHALL open at its own address with the list panel hidden.

### Requirement: The editor is a page for one author
A document SHALL open fresh each time. Someone who may only read SHALL see it as text with the reason, without toolbar or title field. Someone who may write SHALL have a toolbar (paragraph style, bold, italic, underline, strike, lists, quote, undo, redo) in which every control works, a status menu (Bản nháp, Đang dùng, Lưu trữ) saved at once, and Ctrl/Cmd+S. A document that cannot be opened (deleted or forbidden, answered alike) SHALL say so and point back to the list.

### Requirement: Cài đặt is a list of sections, each at its own address
On a desktop (1024 px and wider) Cài đặt SHALL be a section list beside the open section, at the same addresses as the phone's pages (`/settings/<section>`, see `user-settings`): Tài khoản và hồ sơ (`account`), Thông báo, Giao diện và ngôn ngữ (`appearance`), Thiết bị và phiên, and, in the workspace, Hồ sơ trong tổ chức, one section per module and Cài đặt workspace (`workspace`). `/settings` SHALL open Tài khoản và hồ sơ. The former tabs' addresses SHALL be replaced by their section: `?tab=ho-so` → `account`, `?tab=workspace` → `workspace`, `?tab=giao-dien` → `appearance`, `?tab=thong-bao` → `notifications`, and an unknown value → `account`, keeping the workspace. There are no tabs. Below 1024 px `/settings` is Tôi, which lists the same sections as rows.

#### Scenario: An old tab address
- **WHEN** a person on a desktop opens `/settings?ws=<workspace>&tab=workspace`
- **THEN** the address becomes `/settings/workspace?ws=<workspace>`, Cài đặt workspace is open and is the current item of the list

#### Scenario: Tài khoản và hồ sơ saves only what changed
- **WHEN** a person changes their workplace (or, in a personal workspace, their title) and saves
- **THEN** the request carries only that field, so a field set meanwhile by an administrator is not overwritten, and the new value shows in the sidebar, chat and tables
- **AND** Lưu thay đổi and Huỷ are off until something changes, an empty name is refused when the field is left or saved and nothing is sent, a refusal from the server keeps what was typed, and Huỷ puts the stored values back

#### Scenario: Fields that cannot be changed here
- **WHEN** Tài khoản và hồ sơ is shown on a desktop or a phone
- **THEN** it holds the picture, Tên hiển thị, Nơi làm việc and the email, the email as text with the reason beside it (a sunk box on a desktop); in an organization it shows no Chức danh, Phòng ban or Vai trò, and a personal workspace keeps Chức danh here and has no department or role

#### Scenario: Organization details live only on Hồ sơ trong tổ chức
- **WHEN** a person in an organization opens Hồ sơ trong tổ chức on a desktop or a phone
- **THEN** Chức danh is editable there and saving sends only the title; Phòng ban and Vai trò are read only with the reason that an administrator sets them, and a department nobody assigned reads "Chưa có"

#### Scenario: Chức danh is this organization's
- **WHEN** a person who belongs to two organizations changes Chức danh on Hồ sơ trong tổ chức of one (or in Tài khoản và hồ sơ of a personal workspace)
- **THEN** the save names the workspace open (`PATCH /api/me/profile?workspace=<id>`), the title changes in that workspace only and the other organization keeps its own; an organization where none was set shows the account's default (the title given at sign-up) until one is saved there

#### Scenario: Changing the picture
- **WHEN** a person touches their picture in Tài khoản và hồ sơ (one button around it, no Đổi ảnh button beside it), picks Đổi ảnh or Tải ảnh lên from the menu (Chụp ảnh or Chọn ảnh in the sheet below 768 px) and chooses a JPG or PNG of at most 5 MB
- **THEN** it is uploaded at once (not with Lưu thay đổi), "Đã đổi ảnh" is said, and the new picture shows in Tài khoản và hồ sơ, the sidebar and the account sheet without a reload; elsewhere it shows from the next directory read

#### Scenario: A picture that cannot be used
- **WHEN** the chosen file is not a JPG or PNG, or is over 5 MB
- **THEN** a sentence under the picture says so and nothing is sent; a refusal from the server (`avatar_not_image`, `avatar_too_large`, `avatar_too_many_pixels`, `avatar_busy`, `rate_limited`) is said the same way, in words, never the server's text

#### Scenario: Removing the picture
- **WHEN** a person has a picture and picks Bỏ ảnh from the menu or sheet
- **THEN** it is removed, "Đã bỏ ảnh" is said and the initials show; without a picture the menu offers only Tải ảnh lên

#### Scenario: A picture that does not load
- **WHEN** an avatar's picture fails to load (replaced, removed, or a server that does not serve it)
- **THEN** the initials on the person's colour show instead, never an empty circle

#### Scenario: Workspace without Quản lý
- **WHEN** a member without `manage` opens Cài đặt workspace on a desktop
- **THEN** the name and description are plain text with "Do quản trị viên đặt", there is nothing to save and the manager-only member list is not requested

#### Scenario: Leaving
- **WHEN** a member (manager or not) chooses "Rời workspace" and confirms the consequence
- **THEN** they leave, the cached data of that workspace is dropped, and they land on the workspace picker
- **AND** the last Owner is told to hand over ownership first, the dialog stays, and opening it again starts clean

#### Scenario: Workspace with Quản lý
- **WHEN** a manager changes the name in Cài đặt workspace on a desktop
- **THEN** only the name is sent, "Đã lưu workspace" is said, the switcher and the sidebar show the new name, and the rows Thành viên, Phòng ban and Vai trò và quyền (each with its count) open Quản trị for people, departments and roles

### Requirement: A person is never shown by a username
A missing display name SHALL read "Thành viên" in the directory, the sidebar, the profile and the name field; a username is a login handle.

### Requirement: Leaving a document never loses text silently
Saving SHALL be held while a delete is in flight and carry on if the delete is refused. After the editor has gone, nothing SHALL be rescheduled; text that could not be saved at that point (a conflict, an error, no network) SHALL be reported in a toast, and no save SHALL be attempted while the browser reports it is offline.

### Requirement: Appearance is applied before the first paint
A script in the page SHALL put the stored theme and motion choice on the document before the application loads, reading the signed-in person's own choice and else the device's last; any failure leaves the defaults. `motion-reduce:` styles SHALL follow "Luôn bật" as well as the system setting. The sidebar rail's link names SHALL carry the unread count.

### Requirement: Giao diện is kept on this device for this person
Giao diện và ngôn ngữ on a desktop (`/settings/appearance`) SHALL offer the theme (Theo hệ thống, Sáng, Tối) as radio cards that apply at once and without a colour cross-fade, reduced motion (Theo hệ thống, Luôn bật) and a switch to collapse the sidebar to a rail of icons with tooltips. The choices SHALL be stored under the signed-in person in the browser and never sent to the server. Realtime colour SHALL NOT be a setting: it is information (DESIGN.md §7).

#### Scenario: Two people on one browser
- **WHEN** one chooses Tối and another signs in
- **THEN** the second sees their own choice, not the first's

## Status
- Danh bạ (categories, department browsing, scope-wide search, profile card with Gọi, hidden person card) is built on desktop, tablet and phone (design approved 2026-10-17, `design/mockups/contacts-lark.html`). The place is in the address: `?cat=` (`org`, `roles`, `groups`, `members`; `search` is the phone's search screen), the open place (`dept`, `role`, `chat` for a group), the open profile (`person`) and the search (`q`); a phone has one history entry per level, so the system back climbs the stack one level at a time. "Gọi" starts a voice call (the call screen offers the camera). On a phone the top bar keeps its hint "Tìm người", and a group's screen has "Mở nhóm" at the end of its bar.
- Hồ sơ has no crop or zoom for a picture: the centre square is kept. There is no larger view of a picture yet; the 1024 px size is stored for it. An invitation and the "converted by" line name a person without an id, so they show initials only.
- Hồ sơ saves Nơi làm việc where the mockup shows a phone number (the profile API has no phone). Department is the profile's free text, not the organisation's department.
- Desktop Cài đặt replaced its tabs (Hồ sơ, Workspace, Giao diện, Thông báo) with the section list on 2026-11-01 (plan `plans/261101-0200-desktop-settings-parity`, mockup `design/mockups/settings-desktop.html`).
- Cài đặt workspace cannot change the icon and shows no organisation field: neither has a server behind it.
- Presence "đang xem" and live cursors are not drawn: see `text-documents`.

### Requirement: The legacy document routes pass the drive's refusals through
`GET /api/workspaces/{id}/documents`, `POST .../documents/upload-url`, `POST /api/documents/{id}/confirm` and `GET /api/documents/{id}/download-url` proxy to the drive service and SHALL answer with the drive's own refusal: 404, 403, 400, 401 and 409 keep their status and message. Only a failure that is the drive's own (an internal error, an unreachable service) is a 500, and then it carries `internal error` and a request ID, never the drive's text.

#### Scenario: A file not pending
- **WHEN** the drive refuses a confirmation because the file is no longer pending
- **THEN** the proxy answers 409, not 500

## Planned change (mobile-ux-overhaul, 2026-10-19)
Phase 05: on phones settings move to the Tôi page (Tài khoản / Trong workspace này) and per-module settings pages; the workspace conversion block of Cài đặt → Workspace is removed. See `user-settings`. Plan: `plans/261018-1742-mobile-ux-overhaul`.
