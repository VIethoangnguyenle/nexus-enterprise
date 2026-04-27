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
