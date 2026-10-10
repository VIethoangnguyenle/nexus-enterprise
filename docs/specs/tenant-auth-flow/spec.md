# tenant-auth-flow

## Purpose
Get a user from credentials to a tenant-scoped session, and let them move between tenants, without ever issuing a token that is ambiguous about which tenant it speaks for.

## Requirements

### Requirement: There is no password sign-up or sign-in
People sign in with Google or a one-time code, over REST. `POST /api/auth/signup`, `/signin`, `/register` and `/login` do not exist, nor do the gRPC `Register`, `Login`, `Signup`, `Signin` and `ListUsers`; accounts created before this change keep their `users.password` column, which nothing reads, and Google linking or the first proof of the address clears it. Because there is no password, there is no password timing or "no such account" oracle to defend.

#### Scenario: Removed routes
- **WHEN** a client posts to `/api/auth/signup`, `/api/auth/signin`, `/api/auth/register` or `/api/auth/login`
- **THEN** it gets no sign-in (404 or 405, or 401 behind the `/api` session check), never 200 or 201

### Requirement: Domain auto-join requires proof of email ownership
The system SHALL add a user to a tenant on the basis of that tenant's `domain` only when it holds proof that the user controls an address at that domain. Today the only accepted proof is a Google Workspace sign-in whose verified ID token carries `hd` equal to the domain (see "Google sign-in resolves the company from the hosted domain only"). OTP sign-in, and a consumer Google account (no `hd`), SHALL NEVER join a tenant by email domain and SHALL NEVER claim a domain. (OTP is excluded: in its default fixed-code test mode the code is the same for everyone and proves nothing. It MAY qualify once random codes are delivered to the email by a real sender and that mode is the only one enabled.) Otherwise a user joins a company tenant only through an invitation.

#### Scenario: Consumer Google account at a claimed domain
- **WHEN** a user signs in with Google as `x@acme.com` without `hd` and a tenant has `domain = 'acme.com'`
- **THEN** the user is NOT added to that tenant
- **AND** a personal workspace is created with the user as `owner` and a #general channel is auto-provisioned
- **AND** the personal workspace does not claim `acme.com`

#### Scenario: Consumer Google account at an unclaimed domain
- **WHEN** a user signs in with Google as `bob@newcorp.com` without `hd` and no tenant has `domain = 'newcorp.com'`
- **THEN** a personal workspace is created with the user as `owner`
- **AND** no tenant claims `newcorp.com`

#### Scenario: OTP at a claimed domain
- **WHEN** a new user is created by OTP verification of `x@acme.com`, and a tenant has `domain = 'acme.com'`
- **THEN** the user is NOT added to that tenant

#### Scenario: Google Workspace sign-in at a claimed domain
- **WHEN** a user signs in with Google and the verified ID token has `hd = 'acme.com'`, and a tenant has `domain = 'acme.com'`
- **THEN** the user is added to that tenant as a `member`

### Requirement: Public email domains never place a user in a tenant
The system SHALL keep one list of public mailbox domains (gmail.com, googlemail.com, outlook.com, hotmail.com, live.com, yahoo.com, icloud.com, proton.me, protonmail.com, gmx.com, yandex.com, mail.ru, aol.com, zoho.com and similar) in `backend/services/auth/internal/domain/public_domains.go`. A domain on that list SHALL never auto-join a user to a tenant and SHALL never be claimed as a tenant's `domain`, on any sign-in path. Domains are compared case-insensitively, and `workspaces.domain` is unique case-insensitively.

#### Scenario: Public domain on any path
- **WHEN** a user signs in as `victim@gmail.com`, or signs in with Google with `hd = 'gmail.com'`, and a tenant has `domain = 'gmail.com'`
- **THEN** the user is NOT added to that tenant
- **AND** no tenant ever claims `gmail.com`

