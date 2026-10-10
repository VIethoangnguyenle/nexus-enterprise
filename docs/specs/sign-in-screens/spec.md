# sign-in-screens

## Purpose
Take a person from "not signed in" to "inside a workspace" in the fewest honest steps: prove who they are, say what to call them if they are new, answer any invitation waiting for them, and choose or create a workspace. Design source: `design/mockups/auth.html`.

## Requirements

### Requirement: Sign-in offers only the ways the server has
The sign-in page SHALL offer Google when `GET /api/auth/providers` says `google: true`, and a six-digit code to an email or a Vietnamese phone number unless it says `otp: false`. A failed providers request hides Google and keeps the code form. When neither is offered the page says sign-in is paused and offers "Thử lại". There is no single-sign-on button, no "Join an Organization" and no plan label. No screen, in any build, prints the fixed test code or a "test mode" line.

#### Scenario: Codes switched off
- **WHEN** the server reports `otp: false` and `google: true`
- **THEN** only the Google button is shown

#### Scenario: The fixed test code is in force
- **WHEN** the server reports `otp_fixed_code: true`
- **THEN** no screen mentions a code or a test mode

### Requirement: Failures are shown beside what caused them, in words that say what to do
Sign-in, profile, workspace and invitation requests are made with the shared error toast off; each failure is shown inline in Vietnamese, chosen by status and machine code and never echoing the server's text or a code. A malformed email or number is explained under the field on blur with an example. A refused code request with 429 says how long to wait; 503 says sign-in is paused; no connection says to check the network.

#### Scenario: Rate limited
- **WHEN** a code request is refused with 429 and `retry_after_seconds` 700
- **THEN** the page says to try again in 12 minutes and stays on the first step

### Requirement: The code field takes a code the way people receive one
The code is entered in one real input (numeric, `autocomplete="one-time-code"`) drawn as six boxes, so a paste or an SMS autofill fills all six. Pasting keeps only the digits, from the first six; arrow keys, Home and End move the highlighted box; the sixth digit submits once. States: idle, checking (digits kept, typing ignored), wrong, expired, locked and success. A wrong code is shown with the error colour and a fade (no transform), clears the boxes, keeps focus, and says how many tries remain; after the last try, or when the code expired, the field stops taking digits and "Gửi lại" is available at once. "Gửi lại" otherwise waits 60 seconds, counted in tabular figures, and a refused resend keeps it locked for the time the server named.

#### Scenario: Pasting a message
- **WHEN** a person pastes "Mã của bạn là 481 209"
- **THEN** the boxes read 481209 and the code is submitted once

#### Scenario: Wrong code
- **WHEN** the server answers `otp_invalid` with 4 tries left
- **THEN** the boxes empty and take the error colour, and the page says "Mã chưa đúng. Còn 4 lần thử."

### Requirement: A new person is asked one thing
After a session exists the flow goes to the profile step if `needs_profile` is true (whichever route the account came in by, code or Google) and to workspace selection otherwise. The profile step asks for the display name (1 to 80 characters), optionally a title, and previews the avatar as the initials of the typed name on the person's own colour. It offers no photo upload: the server keeps no image store a profile could point at. It sends only the fields filled in. A person who has no profile owed is moved on.

#### Scenario: Google person with a name
- **WHEN** an account created by Google already has the name Google gave
- **THEN** the field starts with that name

### Requirement: Workspace selection lists workspaces and answers invitations
The screen lists the person's workspaces from `GET /api/me/workspaces` (the very set `switch-tenant` accepts) with their own role, the headcount and, when the workspace claimed one, its company domain, above them the invitations waiting for the person (who invited, to what, in which role, when it lapses) with "Tham gia" and "Từ chối". Accepting joins and opens that workspace; declining removes the offer and says so. A refused answer (404 gone, 409 already answered, 400 expired, 403 the inviter lost the right) is explained and the list catches up. Opening a workspace first re-scopes the session with `POST /api/auth/switch-tenant` unless the token already speaks for it, then opens it with `?ws=`; a link naming a workspace the person belongs to opens it. Exactly one workspace and nothing to answer or explain goes straight in on first load; the screen never decides again after that. Loading, empty ("Bạn chưa thuộc workspace nào"), and error states exist for the list; a failed invitations request is shown inline while the workspaces stay usable.

#### Scenario: Accepting
- **WHEN** a person accepts an invitation
- **THEN** `POST /api/invitations/:id/accept` is sent with no body, the session is re-scoped to the new workspace and the app opens in it; if the role or department did not apply, a toast says so

### Requirement: An unverified address is explained, not faked
The invitations a person sees depend on `email_verified`. A person whose address is unverified sees no invitations, and a calm line says why and that they appear after the address is proved. The screen offers only ways that really prove it, and neither signs anyone in: "Xác minh bằng Google" calls `POST /api/me/email/verify/google` and leaves for the returned URL (http(s) only), coming back to `/workspace-select?verified=1` (a toast, and the lists refresh) or `?verify_error=<code>` (a calm message naming the address that must be used, e.g. when the Google account differs); and a code sent to the address (`POST /api/me/email/verify`, then the code in the same field as sign-in) when `otp_proves_email` is true. When neither is available it says so. A phone-only account is told invitations go to an email. An unverified account with one workspace is not taken straight in, so the explanation is seen.

#### Scenario: Fixed test code only
- **WHEN** the address is unverified and the only code available is the fixed test code
- **THEN** no "send a code" button is shown, because that code proves nothing

### Requirement: A workspace is created from its name
The create screen has one field, the name (1 to 80 characters), and no URL, slug, logo or invitation field; colleagues are invited later from Quản trị. It calls `POST /api/me/workspaces {name}`, then opens the new workspace. A person whose address is unverified (or has none) does not see the form: the screen shows the same calm verify path as workspace selection, because the server refuses with 403 `email_unverified`; a phone-only account is told a workspace needs an email address. A workspace created but not opened says so and returns to the list. The step bar appears only for a person who has no workspace yet.

#### Scenario: Name only
- **WHEN** a person submits "Tổ Đối soát"
- **THEN** the request body is exactly `{"name":"Tổ Đối soát"}`

### Requirement: Routes and sessions
Moving from the profile step to workspace selection or creation keeps the `ws` the person arrived with. The sign-in page turns away a signed-in person to workspace selection; the profile, workspace-selection and create-workspace pages turn away a signed-out person to the sign-in page. Both checks run when navigation starts, not whenever the session changes. "Đổi tài khoản" ends the session through the one logout path (`logoutSession`: the server revokes the refresh token, then the local session and every cached query are dropped). There is no welcome page and no timed redirect.

#### Scenario: Signed-out visit
- **WHEN** a signed-out person opens the workspace selection URL
- **THEN** they are sent to the sign-in page

### Requirement: No identifier reaches the screen
No UUID, node id, session id or machine code is rendered, labelled or typed on any sign-in screen; people are shown by display name with an avatar from their initials, workspaces by name.
