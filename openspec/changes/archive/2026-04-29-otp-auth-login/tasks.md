## Tasks

### Phase 1: Database + Store
- [x] Add `phone` column to users table (migration in `data/migrations/006_otp_auth.sql`)
- [x] Add `GetUserByPhone()` to auth store
- [x] Update `CreateUser()` signature to accept phone parameter
- [x] Run migration on local DB

### Phase 2: Domain — OTP Logic
- [x] Create `domain/otp.go` with phone/email validation helpers (`normalizePhone`, `isValidPhone`, `isValidEmail`)
- [x] Implement `RequestOTP()` — validate identifier, store OTP session in Redis, return session_id
- [x] Implement `VerifyOTP()` — check code, auto-register new users, return JWT
- [x] Add new sentinel errors (`ErrOTPExpired`, `ErrOTPInvalid`, `ErrTooManyAttempts`)

### Phase 3: REST Handler
- [x] Add `POST /api/auth/otp/request` handler
- [x] Add `POST /api/auth/otp/verify` handler
- [x] Update `mapError()` for new OTP errors
- [x] Vite proxy already covers `/api/auth/*` — no change needed
- [x] Build + verify auth service compiles

### Phase 4: Frontend — Login UI
- [x] Create `OtpInput.tsx` — 4-digit input with auto-advance and auto-submit
- [x] Rewrite `login.tsx` — Lark-style split layout with tabs (Email | Phone)
- [x] Implement Step 1 (identity input with validation)
- [x] Implement Step 2 (OTP verification with resend countdown)
- [x] Add `requestOTP()` and `verifyOTP()` to `api/auth.ts`
- [x] Update `useAuth.ts` hook for OTP flow
- [x] Update `register.tsx` to redirect to unified login

### Phase 5: Verify
- [x] Test full flow: phone OTP → auto-register → workspace created → chat accessible
- [x] Test full flow: email OTP → auto-register → workspace created
- [x] Test returning user: phone OTP → login → existing workspace
- [x] Test validation: invalid phone → error message
- [x] Test validation: invalid email → error message
- [x] Test OTP expiry: session expired error
- [x] UI smoke test via browser