### Requirement: Sign in with Google
The system SHALL support OpenID Connect sign-in with Google using the authorization-code flow with PKCE (S256), `state` and `nonce`, run entirely in the auth service. `GET /api/auth/google/start` stores the nonce and PKCE verifier server-side (Redis, 10 minutes, single use) keyed by `state`, binds `state` to the browser in an httpOnly `SameSite=Lax` cookie scoped to `/api/auth/google`, and redirects to Google with scopes `openid email profile` (and an optional, validated `login_hint`). `GET /api/auth/google/callback` SHALL accept the result only when the query `state` equals the cookie, the stored flow exists and is consumed, the code exchange succeeds, and the ID token verifies: RS256 signature against Google's JWKS (cached, refetched on unknown key), `iss` ∈ {`https://accounts.google.com`, `accounts.google.com`}, `aud` = the client ID, not expired, `nonce` equal to the stored nonce, a `sub`, an `email`, and `email_verified = true`. Any other outcome SHALL redirect to `/login?error=<code>` without creating or changing an account and without issuing a session.

#### Scenario: State does not match the browser
- **WHEN** the callback's `state` differs from the state cookie, the cookie is absent, or the state was already used or has expired
- **THEN** the code is not exchanged and the browser is sent to `/login?error=google_state`

#### Scenario: ID token fails verification
- **WHEN** the ID token has a bad signature, a wrong `aud`, a wrong `iss`, has expired, or carries a different `nonce`
- **THEN** the browser is sent to `/login?error=google_failed` and no account is touched

#### Scenario: Email not verified by Google
- **WHEN** the ID token has `email_verified` false or absent
- **THEN** the browser is sent to `/login?error=google_unverified`
- **AND** the email is neither used to create an account nor matched against an existing one

#### Scenario: User cancels at Google
- **WHEN** Google returns `error=access_denied`
- **THEN** the browser is sent to `/login?error=google_cancelled`

#### Scenario: Google sign-in not configured
- **WHEN** `GOOGLE_CLIENT_ID` (or the secret, or Redis) is unavailable
- **THEN** `GET /api/auth/providers` returns `{"google": false}`, `GET /api/auth/google/start` returns 503, and the login page hides the Google button

#### Scenario: Successful sign-in
- **WHEN** the callback is accepted
- **THEN** the system issues the same session as OTP sign-in — an access token scoped to the default tenant and a rotating refresh token in the httpOnly `SameSite=Strict` cookie bound to the same session
- **AND** redirects to `APP_BASE_URL + /auth/google/done` with no token in the URL; that page obtains the access token from `POST /api/auth/refresh`, loads `/api/me`, and continues to workspace selection

### Requirement: Google accounts are identified by subject
The system SHALL identify a Google user by the ID token's `sub`, stored in `user_identities (provider, subject, user_id, email, created_at, last_login_at)` with primary key `(provider, subject)` and at most one identity per provider per user. A linked subject never moves to another account.

#### Scenario: Returning Google user
- **WHEN** a `sub` that is already linked signs in, even with a changed email
- **THEN** the linked user is signed in and no account is created

#### Scenario: Existing account with the same verified email
- **WHEN** an unlinked `sub` signs in with a verified email that belongs to an existing user who has no Google identity yet
- **THEN** the identity is linked to that user and no duplicate account is created
- **AND** the user keeps their existing tenants
- **AND** every credential that was set up without proof of the email is evicted first (see "Google linking evicts unverified credentials")

#### Scenario: Returning user is not evicted
- **WHEN** a `sub` that is already linked signs in
- **THEN** the user's password and existing sessions on other devices are left untouched

### Requirement: Google linking evicts unverified credentials
Nothing else in the system verifies email ownership, so an account found by email may have been created by someone other than the address's owner (pre-hijacking: an attacker signs in with the victim's address through the test-only fixed code, or holds a credential from before passwords were removed, then waits). When Google sign-in links a verified identity to such an existing account, the system SHALL treat Google as the proof of ownership and, before linking, SHALL clear the account's password and revoke all of its existing sessions on every device, then write an audit log line naming the user id only (never the email). If either step fails, the sign-in SHALL fail rather than link. Session revocation records a per-user cutoff in Redis for the refresh-token lifetime; `POST /api/auth/refresh` rejects any session that started before the cutoff, including sessions created before session start times were recorded. Access tokens already issued remain valid until they expire (at most 15 minutes).

