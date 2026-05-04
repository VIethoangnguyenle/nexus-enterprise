# WS Drive Object Cache Injection

## Proto Mapping
- WS Event: `DriveObjectEvent` — exists in ws.proto (field 13)
- Fields: `event_type` ("created"|"updated"|"deleted"|"moved"), `item_id`, `parent_id`, `workspace_id`
- Limitation: Only has item_id, NOT the full object. Cannot inject new items (no name/size/type).

## User Stories

### Story 1: Real-time drive folder updates
As a workspace member, I want the file list to update when others create, delete, or move files.

**Acceptance Criteria:**
- [ ] "deleted" event → remove item from `['drive', wsId, 'folder', parentId]` cache
- [ ] "moved" event → remove from old parent cache
- [ ] "created"/"updated" → invalidate the specific folder (NOT broad `['drive']`)
- [ ] Only the affected folder query is invalidated, not ALL drive queries

### Story 2: Optimistic drive mutations
As a user, I want folder creation and file deletion to reflect instantly.

**Acceptance Criteria:**
- [ ] Deleting a file removes it from the list immediately
- [ ] If delete API fails, file reappears
- [ ] Creating a folder shows it immediately (then confirmed by WS event)

## Flows

### Flow: WS driveObject "deleted"
1. Receive `driveObjectEvent` with `event_type: "deleted"`
2. Remove item from `['drive', wsId, 'folder', parentId]` cache
3. No HTTP call

### Flow: WS driveObject "created"
1. Receive `driveObjectEvent` with `event_type: "created"`
2. Cannot inject (no full object data) → invalidate `['drive', wsId, 'folder', parentId]`
3. Only invalidate the specific folder, NOT all drive queries
