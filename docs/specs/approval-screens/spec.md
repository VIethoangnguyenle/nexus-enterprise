# approval-screens

## Purpose
Let people send, decide and follow approval requests from one screen, and let administrators define the templates those requests follow. Everything is shown by name, amount and plain-language status: no request, user, role or department identifier ever reaches the screen, and approvers are chosen from lists, never typed.

## Requirements

### Requirement: Tabs by role, in the URL
The Phê duyệt screen SHALL offer the tabs "Chờ tôi duyệt", "Tôi đã gửi", "Đã xử lý", "Phòng ban" and "Mẫu". The open tab, the open request or template and the template builder SHALL be part of the URL (`?tab=`, `?request=`, `?template=`, `?edit=`), so a reload, Back/Forward or a pasted link lands in the same place. The first tab carries the count of requests waiting on the user.

#### Scenario: Reload keeps the place
- **WHEN** the user opens a request on the "Đã xử lý" tab and reloads
- **THEN** the same tab is shown with the same request open

#### Scenario: Paged lists grow
- **WHEN** the user chooses "Xem thêm" on a paged tab
- **THEN** the next page is added under the rows already shown; none is replaced

#### Scenario: Every row is reached
- **WHEN** a tab with five rows is read two at a time by following `next_cursor` until it is absent
- **THEN** each of the five rows is seen exactly once: the cursor names the last row of the page, so the next page starts right after it and skips none, even where rows share a timestamp: the cursor is the opaque pair (time, id) the lists are ordered by, and a cursor that is not one the service issued is 400 (this holds for "Đã xử lý", "Tôi đã gửi" and "Phòng ban")

### Requirement: Request table
Each request row SHALL show its title, the sender (avatar and display name), the amount when the template has a currency field, and a status as a word ("Chờ bạn", "Chờ <tên bước>", "Đã duyệt", "Trả lại", "Đã huỷ"). The row's title SHALL be a real button over the whole row, reachable and activatable from the keyboard, with ↑/↓ moving between rows.

#### Scenario: Waiting on me
- **WHEN** the user holds the pending assignment of a request's current step
- **THEN** its status reads "Chờ bạn"

#### Scenario: Narrow table
- **WHEN** the table is too narrow for four columns
- **THEN** the sender and the amount sit under the title

### Requirement: Loading, empty and error states
Every list SHALL show a skeleton while loading, an empty state that says what belongs there (with the way to create one where there is one), and an error state with a retry. An approval service that is not set up for the workspace SHALL say so instead of showing an error.

### Requirement: Request detail
Opening a request SHALL show, in a right panel: the amount, the sender with role, the department, when it was sent, the submitted answers, the approval chain and the trail. The approval chain SHALL list every approver (not only the user) with avatar, name, a status word, the step and the time; the user's own place is marked "(bạn)". The trail SHALL name the actor of each entry with an avatar, say what they did in a sentence, and give the time; entries made by the system carry no actor.

#### Scenario: Chain of the whole request
- **WHEN** a request has three steps and the user holds step two
- **THEN** all three approvers are listed, the first "Đã duyệt", the user's "Đang chờ", the third "Chưa tới"

#### Scenario: Trail not readable
- **WHEN** the server answers 403 for the trail of a request
- **THEN** the trail section says the user may not read it; the rest of the panel is unchanged

#### Scenario: Request not openable
- **WHEN** the server refuses a request named in the URL
- **THEN** the panel says it cannot be opened and shows no data

#### Scenario: No identifier
- **WHEN** any request, template, step or trail entry is shown
- **THEN** no UUID, node id, role code, entity code or other internal identifier appears in text, names, tooltips or form values

### Requirement: Decide
A user whose turn it is SHALL see "Duyệt" and "Trả lại" in the panel. "Duyệt" approves at once and tells the user. "Trả lại" SHALL open a dialog that requires a reason; an empty or blank reason is refused in the dialog and nothing is sent. The dialog starts empty for each request, traps focus, closes on Esc without closing the panel behind it, and returns focus to "Trả lại".

#### Scenario: Return with a reason
- **WHEN** the user types a reason and confirms
- **THEN** the request is rejected with that reason as its comment, the dialog closes and the user is told "Đã trả lại đề nghị"

#### Scenario: Return without a reason
- **WHEN** the user confirms with an empty reason
- **THEN** the dialog shows what is missing, focuses the field and sends nothing

#### Scenario: Server refuses
- **WHEN** the server refuses an approval
- **THEN** the request returns to the list and the user is told what failed