#### Scenario: A credential set before the proof is dropped
- **WHEN** an account holds a password from before the address was proved and the owner then signs in with Google as that address for the first time
- **THEN** the stored password is cleared

#### Scenario: Pre-existing sessions are ended
- **WHEN** a refresh token was issued for that account before the Google link
- **THEN** presenting it to `POST /api/auth/refresh` is rejected
- **AND** the session issued by the Google sign-in itself refreshes normally

#### Scenario: Email already linked to a different Google subject
- **WHEN** an unlinked `sub` signs in with an email whose user is already linked to another Google `sub` (e.g. a deleted Workspace account's address reassigned)
- **THEN** the sign-in is rejected with `/login?error=google_conflict`

### Requirement: Google sign-in resolves the company from the hosted domain only
The system SHALL place a Google user in a company tenant only on the basis of the ID token's `hd` (hosted domain) claim, which Google sets only for Google Workspace accounts. The email's domain SHALL never be used for this on the Google path. It is the only sign-in path that joins a tenant by domain (see "Domain auto-join requires proof of email ownership").

#### Scenario: Hosted domain matches a tenant
- **WHEN** a Google user with `hd = 'acme.com'` signs in and a tenant has `domain = 'acme.com'`
- **THEN** the user is added to that tenant as a `member` (an existing membership is left unchanged, including a disabled one) and the session is scoped to it

#### Scenario: Hosted domain has no tenant yet
- **WHEN** a Google user with `hd = 'newco.io'` signs in and no tenant has that domain
- **THEN** a new tenant named after the domain is created with the user as `owner`, provisioned like workspace creation (tenant NGAC UAs, `#general` channel), and claims `domain = 'newco.io'` with type `organization`
- **AND** later users with the same `hd` join it as `member`

#### Scenario: Consumer Google account
- **WHEN** a Google user without `hd` (e.g. `someone@gmail.com`) signs in
- **THEN** the user is never auto-joined to any tenant by domain, even if a tenant claims `gmail.com`
- **AND** a new user gets a personal workspace as `owner`, as an OTP user with no company does; an existing user keeps the tenants they already belong to

#### Scenario: Company email without a hosted domain
- **WHEN** a Google account registered on `bob@acme.com` without Google Workspace (no `hd`) signs in and a tenant has `domain = 'acme.com'`
- **THEN** the user is NOT added to that tenant

### Requirement: Email addresses are normalised, unique and proved
Addresses SHALL be trimmed and lower-cased at sign-in and OTP, and an address that is not well formed is rejected (400). `users.email` is unique case-insensitively (unique index on `lower(email)`, migration 030), so a case variant of an existing address signs in to that account and never creates another. `users.email_verified_at` records proof of ownership and is set only by: Google sign-in with `email_verified` true (new, linked or returning account), and an OTP code that a real sender delivered to the address (`DeliversToOwner`). Fixed-code test mode and the log sender never set it, since anyone can complete those. The first proof on an existing account evicts its unverified credentials, as Google linking does. Phone identifiers carry no email proof. Other services treat an address as the account's only when it is verified (workspace invitations).

#### Scenario: Fixed code does not verify
- **WHEN** a user signs in through the fixed test code with an email address
- **THEN** the session is issued but the account's email stays unverified

#### Scenario: Case-variant address
- **WHEN** `Victim@Acme.com` asks for a code while `victim@acme.com` has an account
- **THEN** the code signs in to that account and no second account is created

#### Scenario: Delivered code proves the address
- **WHEN** a code delivered by a real sender is verified for an address
- **THEN** `email_verified_at` is set and any earlier password on that account is cleared

### Requirement: OTP sign-in codes
`POST /api/auth/otp/request` opens a 5-minute session for an email or phone and `POST /api/auth/otp/verify` exchanges its code for the same session as other sign-ins. The code's mode is set by `AUTH_FIXED_OTP_CODE`:
- **Fixed-code test mode (default).** Unset → `999999`; any six digits → that code. Every OTP session accepts that code, so anyone who knows it can sign in as any email or phone. This is a documented TEST-ONLY mode for testers on deployed builds, and the auth service logs a warning at startup while it is on. `GET /api/auth/providers` reports `otp: true, otp_fixed_code: true, otp_proves_email: false`. The flag is for operators and API users: no screen prints the code or a "test mode" line, in any build (see `sign-in-screens`).
- **Random-code mode.** Setting `AUTH_FIXED_OTP_CODE` explicitly empty (`AUTH_FIXED_OTP_CODE=`) turns the test mode off. Each request then gets a uniformly random six-digit code from `crypto/rand` (rejection sampling, no modulo bias), stored only as an HMAC-SHA256 keyed by a secret derived from the JWT secret and bound to the session, compared in constant time, and handed to a `CodeSender` for delivery. If delivery fails, the request fails and the session is removed. The only sender today is a log sender that runs only when `APP_ENV=dev` or `AUTH_DEV_OTP=1` and refuses otherwise. With no sender, OTP is disabled: request and verify return 503, `/api/auth/providers` reports `otp: false`, and the login page hides the code form. `otp_proves_email` is true only when random codes are delivered by a sender that declares it reaches only the owner of the address; the fixed code and the log sender never make it true.

In both modes: at most 5 verification attempts per session (counted atomically, the 6th fails even with the right code), a code works once, at most 5 code requests per identifier per 15 minutes (429), and the service never logs the code.

A wrong code answers `401 {"code":"otp_invalid","attempts_left":N}`, where N is what is left of this session's 5 tries; an unknown or expired session is `401 otp_expired`; the 6th try is `429 otp_too_many_attempts`. A refused code request is `429 otp_rate_limited` with `Retry-After` (seconds) and `retry_after_seconds` in the body. The count and the wait belong to the session and the identifier, never to an account, so nothing in them says whether an account exists.

#### Scenario: Default configuration accepts the fixed test code
- **WHEN** `AUTH_FIXED_OTP_CODE` is unset and a user requests a code and verifies with `999999`
- **THEN** the user is signed in

#### Scenario: Fixed code turned off
- **WHEN** `AUTH_FIXED_OTP_CODE` is set empty and a user verifies with `999999`
- **THEN** verification fails with "invalid otp code" unless `999999` happens to be the code that was delivered
- **AND** each request produces a different random code, and Redis holds only its hash

#### Scenario: OTP disabled
- **WHEN** the fixed code is off and no code sender is configured (not dev mode)
- **THEN** `POST /api/auth/otp/request` returns 503 and `/api/auth/providers` reports `otp: false`

#### Scenario: Too many attempts
- **WHEN** a session has received 5 wrong codes
- **THEN** the next verification fails with "too many attempts", even with the right code

#### Scenario: Too many requests
- **WHEN** a sixth code is requested for the same identifier within 15 minutes
- **THEN** the request is rejected with 429, `Retry-After` says how long is left of the window, and other identifiers are unaffected

#### Scenario: Wrong code says how many tries remain
- **WHEN** a code is wrong on the first try of a session
- **THEN** the answer is 401 with `attempts_left` 4, and the same for an address that has an account and one that has none

#### Scenario: Same answer for known and unknown addresses
- **WHEN** a code is requested for an address that has an account and for one that has none
- **THEN** both answers are 200 with the same shape; only a successful verification says whether the account is new (`is_new_user`)

### Requirement: Sign-in issues a session scoped to a default tenant
A successful sign-in (OTP or Google) SHALL issue an access token scoped to the person's default tenant (an owned tenant, else the first) and a refresh token that names the SAME tenant, so that trading the cookie for a new access token after a reload keeps the workspace. A person with no tenant gets a token with none.

#### Scenario: Refresh keeps the workspace
- **WHEN** a person with one workspace signs in by code and later refreshes
- **THEN** both the first and the refreshed access token carry that workspace's `tenant_id`

### Requirement: Switch tenant re-issues JWT
The system SHALL provide a switch-tenant endpoint that verifies membership and issues a new JWT scoped to the target tenant.

#### Scenario: Valid switch
- **WHEN** authenticated user requests switch to tenant they belong to
- **THEN** the system returns a new JWT with the target `tenant_id`

#### Scenario: User not member of target tenant
- **WHEN** user requests switch to tenant they do NOT belong to
- **THEN** the system returns "access denied" error

#### Scenario: Suspended or unaccepted membership
- **WHEN** the user's membership in the target tenant is `disabled` or `invited`
- **THEN** the system returns "access denied" and issues no token, and `GET /api/me` does not present that tenant as the current one

### Requirement: JWT carries tenant context
The system SHALL include `tenant_id` and `session_id` in all JWTs. All downstream services receive tenant context automatically.

#### Scenario: JWT structure
- **WHEN** a JWT is issued (OTP verify, Google callback, switch-tenant)
- **THEN** the token payload contains `user_id`, `username`, `ngac_node_id`, `tenant_id`, and `session_id`

#### Scenario: Backward compatibility
- **WHEN** a service receives a JWT without `tenant_id` (legacy token)
- **THEN** the service SHALL accept it and treat `tenant_id` as empty string

### Requirement: Account state tells the client what is still owed
`GET /api/me` and the OTP verification response SHALL report `email_verified` (the address has been proved, see "Email addresses are normalised, unique and proved") and `needs_profile` (the person has not yet saved the name colleagues will see). `users.profile_completed_at` (migration 032) is NULL for an account created by any route (code, Google, signup) and is set the first time `PATCH /api/me/profile` saves something; accounts that existed when the column was added are marked done. The sign-in flow asks for the profile exactly when `needs_profile` is true, whichever route the account came in by.

#### Scenario: New account owes a profile
- **WHEN** an account has just been created by a code or by Google
- **THEN** `needs_profile` is true and `email_verified` follows the proof it was created with

#### Scenario: Saving the profile ends the step
- **WHEN** the person saves a display name
- **THEN** `needs_profile` becomes false and stays false

### Requirement: Profile updates are partial, validated and the caller's own
`PATCH /api/me/profile` SHALL change only the fields present in the body among `display_name`, `title` and `location`: an absent or null field is left as it is and an empty string clears it. The person edited is the token's user; nothing in the body can name another. Text is trimmed and refused (400, nothing written, including the valid fields of the same request) when: the display name is empty or over 80 characters, another field is over 120, or any contains a control character or an invisible formatting character (zero-width, bidirectional override). A body with no field is 400 and does not end the profile step. Limits count characters a person sees, not bytes.

Two fields are refused by name, 400, writing nothing: `department` (`department_not_editable`: a department is assigned by a workspace administrator, per workspace, and a person who could write their own could present themselves under any department in the directory) and `avatar_url` (`avatar_not_supported`: there is no image store a profile could point at, and a URL a person typed would be loaded by every colleague's browser).

#### Scenario: Refused input writes nothing
- **WHEN** a request carries a valid display name and a department
- **THEN** the answer is 400 `department_not_editable`, the display name is unchanged and `needs_profile` is unchanged

#### Scenario: An avatar
- **WHEN** a request carries `avatar_url`, even an empty one
- **THEN** the answer is 400 `avatar_not_supported`

#### Scenario: Naming another person
- **WHEN** the body carries `user_id` or `id` of another account
- **THEN** it is ignored: only the caller's profile changes

### Requirement: GET /me is the person's own record
`GET /api/me` SHALL return the signed-in user (`id`, `username`, `ngac_node_id`, `email`, `display_name`, `title`, `location`, `avatar_url`, `email_verified`, `needs_profile`) and `current_tenant` (`id`, `name`, `role`, `open_id`, `department`) for the token's workspace, or for `?workspace=<id>` when given. `current_tenant` is present only for a workspace the caller is an ACTIVE member of, and `department` is the one an administrator assigned there (empty when none), never the person's own claim. Settings reads the person from here instead of searching the directory.

#### Scenario: Someone else's workspace
- **WHEN** a caller asks `?workspace=` for a workspace they do not belong to, or where their membership is suspended
- **THEN** `current_tenant` is absent and nothing about that workspace is revealed

### Requirement: A person lists and creates their own workspaces
`GET /api/me/workspaces` SHALL list the workspaces the caller is an active member of, each with `id`, `name`, the caller's own `role`, the `member_count` of active members and the company `domain` the workspace claimed (empty for a personal one). It is exactly the set `POST /api/auth/switch-tenant` accepts. `POST /api/me/workspaces` SHALL create a workspace from `{"name"}` alone, with the caller as owner, provisioned as signup does (graph, owner and member attributes, `tenant_users` row, `#general`), and answer 201 with the same shape. Nothing else in the body is read: no slug, domain or owner. The name is trimmed and 1 to 80 characters with no control or invisible formatting characters (400 otherwise, nothing created). Each person may create 5 workspaces per hour (429 `rate_limited` with `Retry-After`), counted before anything is created and only for requests whose name is valid. The caller's address MUST be verified: an account whose address is unproved (or has none) is refused 403 `email_unverified`, before anything is created and without spending the budget. Unlike sign-in provisioning, any failure after the workspace exists (the tenant graph, the membership row, the owner's attachment, `#general`) removes the workspace through the workspace service's `DeleteWorkspace` (graph, rows, drive, channels, approval schema) and answers 500; the removal runs on a context detached from the request. The older `POST /api/workspaces` does not provision a `tenant_users` row and is not what the screens call.

#### Scenario: Creating from a name
- **WHEN** a signed-in person posts `{"name":" Tổ Đối soát "}`
- **THEN** the answer is 201 with the trimmed name, role `owner` and member count 1, and the person is listed as owner in `tenant_users`

#### Scenario: Unverified address
- **WHEN** an account whose address nobody has proved posts a valid name
- **THEN** the answer is 403 `email_unverified` and no workspace exists

#### Scenario: Provisioning fails half way
- **WHEN** the tenant graph cannot be built after the workspace was created
- **THEN** the workspace is removed again and the answer is 500; nothing of it remains

#### Scenario: Not creating for someone else
- **WHEN** the body also carries `owner_id` of another account
- **THEN** it is ignored and the caller is the owner

#### Scenario: Too many
- **WHEN** a person creates a sixth workspace within an hour
- **THEN** the answer is 429 with the wait, and nothing is created

### Requirement: Error answers carry a code and never internals
Every error from the auth service's REST routes SHALL be a JSON object `{"message", "code"}` (plus `attempts_left`, `retry_after_seconds` where they apply), whichever handler or middleware raised it: the session check (`session_required`, `session_invalid`, `tenant_required` from `pkg/httputil`), refresh and logout (`session_required`, `session_invalid`, `internal`), Google (`google_unavailable`), switch-tenant, the directory, an unknown route (`not_found`) or a failure Echo raises with only a status. `code` is stable and machine-readable (`invalid_input`, `not_found`, `access_denied`, `already_exists`, `already_verified`, `email_unverified`, `email_mismatch`, `otp_invalid`, `otp_expired`, `otp_too_many_attempts`, `otp_rate_limited`, `rate_limited`, `otp_unavailable`, `verification_unavailable`, `department_not_editable`, `avatar_not_supported`, `unavailable`, `internal`); `message` is an English fallback that no screen shows. An unexpected failure is `500 {"code":"internal","message":"internal error"}`: its own text (hosts, queries, downstream errors) goes to the log only. A sweep over the whole route table asserts it.

#### Scenario: Internal failure
- **WHEN** a downstream call fails with text naming a host
- **THEN** the caller sees `internal error`, not the host

#### Scenario: Missing session
- **WHEN** a protected route is called without a session
- **THEN** the answer is 401 `session_required`, and with a bad one 401 `session_invalid`

### Requirement: Public routes are limited per address
`POST /api/auth/otp/request`, `POST /api/auth/otp/verify`, `POST /api/auth/refresh`, `POST /api/auth/logout` and `GET /api/auth/google/start` SHALL each allow one address a number of requests per minute (default 60, `AUTH_PUBLIC_RATE_LIMIT`, 0 turns it off), counted in Redis per address and per route and answered 429 `rate_limited` with `Retry-After` and `retry_after_seconds`. A counter that cannot be reached never blocks a person. The address is the connection's own: `X-Forwarded-For` is read only from networks listed in `AUTH_TRUSTED_PROXIES` (CIDR, default empty: none; Docker lists the bridge networks Traefik sits on), right to left, skipping only entries those networks added, so a header a client wrote cannot choose the counter. An unparseable range stops start-up.

#### Scenario: Spoofed X-Forwarded-For
- **WHEN** a client with nothing trusted sends each request with a different `X-Forwarded-For`
- **THEN** all of them count against the connection's own address and the limit trips

#### Scenario: Behind a trusted proxy
- **WHEN** requests arrive through a trusted proxy for two clients
- **THEN** each client has its own allowance, and a forged leading entry does not give one a new one

### Requirement: The directory is members-only, paged, and names no handle
`GET /api/workspaces/:id/contacts` SHALL require the caller to be an ACTIVE member of that workspace (403 otherwise, including a `disabled` or `invited` one; nothing listed). It answers one page of at most `limit` people (default 50, at most 200; 400 otherwise) in a stable order, `{contacts, total, next_cursor?}`: `total` is the true number of people the filters match, not the page's length, and `next_cursor` (opaque, keyset: the sort key and ID of the last person) is present only when more follow and is passed back as `cursor`. A malformed cursor, or a search longer than 100 characters, is 400. Filters: `department` (the administrator-assigned department's name, exact), `location`, `search` (case-insensitive substring of name, title, department or email; `%` and `_` are literal). A person's `department` is the one assigned in THIS workspace (`tenant_users.department_id` joined to `departments`), never what they wrote about themselves. `display_name` is empty, not the login handle, for a person who has not yet saved their profile. The routes that listed every account (`GET /api/users`) or looked one up by username do not exist.

#### Scenario: Outsider or suspended member reads a directory
- **WHEN** a signed-in person who is not an active member requests a workspace's contacts
- **THEN** the answer is 403 `access_denied` and no contact is returned

#### Scenario: A person joins between two pages
- **WHEN** someone who sorts before the cursor joins after the first page was read
- **THEN** the second page repeats nobody and the total reflects the new person

### Requirement: Proving the address of the signed-in account
`POST /api/me/email/verify` SHALL prove the address on the caller's account without signing anyone in: with no `code` it sends a code to the address (202 `{expires_in}`), with a `code` it checks it (200 `{email_verified:true}`). Neither returns a token or sets a cookie. A code is offered only when a code delivered here reaches only the owner of the address (a real sender; 503 `verification_unavailable` for the fixed test code, the log sender, or no sender); an already-proved address is 409 `already_verified`; requests share the sign-in allowance for that address (5 per 15 minutes, 429 with `Retry-After`). A code is stored as an HMAC, keyed by the account that asked, works once, for that account only, while the address is still the one it was sent to, with at most 5 wrong tries (401 `otp_invalid` with `attempts_left`, then 429 `otp_too_many_attempts`). The first proof ends every other session on the account and clears a credential set before it, but spares the session that proved it.

`POST /api/me/email/verify/google` SHALL start the same proof with Google: it stores a flow naming the account and its session, binds the browser with the state cookie, and answers `{url}` (the Google URL, hinted to the account's address). The callback proves the address only if the Google account's own VERIFIED email equals the account's; any other result is refused, and in every case it issues no session, sets no cookie, links no identity and creates nothing, returning to `/workspace-select?verified=1` or `?verify_error=<google_mismatch|google_unverified|google_cancelled|google_failed|google_error>`.

#### Scenario: Another Google account
- **WHEN** the person signed in as `hoa.le@novapay.vn` picks a Google account whose verified email is another address
- **THEN** nothing is verified, nobody is signed in as the other account, and the browser returns with `verify_error=google_mismatch`

#### Scenario: A stolen code
- **WHEN** a code issued to one account is presented by another
- **THEN** it is refused and neither account is verified

### Requirement: GET /me returns current user with tenant context
The system SHALL provide a `/me` endpoint that returns the authenticated user's info and their current tenant membership.

#### Scenario: Authenticated user
- **WHEN** user calls GET /api/me with valid JWT
- **THEN** the response includes user info (id, email, display_name) and current tenant info (id, name, role, open_id)
