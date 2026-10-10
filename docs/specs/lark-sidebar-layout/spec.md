# lark-sidebar-layout

## Purpose
Give the workspace a single persistent navigation surface that can yield screen space on demand.

## Status

Verified against code 2026-07-31. **Diverges from the implementation.**

`frontend/src/components/patterns/AppSidebar.tsx` hardcodes `w-[280px]`, not the 240px this
spec requires, and implements no collapse toggle — so the 64px icon-only mode does not exist.
Either the spec predates a deliberate redesign or the collapse feature was dropped; this has
not been decided, so the requirements are recorded here as written rather than edited to match
the code.

**Bổ sung 2026-08-01 — di cư design system.** `AppSidebar.tsx:105` giờ viết `w-70` thay cho
`w-[280px]`. Đây chỉ là đổi cách viết sang thang spacing, **không phải sửa độ lệch**: `w-70`
bằng đúng 280px, nên khoảng cách với 240px spec đòi vẫn còn nguyên và quyết định trên vẫn chưa
được đưa ra. Nút điều hướng trong sidebar đã chuyển sang primitive `NavRow`; chế độ thu gọn 64px
vẫn không tồn tại.

## Requirements

### Requirement: Hierarchical Sidebar Navigation
The layout SHALL feature a left sidebar (default width 240px) replacing the legacy icon rail. It SHALL include the workspace switcher, search input, and hierarchical navigation links (Messaging, Drive, Assets).

#### Scenario: Sidebar renders in workspace
- **WHEN** user is in the workspace view
- **THEN** they SHALL see the expanded sidebar with text labels and icons

### Requirement: Sidebar Collapse
The sidebar SHALL be collapsible to an icon-only mode (width 64px) via a toggle button, maximizing the main content area.

#### Scenario: User collapses sidebar
- **WHEN** the user clicks the collapse toggle
- **THEN** the sidebar width shrinks to 64px and text labels are hidden

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
