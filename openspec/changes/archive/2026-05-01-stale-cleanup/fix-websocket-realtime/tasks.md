## Tasks

### Phase 1: Backend fixes (DONE)
- [x] Update `cmd/main.go` — WS path `/ws` → `/api/ws`
- [x] Add `Broadcaster` interface to `rest/handler.go`
- [x] Wire Hub into REST `Handler` struct
- [x] Add `BroadcastToChannel()` in REST `SendMessage`
- [x] Pass `hub` to `rest.NewHandler()` in `cmd/main.go`
- [x] `go build ./cmd/` passes

### Phase 2: Frontend subscribe fix
- [x] Add `sendSubscribe` / `sendUnsubscribe` imports from WS store in `channels.$channelId.tsx`
- [x] Add `useEffect` that subscribes on mount and unsubscribes on cleanup

### Verification
- [x] Two-tab test: send message in Tab A → appears in Tab B without reload
- [x] Channel navigation: switching channels unsubscribes old, subscribes new
