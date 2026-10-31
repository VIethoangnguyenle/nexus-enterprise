# Code Review: QR sign-in (feat/qr-login, `git diff main...HEAD`, 3 commits)

Date: 2026-11-01 · Reviewer: code-reviewer (read-only)

## Scope
- Backend: `backend/services/auth/internal/domain/qr_login.go`, `rest/qr_login.go`, `rest/handler.go` (wiring + `ErrQRUnavailable` → 410), tests.
- Frontend: `hooks/useQrLogin.ts`, `components/auth/{QrLoginPanel,QrCode,LoginScreen}.tsx`, `components/qr/*`, `lib/qr-login.ts`, `routes/{qr.$token,scan,auth.google.done}.tsx`, AccountSheet / NewChatButton rows.
- Specs: `docs/specs/tenant-auth-flow/spec.md`, `docs/specs/sign-in-screens/spec.md`.
- ~4.4k LOC added (≈1.1k is the mockup, ≈330 lockfile).

## Verification run
- `go vet ./...` (auth): clean.
- `go test ./internal/...` (auth) with `TEST_DATABASE_URL=…5433/ngac_ci`, `REDIS_ADDR/REDIS_URL/TEST_REDIS_URL=localhost:6379`: all pass, all 12 QR tests PASS (not skipped). Without Redis the domain QR tests SKIP and REST tests FAIL — known env requirement.
- Frontend: vitest on QR + AccountSheet tests 48/48 pass; eslint clean on touched files; `tsc --noEmit -p tsconfig.app.json` clean.

## Overall assessment
The core binding is sound: the token (in the QR) and the poll secret (httpOnly, SameSite=Strict, path-scoped cookie) are separate 256-bit secrets, the session is minted only for the poll secret holder, the code is deleted in the same WATCH/MULTI transaction that hands out the approval, scan/approve/decline are bound to the first scanner, approval re-checks active membership and that the tenant has not changed since the scan, and the phone routes are Bearer-only (no CSRF surface). Scanner URL filter (`qrTokenFromScan`) is strict (same origin, exact path, no query/hash, 43-char base64url) and the sessionStorage return path re-validates the token and only navigates to an internal route — no open redirect / injection found.

The problems are at the edges: a revoked session can mint a new one, the single cookie per browser breaks multi-tab use, QRLjacking mitigations are thin, and the status limiter is too tight for the NAT case the limiter was designed around.

---

## Critical
None found.

## High

### H1. A revoked session's access token can mint a fresh, persistent session (bypasses logout and the Google pre-hijack eviction)
- `rest/qr_login.go:112-114` (`qrPhone` drops `claims.SessionID`), `domain/qr_login.go:324-338` (`ApproveQRLogin`), `domain/qr_login.go:254-272` (`qrSession` → `issueSession` with a new SessionID, `StartedAt = now`).
- `RefreshStore.RevokeSession` (`domain/refresh.go:476-483`) documents its guarantee: "the access token already issued … stays valid for the remainder of its short TTL. What this guarantees is that no *further* access token can be minted." The QR path breaks that: the 15-minute access token of a logged-out / revoked session can approve a code and the computer receives a brand-new refresh session whose `StartedAt` is after any user cutoff, so `revokedForUser` (`refresh.go:542-559`) never fires.
- Failure scenario (realistic): `evictUnverifiedCredentialsExcept` (`google.go:155-165`) evicts a pre-hijacker when the real owner proves the address with Google. The attacker's script still holds an access token ≤15 min old; on the first refresh failure it calls `POST /api/auth/qr` from its own machine, then `scan` + `approve` with the stale Bearer token, then polls → new long-lived session. The account takeover survives the eviction. Same for "log out" on a stolen device.
- Note: `SwitchTenant` (`rest/handler.go:95-125`) has the same pre-existing gap (ends the session and issues a new one without checking it was still live); out of this diff's scope but worth fixing together.
- Fix: carry `SessionID` (and `IssuedAt`) in `QRPhone`; in `ApproveQRLogin` (and ideally `ScanQRLogin`) refuse unless the session family still exists (`EXISTS sessionKey(id)`) and `revokedForUser` is false for `{UserID, SessionID, StartedAt: iat}`. Expose this as one `RefreshStore.SessionLive(ctx, …)` method. Add deny tests: approve after `EndSession`, approve after `RevokeAllForUser`.

