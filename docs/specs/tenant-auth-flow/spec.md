# tenant-auth-flow

## Purpose
Get a user from credentials to a tenant-scoped session, and let them move between tenants, without ever issuing a token that is ambiguous about which tenant it speaks for.

## Requirements

### Requirement: Domain auto-join requires proof of email ownership
The system SHALL add a user to a tenant on the basis of that tenant's `domain` only when it holds proof that the user controls an address at that domain. Today the only accepted proof is a Google Workspace sign-in whose verified ID token carries `hd` equal to the domain (see "Google sign-in resolves the company from the hosted domain only"). Password signup (`/api/auth/signup`, gRPC `Signup`), legacy register, and OTP sign-in SHALL NEVER join a tenant by email domain and SHALL NEVER claim a domain: none of them verifies the email. (OTP is excluded: in its default fixed-code test mode the code is the same for everyone and proves nothing, and random-code mode has no real delivery channel yet. It MAY qualify once random codes are delivered to the email by a real sender and that mode is the only one enabled.) Otherwise a user joins a company tenant only through an invitation.

#### Scenario: Password signup at a claimed domain
- **WHEN** user signs up with a password as `x@acme.com` and a tenant has `domain = 'acme.com'`
- **THEN** the user is NOT added to that tenant
- **AND** a personal workspace is created with the user as `owner` and a #general channel is auto-provisioned
- **AND** the personal workspace does not claim `acme.com`

#### Scenario: Password signup at an unclaimed domain
- **WHEN** user signs up with a password as `bob@newcorp.com` and no tenant has `domain = 'newcorp.com'`
- **THEN** a personal workspace is created with the user as `owner`
- **AND** no tenant claims `newcorp.com`

#### Scenario: Signup with explicit tenant name
- **WHEN** user provides a `tenant_name` in the signup request
- **THEN** the system creates a new tenant with that name, with the user as `owner`, and no domain

#### Scenario: Legacy register or OTP at a claimed domain
- **WHEN** a new user registers through legacy register, or is created by OTP verification of `x@acme.com`, and a tenant has `domain = 'acme.com'`
- **THEN** the user is NOT added to that tenant

#### Scenario: Google Workspace sign-in at a claimed domain
- **WHEN** a user signs in with Google and the verified ID token has `hd = 'acme.com'`, and a tenant has `domain = 'acme.com'`
- **THEN** the user is added to that tenant as a `member`

### Requirement: Public email domains never place a user in a tenant
The system SHALL keep one list of public mailbox domains (gmail.com, googlemail.com, outlook.com, hotmail.com, live.com, yahoo.com, icloud.com, proton.me, protonmail.com, gmx.com, yandex.com, mail.ru, aol.com, zoho.com and similar) in `backend/services/auth/internal/domain/public_domains.go`. A domain on that list SHALL never auto-join a user to a tenant and SHALL never be claimed as a tenant's `domain`, on any sign-in path. Domains are compared case-insensitively, and `workspaces.domain` is unique case-insensitively.

#### Scenario: Public domain on any path
- **WHEN** a user signs up as `victim@gmail.com`, or signs in with Google with `hd = 'gmail.com'`, and a tenant has `domain = 'gmail.com'`
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
- **THEN** the system issues the same session as password and OTP sign-in — an access token scoped to the default tenant and a rotating refresh token in the httpOnly `SameSite=Strict` cookie bound to the same session
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
Nothing else in the system verifies email ownership, so an account found by email may have been created by someone other than the address's owner (pre-hijacking: an attacker signs up with the victim's address and a password, then waits). When Google sign-in links a verified identity to such an existing account, the system SHALL treat Google as the proof of ownership and, before linking, SHALL clear the account's password and revoke all of its existing sessions on every device, then write an audit log line naming the user id only (never the email). If either step fails, the sign-in SHALL fail rather than link. Session revocation records a per-user cutoff in Redis for the refresh-token lifetime; `POST /api/auth/refresh` rejects any session that started before the cutoff, including sessions created before session start times were recorded. Access tokens already issued remain valid until they expire (at most 15 minutes).

#### Scenario: Pre-registered password stops working
- **WHEN** an account was created with a password for `victim@acme.com` and the owner then signs in with Google as `victim@acme.com` for the first time
- **THEN** signing in with that password fails with "invalid credentials"

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
- **THEN** a new tenant named after the domain is created with the user as `owner`, provisioned like signup (tenant NGAC UAs, `#general` channel), and claims `domain = 'newco.io'` with type `organization`
- **AND** later users with the same `hd` join it as `member`

