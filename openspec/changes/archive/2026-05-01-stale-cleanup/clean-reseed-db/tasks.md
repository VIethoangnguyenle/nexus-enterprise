## Tasks

### Phase 1: Stop services & TRUNCATE
- [x] Stop all services (make stop)
- [x] TRUNCATE all tables with CASCADE
- [x] Clean MinIO buckets (15 buckets removed)

### Phase 2: Restart & Re-seed via API
- [x] Restart services (make run)
- [x] Register user `hoangnlv` via POST /api/auth/register
- [x] Create workspace `NGAC Platform` via POST /api/workspaces
- [x] Create channel `general` via POST /api/workspaces/:id/channels
- [x] Seed 3 welcome messages via POST /api/channels/:id/messages

### Phase 3: Verify
- [x] Verify NGAC graph integrity (0 orphan nodes, 28 nodes, 31 assignments, 17 associations)
- [x] Verify channel access works (GET messages → 200)
- [x] Verify drive works (GET drive → 200)
- [x] UI smoke test via browser (login, view messages, create new channel — all working)
