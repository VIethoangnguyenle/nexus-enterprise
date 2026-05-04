// Package domain defines sentinel errors for the approval service.
// All domain errors are translated to gRPC status codes in the grpc/
// package and to HTTP status codes in the rest/ package via mapError.
package domain

import (
	"errors"

	"ngac-platform/pkg/httputil"
)

var (
	// Aliased from httputil so that httputil.MapDomainError correctly matches
	// these via errors.Is — both packages must reference the same pointer.
	ErrNotFound      = httputil.ErrNotFound
	ErrAccessDenied  = httputil.ErrAccessDenied
	ErrAlreadyExists = httputil.ErrAlreadyExists
	ErrInvalidInput  = httputil.ErrInvalidInput

	// Domain-specific errors (no httputil equivalent needed).
	ErrStepNotActive      = errors.New("step not active")
	ErrRequestCompleted   = errors.New("request already completed")
	ErrNoMatchingTemplate = errors.New("no matching approval template")
)

