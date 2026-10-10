package domain

import (
	"errors"
	"fmt"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/asset/internal/store"
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
)

// Refusals only asset has. Transports map them: gRPC to FailedPrecondition,
// Unauthenticated and Unavailable; REST answers a conflict with 409, an
// unauthenticated caller with 401, and treats a lock refusal as a failure of
// the server's.
var (
	// ErrConflict: well-formed and allowed, but the current state forbids it.
	ErrConflict = errors.New("conflict")
	// ErrUnauthenticated: the call carries no caller.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrUnavailable: a dependency needed to answer could not be reached.
	ErrUnavailable = errors.New("unavailable")
)

// classified is a refusal with a class, a message written for the caller and,
// optionally, a machine-readable reason that tells apart refusals sharing a
// class. errors.Is matches its class; Error is exactly the message shown.
type classified struct {
	kind   error
	reason string
	msg    string
}

func (e *classified) Error() string        { return e.msg }
func (e *classified) Is(target error) bool { return target == e.kind }

// Reason returns the machine-readable reason carried by err's refusal, or "".
func Reason(err error) string {
	var c *classified
	if errors.As(err, &c) {
		return c.reason
	}
	return ""
}

func classify(kind error, format string, args ...any) error {
	return &classified{kind: kind, msg: fmt.Sprintf(format, args...)}
}

// Refuse is a refusal of the given class that carries a reason.
func Refuse(kind error, reason, msg string) error {
	return &classified{kind: kind, reason: reason, msg: msg}
}

func notFound(format string, args ...any) error { return classify(ErrNotFound, format, args...) }
func denied(format string, args ...any) error   { return classify(ErrAccessDenied, format, args...) }
func invalid(format string, args ...any) error  { return classify(ErrInvalidInput, format, args...) }
func conflict(format string, args ...any) error { return classify(ErrConflict, format, args...) }
func unauthenticated(format string, args ...any) error {
	return classify(ErrUnauthenticated, format, args...)
}
func unavailable(format string, args ...any) error { return classify(ErrUnavailable, format, args...) }

// lookupErr classifies the failure of a store lookup. A row that is not there is
// a refusal the caller may hear ("asset not found"); any other failure is ours,
// stays unclassified, and keeps its cause for the log.
func lookupErr(err error, what string) error {
	if errors.Is(err, store.ErrNotFound) {
		return notFound("%s not found", what)
	}
	return fmt.Errorf("load %s: %w", what, err)
}
