package domain

import (
	"errors"

	"ngac-platform/pkg/httputil"
)

// These alias the shared sentinels rather than declaring new ones. errors.Is
// compares identity, not message text, so a local errors.New with the same
// string would be a different value and httputil.MapDomainError would never
// match it — every failure would surface as 500, including denials.
var (
	ErrNotFound      = httputil.ErrNotFound
	ErrAccessDenied  = httputil.ErrAccessDenied
	ErrAlreadyExists = httputil.ErrAlreadyExists
	ErrInvalidInput  = httputil.ErrInvalidInput
	ErrUnavailable   = httputil.ErrUnavailable
)

// ErrAlreadyOrganization means the workspace is an organization already. Turning
// a personal workspace into one is a single step that is not undone.
var ErrAlreadyOrganization = errors.New("workspace is already an organization")
