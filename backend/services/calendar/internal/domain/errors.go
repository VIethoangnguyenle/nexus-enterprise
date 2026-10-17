// Package domain is the calendar service's business logic. Every decision about
// who may see or change a calendar is a check against the policy service; this
// package decides nothing by role or by table.
package domain

import (
	"errors"
	"fmt"

	"ngac-platform/pkg/httputil"
)

// These alias the shared sentinels rather than declaring new ones: errors.Is
// compares identity, so a local errors.New with the same text would never match
// httputil.MapDomainError and every refusal would surface as a 500.
var (
	ErrNotFound      = httputil.ErrNotFound
	ErrAccessDenied  = httputil.ErrAccessDenied
	ErrAlreadyExists = httputil.ErrAlreadyExists
	ErrInvalidInput  = httputil.ErrInvalidInput
)

// classified is a refusal with a class and a message written for the caller.
// errors.Is matches its class; Error is exactly the message shown.
type classified struct {
	kind error
	msg  string
}

func (e *classified) Error() string        { return e.msg }
func (e *classified) Is(target error) bool { return target == e.kind }

func classify(kind error, format string, args ...any) error {
	return &classified{kind: kind, msg: fmt.Sprintf(format, args...)}
}

func notFound(format string, args ...any) error { return classify(ErrNotFound, format, args...) }
func denied(format string, args ...any) error   { return classify(ErrAccessDenied, format, args...) }
func invalid(format string, args ...any) error  { return classify(ErrInvalidInput, format, args...) }

// internal wraps a failure of ours. It stays unclassified, so a transport
// answers 500 with a generic body and the cause goes only to the log.
func internal(what string, err error) error {
	if err == nil {
		return nil
	}
	var c *classified
	if errors.As(err, &c) {
		return err
	}
	return fmt.Errorf("%s: %w", what, err)
}
