# Chat shortcuts backend (Được nhắc tới / Đã gắn sao)

Status: DONE. Nothing committed, git index untouched.

## Contract as built
- Migration `data/migrations/048_chat_stars.sql` (`starred_conversations`, `starred_messages`, index `(user_id, created_at DESC)`). Dev DB backed up to `~/backups/nexus/ngac-before-048-20261011-104133.sql.gz`; applied to `ngac` (`make db-migrate`) and `ngac_chat_mine`; tables verified in both.
- `PUT|DELETE /api/channels/{id}/star`, `PUT|DELETE /api/messages/{id}/star` -> 204, read required on both verbs (403), idempotent; deleted message star -> 400 (unstar still allowed); unknown id -> 404.
- `GET /api/messages/starred?before&before_id&limit` -> `{messages, has_more}`; messages carry `starred:true`, `starred_at`. Newest star first, tie-break `message id DESC`; only readable channels (workspace in token + DMs), deleted excluded; enriched (reactions/pins/attachments/quotes/thread state).
- `GET /api/messages/mentions?...` -> `{messages, has_more}` + `unread`; others' non-deleted messages mentioning caller (roots + replies), muted included, newest first (`created_at DESC, id DESC`). unread: root = after channel `read_receipts.last_read_at`; reply = after `thread_follows.last_read_at`, else root `created_at`.
- `starred` added on each `GET /api/channels/unread` item and on `GET /api/channels/{id}`. Not on other message payloads.
- limit default and cap 50; bad `limit`/`before` -> 400.

## Frontend must know
- Proto changes (additive): `Message.starred=24, starred_at=25, unread=26`; `Channel.starred=12`; `ChannelUnread.starred=7`. `make proto` run; `npm run proto:gen` run, no diff (frontend only consumes ws.proto).
- JSON omits false/empty fields: `unread`, `starred`, `has_more` are absent when false.
- Timestamps serialize as `{seconds, nanos}` objects (not strings), incl. `starred_at`, `created_at`. The `before` cursor wants RFC3339 with full precision: build it from the last item as `new Date(seconds*1000).toISOString().replace('.000Z', '.' + String(nanos).padStart(9,'0') + 'Z')` (ms-only precision can skip rows). Cursor for starred = last item `starred_at` + its `id`; for mentions = last item `created_at` + its `id`.

## Files
- data/migrations/048_chat_stars.sql
- backend/proto/messaging/messaging.proto, messaging.pb.go (regenerated)
- backend/services/messaging/internal/store/stars.go (new), store/models.go (`StarredAt`, `Unread`)
- .../domain/stars.go (new), domain/messages.go (`messageToProto`), domain/reactions_pins_receipts.go (`GetUnreadCounts` starred)
- .../rest/stars.go (new), rest/handler.go (routes, `GetChannel` -> `GetChannelFor`)
- Tests: domain/stars_test.go (8), rest/stars_test.go (4), rest/engagement_test.go (harness: `BatchCheckAccess`, tenant, routes), rest/threads_test.go (route list)
- docs/specs/chat-shortcuts/spec.md (new), docs/specs/README.md index

## Verification
- `go vet ./...` clean; `go test ./... -count=1` in services/messaging all pass (domain, events, grpc, rest, store).
- `make build-check check-layering check-ngac check-docs` pass.
- Deny: 403 on all four star routes without read (domain + rest); 401 without session on all six routes (production router with JWT middleware); removed member sees no old stars/mentions; other workspace sees none.
- Route order: production router test shows `/api/messages/starred` hits the feed, not `:msgId`. Proxy: vite `/api/messages`+`/api/channels` -> 8183, Traefik PathPrefix, nginx `^/api/(channels|messages|...)` all cover it; no change needed.
- Dev messaging restarted (8183); live curl star/feed/unread/channel verified, test stars removed afterwards.

## Notes
- gRPC `GetChannel`/`GetUnreadCounts` unchanged in signature; `GetUnreadCounts` now fills `starred` (one extra query).
- No SQL index on `messages.mentions`; mentions query filters by readable channel ids with `mentions @>`. Fine at current scale; add GIN if the feed gets slow.

Status: DONE
Summary: Stars (conversations + messages), starred and mentions feeds, `starred` flags, migration 048, spec, tests and gates all done and live on dev.
