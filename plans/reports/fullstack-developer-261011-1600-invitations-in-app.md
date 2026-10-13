# Invitations in-app (offers outside Chọn workspace)

## PART A - workspace menu row + dot (frontend only) - DONE

Files:
- frontend/src/hooks/useInvitations.ts - new `useOpenInvitationCount()`: `/me` verified gate (`email_verified` + email), shares the `invitations` query key, refetch every 60 s + on window focus, 0 when unverified (no request sent).
- frontend/src/components/patterns/AppSidebar.tsx - "Lời mời" link row (count pill, 99+, sr text "N lời mời đang chờ") in the workspace menu, to `/workspace-select`; accent dot on the workspace initial; ", có lời mời" in rail aria-label / hidden text.
- frontend/src/components/patterns/MobileMoreSheet.tsx - phone: dot on the workspace row, "Lời mời" row (min-h-14, 56px target) in the workspace list view, closes the sheet and navigates.
- frontend/src/components/patterns/AppSidebar.test.tsx, MobileNav.test.tsx - tests: row + count + navigates + dot, 99+, hidden at 0, hidden and no `/invitations` request when unverified.

Gates: vitest 177 files / 2263 tests pass; eslint clean; typecheck:diff no new errors. Build skipped as instructed.

## PART B - notification on invite (backend + frontend) - DONE (in-app); web push NOT done

Path reused: messaging's Kafka notification consumer (`CreateNotification` -> row + live WS push), the same one asset/approval events use. Workspace service gets a small producer on a new topic `workspace.invitation` (same pattern as approval/asset producers; no new pipeline, no new consumer). Notification `user_id` = `users.id`, matching `GET /api/notifications`.

Web push skipped, on purpose: `pkg/push`/OneSignal is wired only into messaging's chat `Service` (recipient rules, read-OA check, chat-kind prefs, quiet hours); there is no per-user push entry point for non-chat events. Adding one means a new pref kind + new wiring, i.e. a new pipeline, so I stopped there. Stated in the notifications spec.

Rules: notice fires only when the offer is newly opened (not a refresh), only for accounts whose `users.email_verified_at` is set (looked up in the workspace service; unknown/unverified address publishes nothing), after commit and off the request, so the 202 and timing are unchanged. Record carries only the addressed `users.id`; notification list/count/push are caller-scoped already.

Files, backend:
- backend/services/workspace/internal/store/invitation_store.go - `Invitation.Created` (insert vs refresh via `xmax = 0`), `VerifiedUserIDsByEmail`.
- backend/services/workspace/internal/domain/invitations.go - store interface method, calls `notifyInvitee` after email.
- backend/services/workspace/internal/domain/invitation_notice.go (new) - `InvitationNotifier`, `WithInvitationNotifier`, `notifyInvitee`.
- backend/services/workspace/internal/domain/service.go - `notifier` field.
- backend/services/workspace/internal/events/invitation_producer.go (new) - Kafka producer, nil-safe.
- backend/services/workspace/cmd/main.go, go.mod - wiring; franz-go now a direct dep.
- backend/services/messaging/internal/events/consumer.go - consumes `workspace.invitation`, `handleInvitationEvent`.
- Tests: workspace domain/invitations_test.go (fake), domain/invitation_notice_test.go (new: verified -> notice; unknown/unverified -> none; only addressed account; refused invite -> none; re-invite -> once), store/invitation_store_test.go, events/invitation_producer_test.go (new), messaging events/notifications_from_events_test.go (addressed account only; empty inviter; no account/offer/garbage -> nothing).

Files, frontend:
- frontend/src/lib/notification-copy.ts - `workspace_invitation`: shows server body (names only) with fallback, domain "Lời mời", target `/workspace-select` (only this type shows body; others unchanged).
- frontend/src/components/patterns/NotificationPanel.tsx - Mail icon, link to `/workspace-select`.
- Tests: lib/notification-copy.test.ts, components/patterns/NotificationPanel.test.tsx (no id on screen, navigates, marks read).

Specs updated: docs/specs/notifications/spec.md, docs/specs/workspace-admin-authorization/spec.md, docs/specs/sign-in-screens/spec.md (Part A requirement).

Gates (final, after both parts): vitest 177 files / 2266 tests pass; eslint clean; typecheck:diff no new errors; workspace + messaging `go vet` clean, `go test ./...` pass (DB tests ran against the dev DB); `make check-layering` ok; `make build-check` ok. No schema change, so no db-migrate. No restarts, no commits, git index untouched.

Notes / open:
- Live check needs the workspace and messaging services restarted (not done, per instruction) and Redpanda up; the new topic is auto-created.
- Notification arrives live over WS; the menu row/dot refresh within 60 s or on window focus (no WS hook for invitations).
- A re-invite of an already open address does not notify again (anti-spam); the email resend window is separate.

Status: DONE_WITH_CONCERNS (web push intentionally not built; no end-to-end run against live services).
