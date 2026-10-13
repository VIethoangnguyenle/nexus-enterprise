# lark-sidebar-layout

## Purpose
Give the workspace a single persistent navigation surface that can yield screen space on demand.

## Status

Updated 2026-10-13: phone navigation is the search bar, the ☰ drawer and the Chat bar (`design/mockups/chat-mobile-home.html`). Updated 2026-10-11. The main menu (`frontend/src/components/patterns/AppSidebar.tsx`) has the two states
below, designed in `design/mockups/app-rail.html`. The global search input is still not in the
sidebar; that requirement stays as written.

## Requirements

### Requirement: Main menu in two states
The workspace SHALL show one persistent main menu on the left at 1024px and wider, a single `<nav aria-label="Điều hướng chính">`, in two states: the **rail** (72px wide: the workspace letter tile, then each destination as an icon with its full label under it, 11px) and the **expanded sidebar** (232px: workspace switcher, then each destination as an icon and label in a row). Both SHALL hold the same destinations in the same order: Tin nhắn, Tài liệu, Phê duyệt, Tài sản, Danh bạ, then at the foot Thông báo, Quản trị, Cài đặt and the signed-in person. A personal workspace SHALL NOT offer Phê duyệt, Tài sản or Quản trị. A destination's count SHALL show as a pill (`99+` above 99, hidden at 0) with the number also in text for screen readers, and the open destination SHALL carry `aria-current="page"`. The **rail is the default** at every desktop width, until the person expands it.

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
Below 1024px, where the sidebar is not shown, every list screen of every module SHALL start with a **search bar**: a pill holding a ☰ button (`aria-label` "Mở menu chính", `aria-haspopup="dialog"`, with a dot when a request waits on the person in an organization workspace or a notification is unread), a hint that names the module's search ("Tìm trong chat", "Tìm tài liệu", "Tìm đề nghị", "Tìm tài sản", "Tìm người", "Tìm cài đặt") and the person's avatar. The hint SHALL go to the search of the screen below it (`[data-module-search]`: a box takes focus, a button opens its panel); a screen with none SHALL show the module's name there instead, never a button that does nothing. The bar SHALL NOT be drawn inside a conversation or a document, which carry their own header.

☰ SHALL open the **main menu**, a modal drawer from the left: the workspace block (name and kind; pressing it unfolds the workspaces the person can enter, the open one ticked, "Lời mời" when offers wait, "Tạo workspace"), then Chat, Tài liệu, Phê duyệt, Tài sản, Danh bạ, then Thông báo, Quản trị, Cài đặt, and the person's name and role last. The destinations and what hides them SHALL be the sidebar's (`frontend/src/lib/nav-destinations.ts`): a personal workspace SHALL NOT offer Phê duyệt, Tài sản or Quản trị. Counts SHALL show as in the sidebar (Chat unread by notification level, Phê duyệt "N chờ bạn", Thông báo unread, `99+` above 99) and be part of each link's accessible name; the open module SHALL carry `aria-current="page"`. Thông báo SHALL open the notification list as a full screen with Quay lại. The drawer traps focus, takes it on the workspace block, closes on Esc, the scrim or a drag to the left, returns focus to ☰, and under reduced motion fades instead of sliding. The avatar SHALL open a sheet with the person's name, role and workspace, a link to the notification settings, Cài đặt tài khoản, Cài đặt and Đăng xuất (through the one logout path).

On the list of conversations (`/channels`) only, a floating **Chat bar** SHALL offer four icon-only links to `/channels?tab=home|direct|sections|more` (Trang chủ, Trực tiếp, Mục, Thêm; Trang chủ is the address without a tab, and an unknown tab opens Trang chủ). Each link's `aria-label` carries what it reports: Trang chủ a dot when anything is unread, Trực tiếp the unread direct messages, Mục the groups with unread messages, Thêm a dot when someone has named the person (`99+` above 99, nothing at 0, counts by notification level). Beside it a 64px **+** button SHALL open the sheet "Trò chuyện mới" (Nhắn tin trực tiếp, Tạo nhóm, Tạo mục). The bar is not drawn inside a conversation or in any other module, nor at 1024px and wider; while it is on screen the list keeps 108px plus the safe-area inset under its last row and toasts float 16px above it.

The workspace switcher in the sidebar and in the drawer SHALL list only the workspaces from `GET /api/me/workspaces`. Choosing one SHALL re-scope the session with `POST /api/auth/switch-tenant`, replace the access token, drop the whole query cache and the Tài liệu client state, and open under `?ws=` of the new workspace with no folder or view carried over: the module the person was in from the sidebar, Trang chủ of Chat from the drawer. A refusal leaves the token, the data and the screen as they were and says so in a toast.

The workspace on screen and the session token SHALL always be the same workspace. With no `?ws=`, or one the person does not belong to, the workspace is the one the token speaks for, then the first listed. When the URL names another workspace (deep link, reload, shared link), the workspace shell SHALL re-scope the session with `POST /api/auth/switch-tenant` once for that workspace, without navigating, and render nothing tenant-scoped (so no tenant-scoped request) until the token matches. If the switch is refused, the address SHALL fall back to the token's workspace (`ws` replaced, no history entry) with the same toast.

#### Scenario: A link to another workspace
- **WHEN** a person whose token speaks for workspace A opens a link with `?ws=B`, a workspace they belong to
- **THEN** the screen shows the loading state, the session is re-scoped to B once, and only then do the lists and previews load, all for B

#### Scenario: A list screen on a phone
- **WHEN** a person opens Phê duyệt at 390px
- **THEN** the screen starts with the search bar saying "Tìm đề nghị" when the screen has a search of its own, ☰ and the avatar, and no Chat bar

#### Scenario: Reaching another module
- **WHEN** the person opens ☰ and picks Tài sản
- **THEN** the drawer closes, Tài sản opens and focus is back on ☰

#### Scenario: Switch from the drawer
- **WHEN** the person opens ☰, unfolds the workspaces, picks another one and the server accepts
- **THEN** the token is the new workspace's, the old workspace's cached data is gone, the drawer closes and Trang chủ of Chat opens on the new workspace

#### Scenario: Switch refused
- **WHEN** switch-tenant answers 403
- **THEN** the token and cached data are unchanged, the drawer stays open and a toast says the switch failed

#### Scenario: Inside a conversation
- **WHEN** the person opens one conversation
- **THEN** neither the search bar nor the Chat bar is drawn, and the list does not keep room under it
