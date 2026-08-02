package domain

import "ngac-platform/pkg/httputil"

import "errors"

// These alias the shared sentinels rather than declaring new ones. errors.Is
// compares identity, not message text, so a local errors.New with the same
// string would be a different value and httputil.MapDomainError would never
// match it — every failure would surface as 500, including denials.
var (
	ErrNotFound      = httputil.ErrNotFound
	ErrAccessDenied  = httputil.ErrAccessDenied
	ErrAlreadyExists = httputil.ErrAlreadyExists
	ErrInvalidInput  = httputil.ErrInvalidInput
)

// Error type checkers for REST/gRPC layer translation.
func IsNotFound(err error) bool      { return errors.Is(err, ErrNotFound) }
func IsAccessDenied(err error) bool  { return errors.Is(err, ErrAccessDenied) }
func IsAlreadyExists(err error) bool { return errors.Is(err, ErrAlreadyExists) }
func IsInvalidInput(err error) bool  { return errors.Is(err, ErrInvalidInput) }