#### Scenario: Consumer Google account
- **WHEN** a Google user without `hd` (e.g. `someone@gmail.com`) signs in
- **THEN** the user is never auto-joined to any tenant by domain, even if a tenant claims `gmail.com`
- **AND** a new user gets a personal workspace as `owner`, as an OTP or signup user with no company does; an existing user keeps the tenants they already belong to

#### Scenario: Company email without a hosted domain
- **WHEN** a Google account registered on `bob@acme.com` without Google Workspace (no `hd`) signs in and a tenant has `domain = 'acme.com'`
- **THEN** the user is NOT added to that tenant

### Requirement: OTP sign-in codes
`POST /api/auth/otp/request` opens a 5-minute session for an email or phone and `POST /api/auth/otp/verify` exchanges its code for the same session as other sign-ins. The code's mode is set by `AUTH_FIXED_OTP_CODE`:
- **Fixed-code test mode (default).** Unset → `999999`; any six digits → that code. Every OTP session accepts that code, so anyone who knows it can sign in as any email or phone. This is a documented TEST-ONLY mode for testers on deployed builds, and the auth service logs a warning at startup while it is on. `GET /api/auth/providers` reports `otp: true, otp_fixed_code: true`, and the login page then shows the "OTP code is 999999" hint.
- **Random-code mode.** Setting `AUTH_FIXED_OTP_CODE` explicitly empty (`AUTH_FIXED_OTP_CODE=`) turns the test mode off. Each request then gets a uniformly random six-digit code from `crypto/rand` (rejection sampling, no modulo bias), stored only as an HMAC-SHA256 keyed by a secret derived from the JWT secret and bound to the session, compared in constant time, and handed to a `CodeSender` for delivery. If delivery fails, the request fails and the session is removed. The only sender today is a log sender that runs only when `APP_ENV=dev` or `AUTH_DEV_OTP=1` and refuses otherwise. With no sender, OTP is disabled: request and verify return 503, `/api/auth/providers` reports `otp: false`, and the login page hides the OTP form.

In both modes: at most 5 verification attempts per session (counted atomically, the 6th fails even with the right code), a code works once, at most 5 code requests per identifier per 15 minutes (429), and the service never logs the code.

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
- **THEN** the request is rejected with 429 and other identifiers are unaffected

### Requirement: Signin returns tenant list
The system SHALL authenticate the user and return a list of all tenants the user belongs to, plus a JWT scoped to the default tenant.

#### Scenario: User belongs to one tenant
- **WHEN** user signs in with valid credentials and belongs to one tenant
- **THEN** the response includes a JWT with `tenant_id` set to that tenant and a `tenants` list of length 1

#### Scenario: User belongs to multiple tenants
- **WHEN** user signs in and belongs to 3 tenants
- **THEN** the response includes a JWT scoped to the default tenant (owner tenant or most recent) and a `tenants` list of length 3

#### Scenario: Invalid credentials
- **WHEN** user provides wrong email or password
- **THEN** the system returns "invalid credentials" error

### Requirement: Switch tenant re-issues JWT
The system SHALL provide a switch-tenant endpoint that verifies membership and issues a new JWT scoped to the target tenant.

#### Scenario: Valid switch
- **WHEN** authenticated user requests switch to tenant they belong to
- **THEN** the system returns a new JWT with the target `tenant_id`

#### Scenario: User not member of target tenant
- **WHEN** user requests switch to tenant they do NOT belong to
- **THEN** the system returns "access denied" error

### Requirement: JWT carries tenant context
The system SHALL include `tenant_id` and `session_id` in all JWTs. All downstream services receive tenant context automatically.

#### Scenario: JWT structure
- **WHEN** a JWT is issued (signup, signin, switch-tenant)
- **THEN** the token payload contains `user_id`, `username`, `ngac_node_id`, `tenant_id`, and `session_id`

#### Scenario: Backward compatibility
- **WHEN** a service receives a JWT without `tenant_id` (legacy token)
- **THEN** the service SHALL accept it and treat `tenant_id` as empty string

### Requirement: GET /me returns current user with tenant context
The system SHALL provide a `/me` endpoint that returns the authenticated user's info and their current tenant membership.

#### Scenario: Authenticated user
- **WHEN** user calls GET /api/me with valid JWT
- **THEN** the response includes user info (id, email, display_name) and current tenant info (id, name, role, open_id)
