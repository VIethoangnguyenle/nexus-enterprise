# mobile-shell

## Purpose
Gives the installed phone app one way to move between modules and one search per module, in both workspace kinds.

## Status
Specified ahead of code (2026-10-19, plan `plans/261018-1742-mobile-ux-overhaul`, phases 02–03; design DESIGN.md §5
"Điều hướng di động", mockup `design/mockups/mobile-shell.html`).

## Requirements

### Requirement: Every module is one tap away below 1024px
Below 1024px every list screen SHALL show a bottom bar of four modules the person picked plus **Thêm**. Defaults: Chat ·
Lịch · Tài liệu · Phê duyệt in an organization workspace; Chat · Lịch · Tài liệu · Danh bạ in a personal one. The choice is
kept per person, per workspace. Thêm lists every module the workspace has, Thông báo, and Quản trị for those allowed. The
bar is hidden on detail screens, settings pages and while the keyboard is open. There is no ☰ drawer.

#### Scenario: Personal workspace
- **WHEN** a person in a personal workspace opens the bar, Thêm or Sửa thanh đáy
- **THEN** Phê duyệt, Tài sản and Quản trị appear nowhere

#### Scenario: Choice per workspace
- **WHEN** a person puts Tài sản on the bar in workspace A and switches to workspace B
- **THEN** B shows its own choice, and A's is kept

### Requirement: One search per module screen
On a phone each module screen SHALL have one search, the pill at the top; module screens SHALL NOT show a second search
field. Tab and chip rows SHALL scroll horizontally with a visible fade when tabs are hidden, and keep the active tab in
view. Each module's ⋯ menu SHALL start with "Cài đặt <module>" (not in Danh bạ).

#### Scenario: Tài liệu on a phone
- **WHEN** a person opens Tài liệu at 390px
- **THEN** exactly one search control is visible

### Requirement: Chat keeps its sections and its desktop routes
Chat on a phone SHALL offer the chips Tất cả · Mục · Chưa đọc · Được nhắc · Trực tiếp · Nhóm; Mục SHALL keep the behaviour
of `chat-sections`. The desktop routes for mentions and starred stay as they are.

#### Scenario: Old links
- **WHEN** a person opens an old link to a Chat tab
- **THEN** it opens the matching chip on a phone and the same pane on a desktop

### Requirement: Workspace kind is fixed
A workspace's kind (personal or organization) SHALL be fixed when it is created; there is no conversion in either
direction (see `tenant-identity`).

#### Scenario: Converting is not offered
- **WHEN** a person looks for a way to turn a personal workspace into an organization, or the reverse
- **THEN** no such action exists in the app and the conversion route is refused
