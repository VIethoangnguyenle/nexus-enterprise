// Package domain contains the business logic for the auth service.
// It orchestrates user registration, login, tenant management, and NGAC graph setup.
// No SQL or gRPC/HTTP parsing lives here — only domain rules.
package domain

import "ngac-platform/pkg/httputil"

import "errors"

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
	ErrUserExists         = errors.New("already exists")
	// ErrEmailNotVerified rejects an external sign-in whose provider has not
	// verified the email. Such an address is only a claim, so it may neither
	// create an account nor be matched against an existing one.
	ErrEmailNotVerified = errors.New("email not verified by identity provider")
	// ErrIdentityConflict means the email belongs to an account that is
	// already linked to a different subject at the same provider — typically a
	// deleted Workspace account whose address was reassigned to someone else.
	ErrIdentityConflict = errors.New("account is linked to a different external identity")
)
