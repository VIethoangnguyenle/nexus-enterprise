# unified-search

## Purpose
Lets a person find anything they may read — people, conversations, messages, documents, requests, events, assets — and
start common actions from one search box in every module.

## Status
Specified ahead of code (2026-10-19, plan `plans/261018-1742-mobile-ux-overhaul`, phase 08; design DESIGN.md §6 "Tìm cho
mọi tác vụ"). Messages and events have server search today; documents, requests and assets search follow.

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

### Requirement: Actions match the words
Actions (Tạo đề nghị, Sự kiện mới, Nhắn tin cho <người>, Cài đặt <module>, Đổi workspace) SHALL be offered only when the
workspace kind and the caller's rights allow them.

#### Scenario: Actions in a personal workspace
- **WHEN** a person in a personal workspace types words that match "Tạo đề nghị"
- **THEN** no such action is offered
