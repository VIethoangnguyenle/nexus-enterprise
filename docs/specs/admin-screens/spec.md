# admin-screens

## Purpose
Let a workspace's administrators see and shape their organisation from three screens (Tổng quan, Người dùng, Vai trò): the departments, the people with their roles, and what each role permits. Everything is shown by name, in words: no user, role, department or database identifier reaches the screen, and nothing is chosen by typing one.

## Requirements

### Requirement: Three screens, one frame, state in the URL
Quản trị SHALL have the tabs Tổng quan, Người dùng and Vai trò under the workspace shell. The workspace (`?ws=`), the open department (`?dept=`), person (`?member=`), role (`?role=`) and the permission editor (`?edit=1`) SHALL be part of the URL, so a reload, Back/Forward or a pasted link lands in the same place. Moving to another tab keeps only the workspace. The key is `member`, not `user`: the auth store reads `?user=` as which account's session to use.

#### Scenario: Reload keeps the place
- **WHEN** an administrator opens a person on Người dùng and reloads
- **THEN** the same person is open in the panel and the session is intact

#### Scenario: Another tab
- **WHEN** a department is open and the administrator chooses Người dùng
- **THEN** the URL keeps the workspace and nothing else

### Requirement: The region is for people who may manage it
The frame SHALL ask the people list once. A 403 means the person may not manage the workspace, and every tab SHALL say so in words ("Bạn chưa có quyền quản trị …") with no table, tree or action. While the workspace is not yet resolved, or a list is loading, a skeleton is shown; an empty state never flashes before the answer.

#### Scenario: Not a manager
- **WHEN** the people list answers 403
- **THEN** the tab shows the no-permission message and offers neither "Mời thành viên" nor "Tạo vai trò"

### Requirement: Tổng quan shows the organisation as one tree
Tổng quan SHALL show the member, department and role counts and the departments as the shared tree, two levels open, with each department's head count at the right. Choosing a department opens its panel: where it sits (chosen from the tree by name, never as its own subtree), who is in it by name, adding a person by search, renaming and deleting (after a confirmation). The activity feed of the mockup is not drawn (see Status).

#### Scenario: Moving a department
- **WHEN** the administrator chooses another parent in the panel
- **THEN** the department is moved under it (or to the root) and a toast says where

#### Scenario: Deleting
- **WHEN** the administrator chooses "Xoá phòng"
- **THEN** a dialog asks first; Esc closes the dialog and leaves the panel open

### Requirement: Người dùng lists people with role and department pickers
Người dùng SHALL list the workspace's people as a hairline table (name with email, department, at most two role pills then "+N", standing in words), with search over name, email and title that ignores accents, and filters by department (including the departments beneath) and by role. The default role "Thành viên" appears only for a person who holds nothing else. A row's name is a real button over the whole row; ↑/↓ move between rows and Enter opens the person. The panel shows roles as pills that can be removed in place with a toast offering "Hoàn tác", a picker to add a role, the department chosen from the tree, and "Xoá khỏi workspace" after a confirmation; the Owner pill cannot be removed here and a person cannot remove themselves.

#### Scenario: Taking a role away
- **WHEN** the administrator removes a role pill
- **THEN** the role is removed at once and the toast "Hoàn tác" gives it back

#### Scenario: Filtering by department
- **WHEN** the administrator filters by a department that has sub-departments
- **THEN** people of the sub-departments are listed too

### Requirement: Mời thành viên sends invitations
The invite dialog SHALL take email addresses as chips (typed, pasted, separated by comma, semicolon or space), an optional role and an optional department. Each address becomes a pending invitation; nobody is added by it, and the dialog says the same for every address, so it can say nothing about whether an account exists. An address the server refuses (not an address, over the hourly limit, no right to invite) is named under its chip and kept for another try; a rate limit stops the rest. There is no way to type an identifier, and each opening starts empty.

#### Scenario: Three addresses
- **WHEN** three addresses are invited, one with an account, one without, one already a member
- **THEN** three invitations are sent, a toast says "Đã gửi lời mời cho 3 địa chỉ", and nothing on screen distinguishes them

### Requirement: Lời mời đang chờ can be seen and withdrawn
Người dùng SHALL list the open offers below the table (address, who invited, the role and department that ride along, when it was sent and when it lapses) with "Thu hồi" on each; the header says how many ("64 thành viên, 3 lời mời") and Tổng quan shows the count with how many lapse within two days. A person who may manage but not invite sees none of this.

#### Scenario: Withdrawing
- **WHEN** the administrator chooses "Thu hồi" on an offer
- **THEN** it is removed from the list and the invitee can no longer accept it

### Requirement: Vai trò are read by their names
Vai trò SHALL list the two built-in roles ("Chủ sở hữu", "Thành viên", with a lock and "Hệ thống") and the administrator's roles by the name they were given, with the number of people holding each. A role's panel shows who holds it and, per area, what it permits as words (Xem, Sửa, Tải lên, Duyệt, Chia sẻ, Quản lý, Mời, Tạo nhóm chat). It never shows a node name (`Role_…`, `…_Owners`). The built-in roles cannot be edited or deleted, and Members shows `share` on Documents. Creating a role asks for a name only; a name the server turns down is explained next to the field. Deleting asks first.

#### Scenario: Reserved name
- **WHEN** an administrator names a role "Dept_Sales"
- **THEN** the dialog stays open with an explanation under the field

### Requirement: The permission editor renders what the server offers
"Sửa quyền" SHALL draw one row per area the server lists and one column per operation any area offers, with a pressable cell only where the server says the operation applies to the area; elsewhere there is nothing to press. On a phone the same data is a list of switches grouped by area. Changes are drafted locally; a bar says how many are unsaved and who they affect ("14 người có vai trò Kế toán sẽ được Duyệt trên Tài sản."); saving writes one request per changed area with the whole set for that area, and a toast offers "Hoàn tác". Arrow keys move between cells, skipping those that do not apply. A built-in role opens a note instead.

#### Scenario: A table of the server's own
- **WHEN** the server lists documents with `read` and `share` and assets with `approve`
- **THEN** the columns are Xem, Duyệt and Chia sẻ, and a cell exists only for those three pairs

#### Scenario: Save fails
- **WHEN** saving answers 409
- **THEN** a toast explains that the role changed in the meantime and the editor shows what the server holds

### Requirement: Motion and layers follow the design system
Panels and dialogs enter and leave through the shared presets; Esc closes only the topmost layer (a picker, then a dialog, then the panel); a dialog's state is reset each time it opens and a panel's per item (mounted keyed by the item). Rows are keyboard-reachable; pickers open in place, so they work inside dialogs and panels.

## Status
Known gaps, recorded rather than resolved:
- No administrators' activity feed: nothing records who changed a role, a permission, a member or a department, so Tổng quan does not draw one.
- The "Đã mời" filter of the mockup is a list under the table instead; the invite dialog has no message field. The invited address is emailed (see `workspace-admin-authorization`, "Inviting records an offer"), but the dialog does not report whether the mail went out: the answer is the same 202 either way, and the invitee finds the offer on their next verified sign-in, on a screen that belongs to the sign-in group.
- No lock-account action: nothing in the backend can lock one. A person listed as disabled in `tenant_users` is shown as "Đã khoá".
- A role cannot be renamed, and "Trưởng phòng" of a department is not shown: neither exists in the backend.
- The permission editor drafts in memory; leaving it by another route discards the draft without asking.
