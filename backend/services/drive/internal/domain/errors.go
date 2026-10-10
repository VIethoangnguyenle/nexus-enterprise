package domain

import (
	"errors"
	"fmt"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/drive/internal/reason"
)

// The shared sentinels, so grpcutil.Status and httputil.MapDomainError
// recognise this service's refusals.
var (
	ErrNotFound     = httputil.ErrNotFound
	ErrAccessDenied = httputil.ErrAccessDenied
	ErrInvalidInput = httputil.ErrInvalidInput
)

// Refusals only drive has. Transports map them: gRPC to FailedPrecondition,
// ResourceExhausted, Aborted and Unavailable; REST answers a conflict with 409
// and treats the rest as a failure of the server's, as it always has.
var (
	// ErrConflict: well-formed and allowed, but the current state forbids it.
	ErrConflict = errors.New("conflict")
	// ErrQuotaExceeded: the workspace's storage quota would be exceeded.
	ErrQuotaExceeded = errors.New("storage quota exceeded")
	// ErrAborted: another request changed the item first; the caller may retry.
	ErrAborted = errors.New("aborted")
	// ErrUnavailable: a lock could not be had in time; the caller may retry.
	ErrUnavailable = errors.New("unavailable")
	// ErrFolderHasDocuments is a conflict: a folder holding text documents is
	// never deleted with them. Its message is the machine-readable reason.
	ErrFolderHasDocuments = &classified{kind: ErrConflict, msg: reason.FolderHasDocuments}
)

// classified is a refusal with a class and a message written for the caller.
// errors.Is matches its class, so the transports find it without parsing text,
// and Error is exactly the message the caller sees.
type classified struct {
	kind error
	msg  string
}

func (e *classified) Error() string        { return e.msg }
func (e *classified) Is(target error) bool { return target == e.kind }

func refuse(kind error, format string, args ...any) error {
	return &classified{kind: kind, msg: fmt.Sprintf(format, args...)}
}

func notFound(format string, args ...any) error { return refuse(ErrNotFound, format, args...) }
func denied(format string, args ...any) error   { return refuse(ErrAccessDenied, format, args...) }
func invalid(format string, args ...any) error  { return refuse(ErrInvalidInput, format, args...) }
func conflict(format string, args ...any) error { return refuse(ErrConflict, format, args...) }
func aborted(format string, args ...any) error  { return refuse(ErrAborted, format, args...) }
func quotaExceeded(format string, args ...any) error {
	return refuse(ErrQuotaExceeded, format, args...)
}
func unavailable(format string, args ...any) error { return refuse(ErrUnavailable, format, args...) }
