// Package domain contains the business logic for the auth service.
// It orchestrates user registration, login, tenant management, and NGAC graph setup.
// No SQL or gRPC/HTTP parsing lives here — only domain rules.
package domain

import "ngac-platform/pkg/httputil"

import (
	"errors"
	"time"
)

// These alias the shared sentinels rather than declaring new ones. errors.Is
// compares identity, not message text, so a local errors.New with the same
// string would be a different value and httputil.MapDomainError would never
// match it — every failure would surface as 500, including denials.
var (
	ErrNotFound           = httputil.ErrNotFound
	ErrAccessDenied       = httputil.ErrAccessDenied
	ErrInvalidInput       = httputil.ErrInvalidInput
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrOTPExpired         = errors.New("otp expired or not found")
	ErrOTPInvalid         = errors.New("invalid otp code")
	ErrTenantNotFound     = errors.New("tenant not found")
	ErrTooManyAttempts    = errors.New("too many attempts")
	// ErrOTPUnavailable means OTP sign-in is switched off: no fixed test
	// code and no way to deliver a random one.
	ErrOTPUnavailable = errors.New("otp sign-in is not available")
	// ErrOTPRateLimited means too many codes were requested for one identifier.
	ErrOTPRateLimited = errors.New("too many codes requested, try again later")
	ErrUserExists     = errors.New("already exists")
	// ErrEmailNotVerified rejects an external sign-in whose provider has not
	// verified the email. Such an address is only a claim, so it may neither
	// create an account nor be matched against an existing one.
	ErrEmailNotVerified = errors.New("email not verified by identity provider")
	// ErrIdentityConflict means the email belongs to an account that is
	// already linked to a different subject at the same provider — typically a
	// deleted Workspace account whose address was reassigned to someone else.
	ErrIdentityConflict = errors.New("account is linked to a different external identity")
)

// ErrRateLimited means a person has used up an allowance (for instance the
// number of workspaces they may create per hour). RetryAfter says when to come
// back. OTP code requests use the more specific ErrOTPRateLimited.
var ErrRateLimited = errors.New("too many requests, try again later")

// rateLimitedError wraps one of the rate-limit sentinels with how long the
// caller has to wait. errors.Is still matches the sentinel.
type rateLimitedError struct {
	base  error
	after time.Duration
}

func (e *rateLimitedError) Error() string { return e.base.Error() }
func (e *rateLimitedError) Unwrap() error { return e.base }

// RetryAfter reports how long a rate-limited caller has to wait, when known.
func RetryAfter(err error) (time.Duration, bool) {
	var r *rateLimitedError
	if errors.As(err, &r) && r.after > 0 {
		return r.after, true
	}
	return 0, false
}

// otpInvalidError is ErrOTPInvalid plus how many tries the session has left, so
// the sign-in screen can say so. The count belongs to the session, not to an
// account, so it tells a caller nothing about who exists.
type otpInvalidError struct{ left int }

func (e *otpInvalidError) Error() string { return ErrOTPInvalid.Error() }
func (e *otpInvalidError) Unwrap() error { return ErrOTPInvalid }

// OTPAttemptsLeft reports the tries remaining after a wrong code, when the
// error is one.
func OTPAttemptsLeft(err error) (int, bool) {
	var o *otpInvalidError
	if errors.As(err, &o) {
		return o.left, true
	}
	return 0, false
}

// ErrEmailUnverified means the action needs an account whose address has been
// proved (by Google, or by a code a real sender delivered to it), and this
// account's has not. It is a state the person can fix, so it has its own answer.
var ErrEmailUnverified = errors.New("email address not verified")

// ErrVerificationUnavailable means no way of proving an address by code exists
// right now: the fixed test code and the log sender prove nothing, and with no
// sender no code can be sent at all. The person can use Google instead.
var ErrVerificationUnavailable = errors.New("email verification by code is not available")

// ErrAlreadyVerified means the account's address is already proved.
var ErrAlreadyVerified = errors.New("email address already verified")

// ErrEmailMismatch means the address a provider vouched for is not the one on
// the account being verified.
var ErrEmailMismatch = errors.New("the verified address is not this account's")
