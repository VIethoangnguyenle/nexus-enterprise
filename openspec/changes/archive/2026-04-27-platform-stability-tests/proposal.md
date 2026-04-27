# Platform Stability: Bug Fixes + Comprehensive Test Suite

## Vấn đề

App hiện tại đang lỗi nhiều khi thao tác thực tế trên UI. Root cause: **không có test nào** — zero Go unit tests, zero frontend component tests. Bugs chỉ được phát hiện khi user click vào app.

### Bugs đã phát hiện

| # | Bug | Root Cause | Service |
|---|-----|-----------|---------|
| 1 | `cannot scan NULL into *string` khi list channels | `ListChannels` và `GetChannel` thiếu `COALESCE(workspace_id,'')` | Messaging |
| 2 | `channels_channel_type_check` khi create channel | Frontend gửi `channel_type: 'channel'` thay vì `'workspace'` | Frontend (**đã fix**) |
| 3 | Channel tạo ra bị orphan (null workspace_id) | `CreateChannelModal` lấy `wsId` từ `useWorkspaces` có thể rỗng | Frontend |

### Vấn đề hệ thống

- **0 Go unit tests** — không service nào có `*_test.go`
- **0 frontend tests** — không có Vitest setup
- **Chỉ có `test_app.sh`** (59 curl tests) — chỉ cover happy path, không test edge cases

## Mục tiêu

1. **Fix tất cả bugs đã phát hiện** — không deploy code chưa pass test
2. **Go unit tests cho tất cả services** — focus vào store layer và gRPC handlers
3. **Frontend component tests** — Vitest + Testing Library cho critical flows
4. **Rule mới: TDD** — test case viết trước, test pass trước khi đánh done

## Scope

### In-scope
- Fix 3 bugs trên
- Go unit tests: messaging, asset, document, auth, workspace, policy, gateway
- Frontend tests: Vitest setup + tests cho CreateChannelModal, Sidebar, API layer
- CI rule: `go test ./...` + `npm test` phải pass

### Out-of-scope
- E2E browser tests (Playwright) — future change
- Performance/load testing
- Frontend integration tests (MSW mocking) — future change