### H2. QRLjacking: an attacker-generated code relayed to a victim gets the victim's session; the only defence is a sentence of text
- `domain/qr_login.go:133-155` stores only the parsed User-Agent (attacker-controlled) and creation time; `QrConfirmScreen.tsx:191-213` shows "Chrome trên Windows · Tạo mã lúc HH:MM" plus a static warning.
- Scenario: attacker's server calls `POST /api/auth/qr` (public), renders the QR on a lookalike "Nexus sign-in" page or in a chat message ("scan to join the meeting room"), polls with its own cookie. Victim scans with the in-app scanner (origin check passes — it is a genuine Nexus code) or the phone camera, sees a plausible "Chrome on Windows", approves. Attacker's server gets a full session in the victim's tenant. The scanner's name and avatar are also leaked to the attacker on scan (`PollQRLogin` → `scanner`) before approval.
- This is the known top attack on QR login; Lark/WeChat/WhatsApp mitigate with location/IP of the requesting computer and network-mismatch warnings.
- Fix (in order of value): record the creating client's IP (`c.RealIP()`) at create; on scan, compare with the phone's IP and show a stronger, different warning when they differ (and/or show coarse location / network). Consider requiring the phone and computer to be on the same IP for tenants that opt in. Do not return `scanner` details to the computer until approval (or show initials only). Add an audit log line on approve naming actor, computer IP and UA (the spec requires actor + action + time for audit entries).

## Medium

### M1. One `qr_poll` cookie per browser: a second login tab silently kills the first; the phone then reports a sign-in that never happened
- `rest/qr_login.go:18-19, 36-46, 67`; `useQrLogin.ts:91-92` (410 → `expired`), `:113-126` (auto-renew calls `create` again).
- Scenario: access token expires with three Nexus tabs open → all redirect to `/login`, and because the chosen method is remembered in localStorage (`LoginScreen.tsx` `storedMethod`) all three open in QR mode → each `POST /api/auth/qr` overwrites the cookie. Only the last tab can collect. The user scans the QR in the tab in front of them (often not the last one), approves, phone shows "Đã đăng nhập trên máy tính" (`QrConfirmScreen.tsx:142-151`), the computer tab flips to "Mã đã hết hạn". Any auto-renew (every ~115 s) in a background tab also kills the foreground tab's code.
- Fix: name the cookie per code (e.g. `qr_poll_<first 16 hex of sha256(token)>`, same Path/attrs, MaxAge = TTL) and look it up by the token in the status body; or return the poll secret in the create body and keep it in hook memory (it never enters the QR, so the "photo of the QR" property holds). Add a REST test: two creates from one cookie jar, both codes still collectable.

### M2. Anyone who sees the QR can sign the victim's computer into the attacker's account (first scanner wins, no confirmation on the computer)
- `domain/qr_login.go:278-320` (first scanner owns the code), `useQrLogin.ts:106-110` (auto `login()` on approval).
- Scenario: shoulder-surfing, screen share, a projector, or nginx/browser-history leakage (L1). Attacker scans and approves within ~2 s; the victim's computer is silently signed into the attacker's workspace account, and documents/messages the victim then writes land in the attacker's tenant (login-CSRF class). The "X đã quét / Quét bằng điện thoại khác" veil only helps if the victim notices before the approval arrives.
- Fix: on the computer, after `approved`, show "Đăng nhập với tài khoản X?" with an explicit confirm before calling `login()` (cheap, and also a second QRLjacking-adjacent check); at minimum display the account name prominently after sign-in.

### M3. Status limiter is 120 req/min per address; five computers on the QR page behind one NAT exceed it
- `rest/qr_login.go:24-29`, `domain/ratelimit.go:22` (`DefaultPublicLimit` 60/min, whose comment explicitly targets "an office behind one NAT address signs in at nine").
- One page polls 30/min (2 s); 5 concurrent pages = 150/min → 429 for everyone on that address for the rest of the window. `useQrLogin` treats 429 as a generic failure (`reconnecting`), so nobody can complete a QR sign-in until the window resets; an approval in that window may expire uncollected.
- Fix: key the status limiter by (IP, token hash) or by the poll cookie, with a per-IP ceiling an order of magnitude higher; or raise `Max` to ~10× like the avatar limiter. Respect `Retry-After` in the hook.

### M4. Code is deleted before the session is minted; a failure after the delete loses the sign-in while the phone says it succeeded
- `domain/qr_login.go:229-249` (delete in the transaction, then `qrSession` → `GetUserByID`, `GenerateToken`), `rest/qr_login.go:92-99` (`issueSession` can still fail after that).
- Scenario: Postgres blip or refresh-store Redis error after the approved state was consumed → computer gets 500, next poll 410 → "Mã đã hết hạn"; phone already shows "Đã đăng nhập trên máy tính". Not a security issue (fails closed), but a confusing broken state.
- Fix: read the approved state, mint token + attach refresh first, then delete with a WATCH-guarded transaction; if the delete loses the race, revoke the just-minted session. Separately see L2 for the premature phone message.

## Low

