# Design: OTP Auth Login

## Auth Flow

### Step 1: Request OTP
```
POST /api/auth/otp/request
{
  "identifier": "0912345678",   // or "user@example.com"
  "type": "phone"               // or "email"
}

→ 200 { "session_id": "uuid", "expires_in": 300 }
→ 400 { "error": "invalid phone number" }
```

Backend logic:
1. Validate identifier format (phone regex or email regex)
2. Generate `session_id` (UUID)
3. Store in Redis: `otp:{session_id}` → `{identifier, type, code:"9999", attempts:0}` TTL 5min
4. Log OTP to console (dev mode): `slog.Info("OTP generated", "code", "9999")`
5. Return session_id

### Step 2: Verify OTP
```
POST /api/auth/otp/verify
{
  "session_id": "uuid",
  "code": "9999"
}

→ 200 { "token": "jwt...", "user": {...}, "is_new_user": true }
→ 401 { "error": "invalid or expired code" }
→ 429 { "error": "too many attempts" }  (max 5)
```

Backend logic:
1. Lookup Redis `otp:{session_id}`
2. If not found → 401 expired
3. If `attempts >= 5` → 429, delete key
4. If `code != input` → increment attempts, return 401
5. If match → delete Redis key, then:
   - Lookup user by phone/email
   - If not found → **auto-register** (create user + NGAC node + workspace + #general)
   - Generate JWT token
   - Return token + user + `is_new_user` flag

## Database Migration

```sql
-- Add phone column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone VARCHAR(20);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone ON users (phone) WHERE phone IS NOT NULL AND phone != '';
```

No existing data affected — current users have `phone = NULL`.

## Store Layer Changes

```go
// New method
func (s *Store) GetUserByPhone(ctx context.Context, phone string) (*User, error)

// Updated CreateUser — add phone parameter
func (s *Store) CreateUser(ctx context.Context, id, username, password, ngacNodeID, email, unionID, displayName, phone string) error
```

## Domain Layer Changes

New file: `auth/internal/domain/otp.go`

```go
// RequestOTP validates identifier, creates Redis session, returns session_id.
func (s *Service) RequestOTP(ctx context.Context, identifier, identType string) (sessionID string, err error)

// VerifyOTP checks code, auto-registers if needed, returns JWT.
func (s *Service) VerifyOTP(ctx context.Context, sessionID, code string) (*AuthResponse, bool, error)
```

OTP Redis structure:
```
Key:   "otp:{session_id}"
Value: JSON {identifier, type, code, attempts}
TTL:   300 seconds
```

## Phone Number Validation

```go
// normalizePhone normalizes Vietnamese phone numbers.
// Accepts: 0912345678, +84912345678, 84912345678
// Returns: 0912345678 (stored format)
func normalizePhone(phone string) (string, error)

// isValidPhone checks length (10-11 digits) and prefix (03,05,07,08,09).
func isValidPhone(phone string) bool
```

## REST Handler Changes

```go
// New routes (public, no JWT)
e.POST("/api/auth/otp/request", h.RequestOTP)
e.POST("/api/auth/otp/verify", h.VerifyOTP)
```

## Frontend UI Design

### Layout: Split screen (Lark-style)

```
┌─────────────────────────────────┬──────────────────────────────┐
│                                 │                              │
│     ┌─────────┐                │   Welcome to NGAC            │
│     │  ┌───┐  │                │                              │
│     │  │ N │  │                │   [Email Address] [Phone #]  │
│     │  └───┘  │                │   ─────────────────────────  │
│     │ ┌─┐┌─┐  │                │                              │
│     │ │ ││ │  │                │   ┌─────────────────────┐    │
│     └─┴─┘└─┘──┘                │   │ Enter phone number  │    │
│                                 │   └─────────────────────┘    │
│   Your enterprise platform     │                              │
│   Manage information,          │   [        Next         ]    │
│   workflows, and people.       │                              │
│                                 │   □ I accept Terms of       │
│                                 │     Service                  │
│                                 │                              │
└─────────────────────────────────┴──────────────────────────────┘
```

### Step 1: Identity input
- Two tabs: "Email Address" | "Phone Number"
- Phone tab: country code selector (+84) + input field
- Email tab: standard email input
- "Next" button (disabled until valid input)
- Terms checkbox (cosmetic)

### Step 2: OTP verification
- Header: "Enter verification code"
- Subtitle: "We sent a code to 091****678" (masked)
- 4 separate digit boxes, monospace font, auto-focus next on type
- Auto-submit when 4th digit entered
- "Resend code" link with 60s countdown timer
- "Back" link to return to Step 1

### Frontend file structure
```
routes/_auth/
├── login.tsx          # REWRITE — 2-step OTP login
├── register.tsx       # Keep but add redirect to login (unified flow)

api/
├── auth.ts            # Add requestOTP(), verifyOTP()

components/auth/
├── OtpInput.tsx       # 4-digit OTP input component
├── LoginIllustration.tsx  # Left-side branding illustration
```

## Auto-Register Flow (new users)

When OTP verifies for a phone/email not in DB:

1. Generate username from identifier:
   - Phone `0912345678` → `user_0912345678`
   - Email `alice@company.com` → `alice.company` (existing `emailToUsername`)
2. Create user with `password = ""` (no password for OTP users)
3. Create NGAC user node
4. Auto-provision workspace + #general channel (existing `autoProvisionWorkspace`)
5. Generate JWT, return with `is_new_user: true`

## Security Considerations

- OTP sessions expire after 5 minutes (Redis TTL)
- Max 5 attempts per session, then session deleted
- Session deleted on successful verify (one-time use)
- Phone numbers stored normalized (consistent lookup)
- Hardcoded OTP `9999` only in dev — flag for future real implementation
