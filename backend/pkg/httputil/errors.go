package httputil

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
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

// MapDomainError translates a domain sentinel error into an Echo HTTP error
// with the appropriate status code. Unknown errors map to 500.
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
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
}
