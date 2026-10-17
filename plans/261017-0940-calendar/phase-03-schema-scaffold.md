---
phase: 3
title: "Schema + khung service + đăng ký hạ tầng"
status: pending
priority: P1
effort: "1d"
dependencies: [2]
---

# Phase 03: Schema + khung service

## Overview
Bảng dữ liệu và service Go `calendar` chạy được (health, REST rỗng), đăng ký ở mọi nơi một service phải có.

## Requirements
- `data/migrations/057_calendar.sql` (schema public, mọi bảng có `workspace_id`):
  - `calendars(id uuid pk, workspace_id, kind check (personal|workspace|channel), owner_node_id null, channel_id null,
    name, color smallint, ngac_oa_id not null, created_at)` + unique partial: personal (workspace_id, owner_node_id),
    workspace (workspace_id), channel (channel_id).
  - `calendar_subscriptions(user_node_id, calendar_id, color null, visible default true, position, pk(user_node_id,
    calendar_id))`.
  - `calendar_shares(calendar_id, grantee_node_id, level check (view|edit|manage), granted_by, created_at, pk)` — cache
    hiển thị; quyền thật là NGAC.
  - `calendar_events(id uuid pk, workspace_id, calendar_id fk cascade, organizer_node_id, title, description, location,
    starts_at timestamptz, ends_at timestamptz, all_day, tzid, rrule null, recurrence_until null, recurring_event_id null
    fk self cascade, original_start null, cancelled default false, kind check (event|focus|ooo), transparency check
    (busy|free), visibility check (default|private), guests_can_invite, guests_can_see_list default true, has_call,
    ngac_oa_id null, guests_ua_id null, version int default 1, created_at, updated_at)`; index (calendar_id, starts_at),
    unique (recurring_event_id, original_start).
  - `calendar_attendees(event_id, user_node_id, response check (needs_action|accepted|declined|tentative), comment,
    is_organizer, responded_at, pk(event_id, user_node_id))`.
  - `calendar_reminders(event_id, user_node_id null, minutes_before, pk/unique)` (user null = mặc định sự kiện).
  - `calendar_reminder_deliveries(event_id, user_node_id, occurrence_start, minutes_before, delivered_at, pk(all 4))`.
  - `calendar_user_settings(user_node_id, workspace_id, tzid, work_start time, work_end time, default_reminder, pk)`.
  - `calls`: `channel_id` DROP NOT NULL, thêm `calendar_event_id uuid null`, CHECK đúng một trong hai, unique một cuộc gọi
    mở mỗi sự kiện.
- Service `backend/services/calendar` sao cấu trúc `asset`: `cmd/main.go` (bootstrap, ConnectDB, grpcauth.Dial policy
  read/write, gRPC health theo `grpcauth.ServerOptions`, Echo `httputil.NewEcho("calendar")`, Shutdown), `go.mod`
  (`replace ngac-platform => ../..`, `github.com/teambition/rrule-go`), `Dockerfile`, `internal/{rest,domain,store,events}`.
- Đăng ký: `Makefile` (SERVICES + khối env của target dev), `Procfile.dev`, `.env.example` (`CALENDAR_SERVICE_ADDR`,
  `CALENDAR_REST_PORT=8187`, gRPC 50059), `docker-compose.yml` (block + Traefik `PathPrefix(/api/calendar) ||
  PathRegexp(/api/workspaces/[^/]+/calendar)`), `deploy/docker-compose.prod.yml`, `deploy/nginx/default.conf` (regex +
  prefix), `deploy/check-compose-env.sh`, `.github/workflows/ci.yml` + `deploy.yml` (matrix), `frontend/vite.config.js`
  (prefix `/api/calendar` và nhánh regex `/api/workspaces/:id/calendar…` → 8187), `backend/pkg/realtime/event.go`
  `Topics()` thêm `calendar.events`.

## Related Code Files
- Create: `data/migrations/057_calendar.sql`, `backend/services/calendar/**`
- Modify: các file đăng ký ở trên

## Implementation Steps
1. Backup DB dev (`pg_dump` vào `.dev-backups/`), viết migration, `make db-migrate`, quan sát `\d calendar_events`, `\d calls`.
2. Scaffold service; `make build-check`; chạy service → health OK.
3. Đăng ký hạ tầng; `docker compose config -q`.

## Success Criteria
- [ ] Migration áp và quan sát trên DB; test messaging calls cũ vẫn xanh sau khi `channel_id` nullable.
- [ ] `make build-check` gồm calendar.
