// Package domain contains the business logic for the auth service.
// It orchestrates user registration, login, tenant management, and NGAC graph setup.
// No SQL or gRPC/HTTP parsing lives here — only domain rules.
package domain

import "errors"

var (
	// ErrInvalidCredentials indicates username/password mismatch.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUserExists indicates the email or username is already taken.
	ErrUserExists = errors.New("already exists")
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = errors.New("not found")
	// ErrInvalidInput indicates missing or malformed request fields.
	ErrInvalidInput = errors.New("invalid input")
	// ErrAccessDenied indicates the user does not have permission.
	ErrAccessDenied = errors.New("access denied")
	// ErrTenantNotFound indicates the requested tenant does not exist.
	ErrTenantNotFound = errors.New("tenant not found")
	// ErrOTPExpired indicates the OTP session has expired or does not exist.
	ErrOTPExpired = errors.New("otp expired or not found")
	// ErrOTPInvalid indicates the OTP code does not match.
	ErrOTPInvalid = errors.New("invalid otp code")
	// ErrTooManyAttempts indicates the OTP session exceeded max verify attempts.
	ErrTooManyAttempts = errors.New("too many attempts")
)
