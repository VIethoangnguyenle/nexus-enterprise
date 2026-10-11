# Smoke defects D1, S2, D4

## Root causes
- D1: `SendMessage` (messages.go) rejected any user message with empty text unless it had an attachment; only `drive_file` links were exempt. Document/approval card messages carry no text -> 400.
- S2 (D3): server answers a conversation with no task as `{"tasks": null}` (nil slice in `domain.ListTasks`). `useCreateTask.onMutate` did `[tempTask, ...old.tasks]` on the cached null -> TypeError before the request; `explain()` maps non-HTTP errors to "Kiểm tra kết nối mạng". No request was sent. Reproduced in vitest first.
- D4: `useQuoteWatch` (composer) mounted a `useQuery` with `queryFn: skipToken` on the same key as the messages list. That observer fetched on invalidate/refetch (its options win), throwing "Attempted to invoke queryFn when set to skipToken" -> list error. Reproduced in vitest first. No other skipToken use in src.

## Fixes
- Backend: `validateLinkedEntity` (user messages only; system lines keep their own link types): known types document/approval/drive_file/poll/task, non-blank id, else 400. `isLinkedCard` (document/approval) exempt from the needs-text rule. Write on channel still checked first. `ListTasks` returns non-nil empty slice.
- Frontend: `useQuoteWatch` now reads the cache via `useSyncExternalStore` on the query cache (no observer). `messagingApi.listTasks` normalizes null to `[]`; `useCreateTask`/`useUpdateTask` tolerate a null list.
- Spec: `docs/specs/chat-cards/spec.md` (no-text card rule, 3 scenarios).

## Files
- backend/services/messaging/internal/domain/messages.go (edit)
- backend/services/messaging/internal/domain/polls_tasks.go (edit)
- backend/services/messaging/internal/domain/link_cards_test.go (new: card no text, unknown type, empty/blank id, no-write deny, unknown type with text, empty task list non-nil)
- docs/specs/chat-cards/spec.md (edit)
- frontend/src/components/chat/use-quote-watch.ts (edit)
- frontend/src/components/chat/use-quote-watch.test.tsx (edit: refresh regression test)
- frontend/src/api/messaging.ts (edit)
- frontend/src/hooks/useChannelTasks.ts (edit)
- frontend/src/components/chat/TaskDialog.test.tsx (edit: null-list regression test)

## Gates
- messaging `go test ./...` pass, `go vet ./...` clean, `make check-layering` ok.
- vitest: 2242 pass, 1 fail = AssetsScreen (known). lint clean. typecheck:diff clean. Build skipped.
- Not re-run in the browser (dev services untouched; backend fix needs a messaging restart to take effect live; the frontend fix works without it).

## Notes
- Explicitly unknown link types with text are now also 400 (previously stored). Frontend sends only known types.
- Serena tools unavailable in this session; used Read/Edit/Bash.

Status: DONE
Summary: all three defects reproduced by a failing test, then fixed with deny tests and spec updated. Gates pass apart from the known AssetsScreen failure.
