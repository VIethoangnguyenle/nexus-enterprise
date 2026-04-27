// Package domain contains the business logic for the auth service.
// It orchestrates user registration, login, and token management.
// No SQL or gRPC/HTTP parsing lives here — only domain rules.
package domain

import "errors"

var (
	// ErrInvalidCredentials indicates username/password mismatch.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUserExists indicates the username is already taken.
	ErrUserExists = errors.New("already exists")
	// ErrNotFound indicates the requested user does not exist.
	ErrNotFound = errors.New("not found")
	// ErrInvalidInput indicates missing or malformed request fields.
	ErrInvalidInput = errors.New("invalid input")
)
