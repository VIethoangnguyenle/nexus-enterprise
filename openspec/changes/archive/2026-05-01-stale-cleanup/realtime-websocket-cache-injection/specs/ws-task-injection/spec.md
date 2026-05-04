# WS Task Update Cache Injection

## Proto Mapping
- WS Event: `TaskUpdateEvent` — exists in ws.proto (field 12)
- Fields: `task_id`, `channel_id`, `status`, `assignee_id`, `title`

## User Stories

### Story 1: Real-time task status updates
As a channel member, I want to see task status changes in real-time.

**Acceptance Criteria:**
- [ ] When another user changes task status, status badge updates without refresh
- [ ] Task list cache `['tasks', channelId]` updates directly
- [ ] No HTTP GET `/channels/{id}/tasks` triggered by WS event

### Story 2: Optimistic task update
As a user, I want task changes to reflect immediately.

**Acceptance Criteria:**
- [ ] Changing task status updates the badge instantly
- [ ] If API fails, status rolls back
- [ ] WS event deduplicates

## Flows

### Flow: WS task update event
1. Receive `taskUpdateEvent`
2. Update `['tasks', channelId]` cache → find task by id, update fields
3. No HTTP call
