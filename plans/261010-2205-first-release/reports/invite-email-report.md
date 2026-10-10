# Invitation email report

## What changed
- `backend/pkg/mailer` (new): the SMTP sender moved out of auth, behaviour unchanged (STARTTLS/implicit TLS only, dial 10s/send 30s timeouts, recipient guard, CRLF, Date/Message-ID/MIME headers, no secret logging). Added: `Message{To,Subject,Text,HTML}`, `FromEnv()` (nil when SMTP_HOST unset, error on partial config, never echoes the password), `MaskEmail`, and subject line breaks folded to spaces before RFC 2047 `mime.QEncoding`.
- `backend/testutil/fakesmtp.go` (new): fake SMTP server shared by the mailer, auth and workspace tests.
- auth: `smtp_sender.go` replaced by `email_code_sender.go` (`EmailCodeSender` wraps a `Mailer`; the OTP message is built in auth). `cmd/main.go` uses `mailer.FromEnv`. Generic sender tests moved to `pkg/mailer/mailer_test.go`; the OTP message tests are in `email_code_sender_test.go`.
- workspace: `domain/invitation_mail.go` (async send, claim, release, message). `InviteByEmail` calls `emailInvitation` after the upsert. `cmd/invite_mail.go` wires SMTP_* and APP_BASE_URL. It refuses to start on a partial SMTP config or a non-http(s) URL, and drains in-flight mail on shutdown.
- Store: `UpsertInvitation` now sets `inv.ID`. New `ClaimInvitationEmail` (one conditional update) and `ReleaseInvitationEmail`.
- Migration `data/migrations/036_invitation_last_emailed_at.sql` adds `last_emailed_at`.
- Deploy: `docker-compose.prod.yml` (workspace gets APP_BASE_URL and SMTP_*), `check-compose-env.sh` (asserts workspace and auth carry identical SMTP_*/APP_BASE_URL), `env.example` comment, `docs/deployment.md`.
- Specs: `admin-screens` (Status line), `workspace-admin-authorization` (requirement text, three scenarios, the Status note that said "Nothing sends an email").

## Design notes
- Everything after the upsert runs in a goroutine: the claim, the name lookups, the send. The context is `WithoutCancel(request ctx)`, which keeps the caller identity that the policy gRPC calls need. It has a 45s timeout and 16 in-flight slots, with a recover. The request path never consults accounts, so the 202 and its timing are identical for every address. Tested with a hanging SMTP server: the answer returns in under 1s.
- Resend window is 10 min (`DefaultInviteResendWindow`, `WithInviteResendWindow`). The claim is atomic, so two invites at once send one email. A failed send releases the slot.
- Logs: a warn with the masked address (`mo***@domain`). The error text has the full address replaced, because server refusals often echo it. The body is never logged. With SMTP unset: an info line.
- Link is `APP_BASE_URL` without a token. Expiry date is written in UTC+7 (fixed zone, no tzdata needed).

## Tests
- New: fake-SMTP end to end (right recipient, names, link), no UUID/ids in the email, 202 and invitation kept on SMTP failure, hanging SMTP does not hold the answer, account vs non-account identical, resend window, failed send does not burn the window, CRLF/markup in workspace or inviter name neutralised, masked logs, SMTP-off info log, store claim (incl. concurrent, expired, revoked), env/URL validation.
- `make test s=auth` and `s=workspace` on ngac and ngac_ci: rc 0, 0 skips, 0 fails. The pkg/testutil tests pass on both DBs. `build-check`, `check-ngac`, `check-layering`, `check-docs` and `deploy/check-compose-env.sh` pass, and the prod compose renders. `make lint`: pkg, auth and workspace are clean. Messaging fails vet (`c.handleRecord` undefined in `notifications_from_events_test.go`), which belongs to the notifications agent.
- The migration was backed up first (`pg_dump -Fc` of ngac and ngac_ci into the scratchpad), `make db-migrate` ran, and 036 was applied to ngac_ci; the column was verified in both.

## Coordinator note: the `invitation_created` event
- Not mine. `s.announce(ctx, realtime.KindInvitationCreated, ws.ID, inv.ID)` in `InviteByEmail` and the constant in `pkg/realtime/event.go` were added concurrently by the notifications agent. I only placed my `emailInvitation` call after it. The two failing realtime tests were failing from their in-progress work; they pass now (the other agent fixed them), and I did not edit those tests.
- Payload as emitted: `Domain=workspace`, `Kind=invitation_created`, `TenantID` and `WorkspaceID` = workspace id, `IDs=[invitation id]`, `ActorUserID` = inviter's user id. It carries no invitee email or account, so it does not leak the address over realtime. Messaging has to resolve the stored invitation itself. It also carries the workspace id, which any subscriber to the workspace channel sees, so the notifications agent should keep the event off the workspace-wide fan-out or deliver only to the resolved recipient.

## Concerns
- I ran `git rm --cached` on `auth/.../smtp_sender.go` once by mistake, which staged a deletion. I reverted it at once with `git reset -q HEAD -- <that file>`; `git status` shows the index entry as before. The file itself is deleted from the worktree (the move). I touched nothing else in the index.
- Dev `docker-compose.yml` and `.env.example` do not pass SMTP_* even for auth, so I left them alone. In dev, native or docker, invitation emails are skipped with an info log.
- `go test -race` on the domain package trips on concurrent-test fakes (TestRemoveOwner_Concurrent..., TestAcceptInvitation_TwoAtOnce); this is unrelated to the mail code, and my new tests are race-clean.
- Gmail's ~500 messages/day now covers sign-in codes and invitations together.

Status: DONE_WITH_CONCERNS
Summary: Invitation email works end to end through the shared `pkg/mailer`; migration 036 is applied to ngac and ngac_ci; specs, deploy and docs are updated; auth and workspace tests are green with zero skips on both DBs.
Concerns: the messaging lint failure and the `invitation_created` event belong to the notifications agent (payload above); the accidental index touch is reverted; dev compose does not carry SMTP.
