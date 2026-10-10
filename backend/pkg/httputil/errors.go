package httputil

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/grpcauth"
)

// The domain sentinel errors, defined once.
//
// Services MUST alias these rather than declare their own:
//
//	var ErrAccessDenied = httputil.ErrAccessDenied
//
// errors.Is compares identity, not message text, so two errors.New calls with
// the same string are different values and never match. Every service used to
// declare its own copy — the comment here even instructed it — which meant
// MapDomainError matched nothing and every domain failure in every service was
// reported as 500, including denials.
var (
	ErrNotFound      = errors.New("not found")
	ErrAccessDenied  = errors.New("access denied")
	ErrAlreadyExists = errors.New("already exists")
	ErrInvalidInput  = errors.New("invalid input")
)

// InternalMessage is the whole of what a client learns about a 500. The cause
// goes to the log, tagged with the request ID the response carries.
const InternalMessage = grpcauth.InternalMessage

// Internal is a 500 whose body is generic. err is kept on the HTTPError, where
// ErrorHandler logs it and the response never shows it: a database error or a
// failed downstream call can name tables, hosts and queries.
func Internal(err error) *echo.HTTPError {
	he := echo.NewHTTPError(http.StatusInternalServerError, InternalMessage)
	if err != nil {
		he.SetInternal(err)
	}
	return he
}

// MapDomainError translates a domain sentinel error into an Echo HTTP error
// with the appropriate status code. The sentinels' messages are written for the
// caller and are sent as they are. Anything else is a failure of ours: it maps
// to a generic 500 (see Internal).
func MapDomainError(err error) *echo.HTTPError {
	switch {
	case errors.Is(err, ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, ErrAccessDenied):
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	case errors.Is(err, ErrAlreadyExists):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidInput):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return Internal(err)
	}
}

// Codes of the errors this package raises. A client branches on the code, never
// on the English message.
const (
	// CodeSessionRequired: the request carries no session at all.
	CodeSessionRequired = "session_required"
	// CodeSessionInvalid: the session is there but cannot be used (bad
	// signature, expired). The client may refresh once, then sign in again.
	CodeSessionInvalid = "session_invalid"
	// CodeTenantRequired: the token is not scoped to a workspace.
	CodeTenantRequired = "tenant_required"
)

// CodedError is an HTTP error whose body is {"message", "code"}: the shape the
// auth service answers every error in, so a screen reads one envelope whichever
// service said no. Echo writes a map message as the body as it is.
func CodedError(status int, code, message string) *echo.HTTPError {
	return echo.NewHTTPError(status, map[string]any{"message": message, "code": code})
}
