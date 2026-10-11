# lark-sidebar-layout

## Purpose
Give the workspace a single persistent navigation surface that can yield screen space on demand.

## Status

Updated 2026-10-11. The main menu (`frontend/src/components/patterns/AppSidebar.tsx`) has the two states
below, designed in `design/mockups/app-rail.html`. The global search input is still not in the
sidebar; that requirement stays as written.

## Requirements

### Requirement: Main menu in two states
The workspace SHALL show one persistent main menu on the left at 1024px and wider, a single `<nav aria-label="Điều hướng chính">`, in two states: the **rail** (72px wide: the workspace letter tile, then each destination as an icon with its full label under it, 11px) and the **expanded sidebar** (232px: workspace switcher, then each destination as an icon and label in a row). Both SHALL hold the same destinations in the same order: Tin nhắn, Tài liệu, Phê duyệt, Tài sản, Danh bạ, then at the foot Thông báo, Quản trị, Cài đặt and the signed-in person. A personal workspace SHALL NOT offer Phê duyệt or Tài sản. A destination's count SHALL show as a pill (`99+` above 99, hidden at 0) with the number also in text for screen readers, and the open destination SHALL carry `aria-current="page"`. The **rail is the default** at every desktop width, until the person expands it.

#### Scenario: A first visit
- **WHEN** someone with no stored choice opens the workspace on a desktop
- **THEN** the menu is the 72px rail and every destination shows its full name under its icon

### Requirement: The menu toggles, and remembers
A toggle button SHALL sit alone on the top row of the menu in both states, with `aria-expanded` (true while expanded), `aria-label` "Thu gọn menu chính" or "Mở rộng menu chính", `aria-keyshortcuts="Control+Backslash"` and a tooltip naming the shortcut. **Ctrl+\** (⌘\ on macOS) SHALL do the same from anywhere on the page. The change SHALL fade the content out, change the column width, and fade the content in; with reduced motion it SHALL be immediate. The choice SHALL be remembered per person on that device (`sidebarCollapsed` in the preferences kept in localStorage under the user's id, never sent to the server) and shared by every module.

#### Scenario: Expand and come back
- **WHEN** a person expands the menu with Ctrl+\ and reloads
- **THEN** the menu is expanded again for that person, and still the rail for someone else on the same browser

### Requirement: The rail hides no name
In the rail, the workspace tile and the person's avatar SHALL be buttons: the tile opens the workspace menu (the open workspace in full, wrapping and never cut, with its type line, then the workspaces to switch to and Tạo workspace), the avatar opens an account menu (name, role, status, Cài đặt tài khoản, Đăng xuất). Each SHALL carry its own `aria-label`. A tooltip SHALL appear only where text is hidden (tile, avatar, toggle), after 400ms of hover or at once on keyboard focus, and Esc SHALL hide it; it is never the only name of a control.

### Requirement: Thông báo opens a panel
The Thông báo item SHALL show the person's unread count (`GET /api/notifications/unread-count`) and open a 380px panel beside the menu listing `GET /api/notifications` newest first, 25 a page ("Tải thêm", "Đã hiện X trong Y"), under "Hôm nay" and "Trước đó". Each row SHALL be a link to the request, asset or asset request it concerns (a button when it names none), worded in Vietnamese from its `type` and never from the server's `title` or `body`, and SHALL show no id. Pressing a row, or its "Đánh dấu đã đọc" button, SHALL mark it read at once and "Đánh dấu tất cả đã đọc" SHALL mark all; a refusal puts the rows and the counts back. The panel has loading, empty and error states, closes on Esc, a press outside or Đóng, and returns focus to the item.

#### Scenario: A failed list
- **WHEN** the notification list cannot be loaded
- **THEN** the panel says "Không tải được thông báo" and offers "Thử lại"

### Requirement: Sidebar Global Search
The sidebar SHALL contain a global search input at the top (below workspace switcher), styled with flat borders and a search icon.

#### Scenario: Search input is accessible
- **WHEN** the sidebar is expanded
- **THEN** a global search input SHALL be visible and usable

### Requirement: Phone Navigation And Workspace Switching
Where the sidebar is not shown, a bottom bar SHALL offer three labelled tabs, Tin nhắn, Tài liệu and Phê duyệt (with the number of requests waiting on the person, `99+` above 99, hidden at 0), and a fourth, Thêm, that opens a modal bottom sheet holding the current workspace and person, Tài sản, Danh bạ, Quản trị, Cài đặt and Đăng xuất (through the one logout path). No avatar floats over the screens. The sheet traps focus, takes focus on open, closes on Esc, the scrim, its Đóng button or a downward drag, and returns focus to Thêm.

The workspace switcher in the sidebar and in the sheet SHALL list only the workspaces from `GET /api/me/workspaces`. Choosing one SHALL re-scope the session with `POST /api/auth/switch-tenant`, replace the access token, drop the whole query cache and the Tài liệu client state, and open the module the person was in under `?ws=` of the new workspace with no folder or view carried over. A refusal leaves the token, the data and the screen as they were and says so in a toast.

#### Scenario: Switch from the sheet
- **WHEN** the person opens Thêm, picks another workspace in the list and the server accepts
- **THEN** the token is the new workspace's, the old workspace's cached data is gone, the sheet closes and the same module opens on the new workspace

#### Scenario: Switch refused
- **WHEN** switch-tenant answers 403
- **THEN** the token and cached data are unchanged, the sheet stays open and a toast says the switch failed