### L1. The token does reach server logs; the spec claims it does not
- `lib/qr-login.ts:10-12` (`<origin>/qr/<token>`), `deploy/nginx/default.conf` (SPA `location /` with default `access_log`), spec line "Tokens travel in request bodies, never in a URL the server logs".
- When the phone camera opens the link, nginx logs `GET /qr/<token>`; the URL is also in the phone's browser history. A log reader within 2 min can scan first (DoS the real user, or M2). Low because the token alone never yields a session.
- Fix: correct the spec; optionally put the token in the fragment (`/qr#<token>`), which browsers never send to the server, and adapt `qrTokenFromScan` (it currently rejects any hash).

### L2. Phone success screen is shown on approve, not on collection
- `QrConfirmScreen.tsx:82, 142-151`. Covers: computer tab closed, cookie overwritten (M1), "Quét bằng điện thoại khác" renewed the code after the scan, renewal cap hit (`useQrLogin.ts:118-120` sets `expired` and stops polling ~5 s before the server TTL, so a scan in that window approves an orphan code), M4.
- Fix: word it as "Đã xác nhận — máy tính sẽ đăng nhập trong giây lát", or let the phone poll a `collected` flag.

### L3. Infra failures are reported as "expired"
- `domain/qr_login.go:134-135, 164-166` return `ErrQRUnavailable` when Redis is absent → 410 → computer shows "Mã đã hết hạn" / create shows 410. `ScanQRLogin` returns `ErrNotFound` (404) if the phone's user row vanished (`GetMe`), surfacing as the generic "failed".
- Fix: return `ErrUnavailable` (503) when `s.rdb == nil`.

### L4. Empty tenant name for graph-only members
- `domain/qr_login.go:372-374` returns `""` when `activeMemberRow` returns `(nil, nil)` (graph member without a `tenant_users` row). Phone renders "… đã vào  với tài khoản của bạn" (`QrConfirmScreen.tsx:148`) and an empty subtitle (`:205`). Fetch the tenant name from the tenant table or fall back to a generic noun.

### L5. Stored "qr" method follows the user onto the phone
- `LoginScreen.tsx` `storedMethod/storeMethod`. On a phone that once chose QR, the "Đăng nhập trên điện thoại trước" path lands on a QR code the phone cannot scan, and burns a code + cookie. Default to the form on coarse-pointer / small viewports.

---

## Test gaps (deny paths)
1. Approve with a tenant different from the one scanned (`qr_login.go:332`, `ScannerTenant != phone.TenantID`) — the only check that stops tenant switching between scan and approve; untested.
2. Re-scan by the same person with a tenant they are not a member of (re-scan rewrites `ScannerTenant`, `:311`).
3. Approve with a revoked session / after `RevokeAllForUser` (H1) — needs the fix first.
4. Concurrency: two goroutines scanning as different users, and two concurrent polls on an approved code — assert exactly one winner / one session. The WATCH logic is correct on reading but unproven by tests.
5. Expiry while scanned/approved: miniredis `FastForward` or a short TTL; assert `SetArgs{KeepTTL}` keeps the original deadline (a regression to plain `Set` would make codes immortal).
6. Decline then second poll → 410 (declined is told once).
7. REST: create twice from one jar (M1); status 429 behaviour; cookie `Secure` in non-dev `APP_ENV`.
8. Frontend: no test for 410 during `scanned`, renewal cap, or 429 handling in `useQrLogin`.

## Checks that came out clean
- Session binding to the creating browser (cookie HttpOnly, SameSite=Strict, Path `/api/auth/qr`, Secure outside dev; constant-time hash compare).
- Phone routes are Bearer-only (`httputil.JWTMiddleware`), so no CSRF; status route needs the Strict cookie, so a cross-site POST gets nothing.
- Token enumeration: 256-bit tokens, uniform 410, unknown tokens cost one Redis read before membership is asked.
- A non-member phone is refused at scan and at approve (`activeMemberRow` → `ErrAccessDenied`); a `(nil, nil)` row is a graph member, not a bypass.
- Scanner filter / sessionStorage return: strict origin + path + charset; navigation is to an internal typed route only.
- StrictMode double effects guarded by refs in both `useQrLogin` and `Confirm`; unmount stops the query and clears the renew timer.
- No UUIDs or tokens rendered on screen; no NGAC writes; `/api/auth` already in Vite proxy and nginx.
- Layering respected (REST → domain → Redis/store), no SQL in transport.

## Recommended actions (priority)
1. H1: session-liveness check on approve (and scan); deny tests. Track the SwitchTenant twin.
2. M1: per-code poll cookie (or in-memory secret).
3. H2 + M2: record creator IP, network-mismatch warning on phone, confirm-account step on the computer, audit log on approve.
4. M3: status limiter keyed per code / higher ceiling; honour Retry-After.
5. M4/L2: mint-then-delete; honest phone wording.
6. Add the test gaps above; fix the spec's logging claim (L1).

## Unresolved questions
- Is QRLjacking acceptable product risk with text-only mitigation (H2), or should approval require same-network? Product decision.
- Should the computer require an explicit "continue as X" click (M2)? It changes the approved mockup flow.