### Requirement: Batch approval
On "Chờ tôi duyệt" each row SHALL have a tick box and the header a select-all box. With at least one ticked, a bar SHALL show the count and offer "Duyệt N đề nghị". If the server approves fewer than were ticked, the user is told how many.

### Requirement: Create a request
"Tạo đề nghị" SHALL ask the user to choose an active template by its name, show that template's form and the names of its approval steps, and send the template and the answers. The client SHALL NOT make up any identifier: the server creates the request from the chosen template. Required fields that are empty are named and nothing is sent.

#### Scenario: Sent from the chosen template
- **WHEN** the user picks "Tạm ứng", fills the form and sends
- **THEN** the request body names the template and carries the answers; the new request follows exactly that template's steps

### Requirement: Templates are an administrator's
Creating or editing a template SHALL require `manage` on the tenant's management attribute (the workspace's Mgmt OA, `ngac.MgmtOAName`), checked by the server on `POST /api/approval/templates` and `PUT /api/approval/templates/{id}` against the caller's node; a caller without it, an anonymous caller, and a tenant with no management attribute get 403 and nothing is stored. `GET /api/approval/permissions` SHALL answer `can_manage_templates` so the screen can leave out what the server would refuse. The "Mẫu" tab, the template panel and the builder SHALL NOT be offered to a caller without it, and a link to them falls back to the first tab. Reading templates (to choose one when creating a request) stays open to every member.

#### Scenario: A member rewrites a chain
- **WHEN** a member sends `PUT` with a step naming themself as approver
- **THEN** the server answers 403 and the template is unchanged

#### Scenario: Administrator of another tenant
- **WHEN** an administrator signed in to tenant B edits a template of tenant A
- **THEN** the answer is 403: tenant A's management attribute is not theirs

### Requirement: Templates and the builder
The "Mẫu" tab SHALL list templates by name, kind in words, number of steps and state ("Đang dùng" / "Đã tắt"). A template panel SHALL show its form and its chain by approver name. The builder SHALL be a screen of its own addressed by the URL (name, kind, whether it is in use, the form fields, the steps). Each step SHALL name its approver by choosing a person, a role or a department from a list; there is no input that accepts an identifier. The kind of an existing template is shown but not editable. Steps can be reordered and removed (never the last one). A step without a name or without an approver, a field without a name, and a template without a name are refused with a message beside the control.

#### Scenario: Editing keeps what is not edited
- **WHEN** an existing template is saved from the builder
- **THEN** its active flag and priority are sent as they were, so renaming a switched-off template does not switch it on and an active one is not switched off

#### Scenario: Only resolvable approvers are offered
- **WHEN** the user adds a step
- **THEN** only person, role and department are offered; kinds the server cannot resolve are not

### Requirement: Realtime attribution
When another person decides a request while a list is open, its row SHALL be washed in that person's colour for 2.4 s with a label such as "Đức vừa duyệt". A change the user made, or one whose author is unknown, SHALL NOT be washed. Three or more changes within two seconds SHALL show one summary line. Under reduced motion the wash remains and nothing else moves.

### Requirement: Approval service names (REST)
The approval REST SHALL return display names beside the ids it holds, additively: `created_by_name` and `department_name` on requests, `user_name` on assignments, `actor_name` on audit entries, `approver_name` on steps, `created_by_name` on templates. A name that cannot be resolved is omitted, never replaced by an id. `GET /api/approval/requests/{id}` SHALL return the request, its steps and form fields as frozen when it was made, and every assignment, to a caller who can see the request by the same three paths as the audit trail (requester, assignee of any step, department scope); anyone else, and a request that does not exist, get 403.

### Requirement: Template editing is saved whole
Updating a template SHALL replace its steps and conditions, and its form fields when the body carries the list (an empty list removes them all; an absent one keeps them), in one transaction. `is_active` and `priority` are written as sent.

### Requirement: Create from a chosen template
`POST /api/approval/requests` SHALL accept `template_id`; `entity_type` and `entity_id` are optional (an absent `entity_id` is generated by the server). Routing SHALL be decided on values the server derives from the submitted form: every answer by its label, and the first currency answer also as `amount`. Figures the client states separately (`entity_fields`) are ignored by REST. A chosen template SHALL be accepted only if it is active, its conditions hold on the form, and no higher-priority active template of the same entity type also matches; otherwise 400. Without `template_id` the highest-priority matching template is used.

#### Scenario: Large amount through the small template
- **WHEN** a form with 500 000 000 is sent with the template whose condition is `amount < 10 000 000`
- **THEN** the request is refused (400) and none is created

