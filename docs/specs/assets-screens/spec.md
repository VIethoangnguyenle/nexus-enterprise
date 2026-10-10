# assets-screens

## Purpose
Let people see, request, hand over and administer assets from one screen inside the shared workspace shell. Everything is shown by name and plain-language status; no asset, request, type or user identifier ever reaches the screen, and people and assets are chosen from lists, never typed.

## Requirements

### Requirement: One screen, four tabs, state in the URL
Tài sản SHALL be a route of the workspace shell (`/assets`) with the tabs Tổng quan, Danh sách, Yêu cầu and Loại tài sản. The tab (`?section=`), list filters and page, the open asset, request or type, the request filter and the new-request form SHALL be in the URL. The old `/assets/dashboard`, `/list`, `/requests`, `/types`, `/request/new` and `/assets/<id>` URLs SHALL redirect to the matching tab, keeping `?ws=`.

#### Scenario: Reload keeps the place
- **WHEN** the user opens an asset on page 2 of a filtered list and reloads
- **THEN** the same filter, page and open asset are shown

### Requirement: States follow the backend lifecycle
The screens SHALL use exactly the states Chờ duyệt, Sẵn sàng, Đang giao, Bảo trì, Ngừng dùng and Đã thanh lý, and offer only the lifecycle steps the server says the caller may take from the current state, as Vietnamese verbs.

### Requirement: Detail, hand-over and history
An asset's panel SHALL show its holder, its type's own fields, the steps available, and a history naming the actor of each step with avatar, sentence and time. Giving an asset away SHALL pick the person from a list. Retiring and disposing SHALL ask for confirmation first.

### Requirement: Requests, with the asset chosen in the approving dialog
Rejecting SHALL require a reason. Where the server says the caller may both decide and hand over, "Duyệt và giao" SHALL open one dialog in which the approver picks an available asset of the requested type, and approving SHALL send that asset in the same request. When nothing is available the dialog offers "Chỉ duyệt". A request approved earlier is given an asset through the same picker. A caller who may not decide sees no decision.

### Requirement: Dashboard facts
The overview feed SHALL show request decisions (approved, rejected) beside lifecycle steps, each naming who decided and for whom. "Đang giao N, cho M người" SHALL count only assets in the Đang giao state. The maintenance card SHALL say how many have been in maintenance more than 14 days.

### Requirement: Failures say which refusal it was
A failed decision or step SHALL be explained by the server's reason (already decided, asset just given away, asset changed, person no longer a member, wrong type, same holder), not by status code alone.

### Requirement: Types and their own fields
Types SHALL list with category in words, asset and field counts. Whoever may manage types can add a type and edit its custom fields (text, number, date, person, choice); others see them read-only. Renaming, re-categorising and deleting a type are not offered.

### Requirement: Loading, empty and error states; no identifier
Every list SHALL show a skeleton while loading, an empty state saying what belongs there, and an error state with retry. No UUID or internal code SHALL appear in text, names, tooltips or inputs.

## Status
Known gaps: no cross-user live updates until the messaging service broadcasts asset events (the client refreshes on them); asset field values cannot be edited after creation; type rename, re-categorise and delete have no backend.
