# unified-search

## Purpose
Lets a person find anything they may read — people, conversations, messages, documents, requests, events, assets — and
start common actions from one search box in every module.

## Status
Specified ahead of code (2026-10-19, plan `plans/261018-1742-mobile-ux-overhaul`, phase 08; design DESIGN.md §6 "Tìm cho
mọi tác vụ"). Messages and events have server search today; documents, requests and assets search follow. The approval
service's access control (`canSeeRequest` in `backend/services/approval/internal/domain/queries.go`) is implemented and
defines who may see a request; the search endpoint itself is not yet built.

## Requirements

### Requirement: Scope defaults to the open module and widens on request
The search SHALL start scoped to the module it was opened from, offer **Tất cả** and the other modules as scopes, and keep
the typed words when the scope changes. A personal workspace offers no Phê duyệt or Tài sản scope. In Tất cả, results come
grouped by kind, each group showing at most three with "Xem tất cả · N".

#### Scenario: Scope change keeps the words
- **WHEN** a person types "đối soát" in Tài liệu and then switches the scope to Tất cả
- **THEN** the typed words stay and results are grouped by kind, at most three per group

### Requirement: Each module searches on its own service and only returns what the caller may read
Each kind SHALL be searched by the service that owns it, on that service's existing route prefix: approval
`GET /api/approval/search` (inside the tenant and membership checks every approval route has), drive
`GET /api/drive/search` (membership of the named workspace, then a read check per result), assets through the existing
asset list with its search and filters, people through the existing contacts search. A result the caller may not read
SHALL never be returned, nor one from another workspace.

#### Scenario: Another workspace named
- **WHEN** a member of workspace A searches approvals or drive naming workspace B
- **THEN** the answer is 403 and nothing of B is returned

#### Scenario: File the caller cannot read
- **WHEN** a document matches the words but the caller has no read on it
- **THEN** it is not in the results and not counted in "Xem tất cả · N"

### Requirement: Approval search contract
`GET /api/approval/search` SHALL accept query parameters: `q` (text query, required), `limit` (page size), `status`
(filter by request status), `template_id` (filter by template), `created_by` (filter by requester: the requester's
**user ID**, the ID the screen's people picker holds; the server resolves it to the NGAC node a request records, and an
unknown ID, or a node ID, matches nothing), `from` and `to` (date range on `created_at`). Text matching SHALL search on
`template_name` and requester `display_name`, case-insensitively; `%` and `_` in `q` are the characters themselves, never
LIKE wildcards, so `q=%%` matches only a name containing `%%`. Access control
SHALL use `canSeeRequest`: a request is visible to its creator (`created_by`), anyone who has an assignment on it
(directly or through a role/department ancestor), or anyone whose department-request scope (`ResolveAccessibleScopes`
with `OpRead`) includes its `scope_oa_id`. The response SHALL be `{items: ApprovalRequest[], total: number}`.

#### Scenario: Requester searches their own requests
- **WHEN** a person searches with `q=nghỉ phép` and has submitted a request whose template name matches
- **THEN** that request appears in the results because `canSeeRequest` allows the creator

#### Scenario: Assignee searches
- **WHEN** an approver searches with `q=mua sắm` and has an assignment on a matching request
- **THEN** that request appears; a request they have no relationship to does not

#### Scenario: Filter by requester
- **WHEN** a person searches with `created_by` set to a requester's user ID
- **THEN** only that requester's matching requests the caller may see are returned; another requester's user ID returns
  none of them

#### Scenario: Wildcards in the words
- **WHEN** a person searches with `q=%%` or `q=__`
- **THEN** only requests whose template or requester name contains those characters are returned, not every visible request

#### Scenario: Department scope
- **WHEN** a department head whose readable scopes include an OA searches
- **THEN** requests scoped to that OA appear even if the head is not the creator or an assignee

### Requirement: Actions match the words
Actions (Tạo đề nghị, Sự kiện mới, Nhắn tin cho <người>, Cài đặt <module>, Đổi workspace) SHALL be offered only when the
workspace kind and the caller's rights allow them.

#### Scenario: Actions in a personal workspace
- **WHEN** a person in a personal workspace types words that match "Tạo đề nghị"
- **THEN** no such action is offered