#### Scenario: Stricter template applies
- **WHEN** the chosen template has lower priority than another active template of the same type whose conditions also hold
- **THEN** the request is refused (400)

### Requirement: Role and department approvers
A step whose approver is a role or a department SHALL be stored as ONE group row on the step: its `user_node_id` is the role's or department's UA and its grant source is `role:<ua>` / `department:<ua>`. A template save SHALL reject (400) an approver that does not exist in the tenant: a person must be a member of the tenant, a role a UA under the tenant's PC, a department one of the tenant's departments (a department id is stored as its `ngac_ua_id`). Step orders SHALL be exactly 1..n, approver kinds one of person / role / department.

When someone acts and has no row of their own on the current step, the server SHALL read the caller's ancestors from the policy service at that moment and accept the action only if a pending group row of that step names one of them; it then inserts a row of the person (same grant source, approved or rejected). A unique (request, step, person) index makes a second action by the same person impossible. The quorum counts people. A step with no resolvable approver SHALL be refused before the request is stored: a request never exists without someone to decide it.

The pending list, the right to read a request (detail, trail) and the event audience SHALL include requests assigned to the roles and departments the caller belongs to; a member who has acted no longer sees the step as waiting on them, and event audiences name the members who have not yet acted.

#### Scenario: A member acts for the role
- **WHEN** a member of the role approves a step given to that role
- **THEN** the member has their own approved row, the group row stays pending until the quorum is met, and the request advances once enough different people have approved

#### Scenario: Not a member
- **WHEN** someone outside the role, or in a different department, approves
- **THEN** the answer is 403 and nothing is recorded

#### Scenario: Removed member
- **WHEN** a member removed from the role after the request was made approves
- **THEN** the answer is 403

#### Scenario: Twice
- **WHEN** the same member approves (or then rejects) the same step again
- **THEN** the answer is 409 and the count does not change

#### Scenario: Policy unreadable
- **WHEN** the caller's memberships cannot be read
- **THEN** acting and reading the trail fail; the pending list falls back to the caller's own rows (shorter, never wider)

### Requirement: Names stay inside the tenant
Display names SHALL be looked up within the caller's tenant only: people through the tenant's members (by `tenant_users.ngac_node_id` or `users.ngac_node`), departments of the tenant's workspace, and roles or departments' UAs only if they sit under the tenant's PC. A role is named by its `display_name` property, falling back to its node name. A key from another tenant is omitted, never named.

### Requirement: Template edits do not overwrite each other
Updating a template SHALL carry the `updated_at` it was read at (`expected_updated_at`, required); if the template has changed since, the answer is 409 and nothing is written. Steps, form fields and conditions are each kept when the body leaves them out and cleared when it sends an empty list (steps cannot be cleared). Duplicate or non-contiguous step orders, unsupported approver kinds and malformed conditions are 400. A template is read as one consistent snapshot. The entity type of a template does not change.

### Requirement: Detail panel as a phone sheet
Below 768 px the detail panel SHALL be a modal layer rendered above the phone tab bar: focus moves into it, Tab stays inside, Esc closes it, and focus returns to what opened it. With a dialog open over it, only the dialog answers the keyboard. Wider than that it is a plain panel beside the content.

## Status
Known gaps, recorded rather than resolved:
- A rejected request is final: there is no undo and no resubmit, so "Trả lại" has no "Hoàn tác" and the dialog says the sender can send a new request.
- Requests created through the UI carry the default scope, so they appear to the requester, the assigned approvers (directly or through their role or department) and whoever reads the default scope.
- The sidebar count of waiting requests (mockup §3) is not shown; the tab carries it.
- The reconciliation consumer no longer copies a pending step to a newly added member (the group row covers them); it still revokes a person's own pending rows when they leave a UA.

### Requirement: A decision and its audit entry stand or fall together
Approving, rejecting and creating a request SHALL each be one change that includes its audit entries: the decision, the step it completes or advances (including the next step's assignments), and every audit entry written for them take effect together or not at all. An audit entry that cannot be written fails the operation and leaves the request exactly as it was. Decisions on one request SHALL be serialised by a lock on the request, taken first, so two approvals of a step that needs two both count each other and an approval racing a rejection ends in one terminal state with nothing left pending.

#### Scenario: Audit cannot be written
- **WHEN** the audit write fails while an approval is being recorded
- **THEN** the approval is not recorded, the request is unchanged, and the caller is told it failed

#### Scenario: Two approvals at once
- **WHEN** two people approve a step that needs two approvals at the same moment
- **THEN** the step completes; it is not left waiting for a third
