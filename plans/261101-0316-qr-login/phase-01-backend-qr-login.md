# Phase 01 — Backend: QR login domain + REST

Service: `backend/services/auth`. TDD: every step starts with a failing test.

## State (Redis)

Key `qr_login:<sha256(token)>`, TTL 120 s, JSON:

```
{ status: "pending" | "scanned" | "approved" | "declined",
  poll_hash:  sha256(poll secret),          // binds the session to the creating browser
  browser, os, created_at,                   // parsed once from the creating request's User-Agent
  scanner?:  { user_id, tenant_id },         // set by scan
  identity?: RefreshIdentity-like fields }   // set by approve (user, username, ngac node, tenant)
```

Transitions run in one Lua script (or WATCH/MULTI) so two phones cannot race:
`pending → scanned` (any signed-in phone; a re-scan by the **same** user is idempotent, a
different user gets `qr_unavailable`), `scanned → approved | declined` (only the scanner),
`approved → (deleted)` on the desktop's claiming poll via GetDel. Model on
`googleauth/flowstore.go` (hash the key, atomic take).

## Domain (`internal/domain/qr_login.go`)

- `CreateQRLogin(ctx, userAgent) (token, pollSecret string, expiresIn time.Duration, err)`
- `QRLoginStatus(ctx, token, pollSecret) (QRStatus, *SigninResult, err)` — wrong/missing poll
  secret → `ErrQRUnavailable`; on `approved` it claims (GetDel), then mints the session exactly
  like `SwitchTenant` does: `auth.GenerateToken(userID, username, ngacNodeID, tenantID)` and
  returns the identity for `AttachRefreshToken`.
- `ScanQRLogin(ctx, token, claims) (QRDevice{Browser, OS, CreatedAt}, err)`
- `ApproveQRLogin(ctx, token, claims) error` — re-check `activeMemberRow(claims.UserID,
  claims.TenantID)`; inactive → `ErrTenantAccessDenied` (existing error if one fits).
- `DeclineQRLogin(ctx, token, claims) error`
- `parseUserAgent(ua) (browser, os string)` — small table (Edge before Chrome, Chrome, Firefox,
  Safari; Windows, macOS, Linux, iOS, Android, ChromeOS); unknown → "" and the UI prints
  "Trình duyệt không rõ".
- Redis nil (auth tolerates it) → `ErrQRUnavailable`, never a panic.
- New domain errors map through `httputil.MapDomainError`: `qr_unavailable` 410,
  `tenant_access_denied` 403.

## REST (`internal/rest/qr_login.go`, routes in `handler.go`)

| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/api/auth/qr` | public, `publicLimiter("qr-create")` | sets HttpOnly cookie `qr_poll` (Path `/api/auth/qr`, SameSite Strict, Secure outside dev, Max-Age 120) — reuse the cookie helpers in `rest/session.go`; body `{token, expires_in}` |
| GET | `/api/auth/qr/status?token=` | public, `limiterAt("qr-status", 120/min)` | `{status, scanner?: {display_name, avatar_url}}`; on approved: `issueSession` + `{status:"approved", access_token, user}` (same shape as OTP verify) and clear `qr_poll` |
| POST | `/api/auth/qr/scan` | JWT (`api` group: `/auth/qr/scan`) | body `{token}` → `{browser, os, created_at, tenant_name}` |
| POST | `/api/auth/qr/approve` | JWT | body `{token}` → 204 |
| POST | `/api/auth/qr/decline` | JWT | body `{token}` → 204 |

Token goes in the query only for the status poll (same-origin, never logged by our access log
format — verify `deploy/nginx` log format and the Echo logger skip list; if the query is logged,
move the token to a header `X-QR-Token`).

`/api/auth` is already proxied in `frontend/vite.config.js:44` and `deploy/nginx/default.conf:123`;
no new proxy entry. `/qr/<token>` is a SPA route served by the existing SPA fallback — verify
both nginx configs (`frontend/nginx.conf`, `deploy/nginx/default.conf`).

## Tests (`internal/domain/qr_login_test.go`, `internal/rest/qr_login_test.go`)

Use the existing Redis test setup (`TEST_DATABASE_URL`, Redis env vars — see memory note).
Allow and **deny** for each path:

1. create → status pending; status without/with wrong poll secret → `qr_unavailable`.
2. scan by A → status shows A; scan by B after A → `qr_unavailable`; re-scan by A ok.
3. approve by non-scanner → denied; approve after member removed → `tenant_access_denied`, no session.
4. approve → status returns session with `TenantID == scanner's tenant`, refresh cookie set; second status → `qr_unavailable` (single use).
5. decline → status declined; approve after decline → `qr_unavailable`.
6. expired (TTL) → every endpoint `qr_unavailable`.
7. unknown token on scan/approve/status → identical response body.
8. `parseUserAgent` table test.

## Done when

`make test s=auth`, `make build-check`, `make check-layering` green.
