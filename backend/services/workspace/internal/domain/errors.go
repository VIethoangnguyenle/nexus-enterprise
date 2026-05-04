package domain

import "errors"

var (
	// ErrNotFound indicates the requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAccessDenied indicates the user lacks permission for the operation.
	ErrAccessDenied = errors.New("access denied")
	// ErrAlreadyExists indicates a duplicate entity.
	ErrAlreadyExists = errors.New("already exists")
	// ErrInvalidInput indicates missing or malformed request fields.
	ErrInvalidInput = errors.New("invalid input")
)

// Error type checkers for REST/gRPC layer translation.
func IsNotFound(err error) bool     { return errors.Is(err, ErrNotFound) }
func IsAccessDenied(err error) bool  { return errors.Is(err, ErrAccessDenied) }
func IsAlreadyExists(err error) bool { return errors.Is(err, ErrAlreadyExists) }
func IsInvalidInput(err error) bool  { return errors.Is(err, ErrInvalidInput) }

