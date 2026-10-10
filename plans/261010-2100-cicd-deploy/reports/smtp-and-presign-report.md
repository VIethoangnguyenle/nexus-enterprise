# SMTP OTP sender and HTTPS presigned URLs (2026-10-10)

Nothing committed, staged or pushed; no server contact.

## 1. SMTP OTP sender (auth)
- New `auth/internal/domain/smtp_sender.go`: `SMTPSender` (implements `CodeSender` + `OwnershipProver`, so `otp_proves_email` is true and verified-address logic applies; fixed-code mode still never verifies). Stdlib only (net/smtp, crypto/tls, mime, quotedprintable); go.mod untouched by this work.
- TLS: `starttls` (default) or `tls` (implicit; default for port 465). No plaintext mode; missing STARTTLS is refused before AUTH (no credential downgrade). Cert verified against ServerName=host.
- Timeouts: dial 10s, whole send 30s (conn deadline), both overridable in config.
- Message: multipart/alternative (text then HTML, quoted-printable, utf-8), CRLF only, RFC 2047 Subject "Mã đăng nhập Nexus: 123456", Date, Message-ID (From domain), MIME-Version, Auto-Submitted. Says it expires in 5 minutes (from `otpTTL`) and to ignore if not requested.
- Header injection: recipient must be a bare addr-spec (control chars, whitespace, `<>,;:"\()[]`, display names all refused, no connection opened); From validated at startup (line breaks refused).
- Errors: wrapped without code/body/password; REST maps them to the generic 500 (cause only in log), independent of account existence. Existing `RequestOTP` deletes the session on send failure.
- `cmd/main.go`: `SMTP_HOST` set -> SMTP sender (wins over dev LogSender); set but incomplete -> refuse to start (no secrets echoed). Fixed code still takes precedence if set.
- Tests: `smtp_sender_test.go` (in-process fake SMTP with real STARTTLS and implicit TLS, self-signed CA): both TLS modes format/envelope/auth; code+password never in logs or errors; 7 injection inputs refused with zero connections; hang timeout, refused connection, no-STARTTLS downgrade, untrusted cert; failed delivery leaves no session; fixed-code mode never verifies and sends no mail; config validation. `cmd/main_test.go`: env wiring.
- Compose (auth only) + `env.example`: `SMTP_HOST/PORT/USERNAME/PASSWORD/FROM/TLS` placeholders, all empty by default. `docs/deployment.md`: Gmail steps 1-5 (2SV, app password, smtp.gmail.com:587, ~500/day and From rule, values only in /opt/nexus/.env).

## 2. HTTPS presigned URLs (document)
- New `storage.NewPresignClient(PublicEndpoint{Host, Secure}, ak, sk)` (region fixed `us-east-1`, path-style, no network call; test uses a cancelled context to prove it). `cmd/main.go` uses it with `MINIO_PUBLIC_ENDPOINT` + new `MINIO_PUBLIC_SECURE` (default false). Internal client unchanged.
- Tests: https + public host + path-style + signed; dev default http `localhost:9100`; path in host refused.
- Compose document: `MINIO_PUBLIC_SECURE: "true"` (endpoint already set). Traefik `nexus-storage` router verified: Host(`storage.nexus.zaneng.xyz`) -> service port 9000 (API, not 9001 console), websecure + le, priority 150; Host header forwarded (passHostHeader default true), MinIO has CORS origin for the site. Router comments updated; the "Known gap" doc section replaced by a "Presigned file URLs" section.

## Verification
- `make build-check`, `make check-ngac`, `make check-layering`: pass.
- `make test s=auth` and `s=document`, strict, on `ngac` and `ngac_ci`: exit 0, 0 skips, 0 fails (4 runs).
- `docker compose -f deploy/docker-compose.prod.yml --env-file deploy/env.example --profile tools config`: pass; SMTP_* empty, MINIO_PUBLIC_SECURE "true", 0 `ports:`.
- Dev with envs unset: SMTP_HOST unset -> otpOptions unchanged (fixed 999999 / LogSender in dev); MINIO_PUBLIC_SECURE unset -> http as before.

## Not verified
Real Gmail delivery (needs the user's app password), Traefik/TLS on the server, an actual browser upload.

Status: DONE_WITH_CONCERNS
Summary: SMTP sender and https presign client implemented with tests, compose/env/docs wired; all gates green on both databases.
Concerns:
- Pre-existing: the code default `MINIO_PUBLIC_ENDPOINT=localhost/storage` is rejected by minio-go ("cannot have fully qualified paths"), so the document service cannot start without that env (dev sets it in .env.dev). Left unchanged per "dev behaviour unchanged"; now a clear startup error mentions the host.
- `deploy/server-bootstrap.sh` (not mine) does not write SMTP_* keys; compose defaults make them optional and docs say to append them. Add them to the bootstrap if wanted.
- Gmail app password shown with spaces: docs say remove them; a `$` needs `$$` in .env.
- Auth `go.mod` shows a pre-existing diff (x/crypto direct -> indirect) from earlier work, not from this change.
- Request-time latency: a send blocks the OTP request up to 30s on a slow SMTP server (synchronous by design to keep the "no session on failure" invariant).
