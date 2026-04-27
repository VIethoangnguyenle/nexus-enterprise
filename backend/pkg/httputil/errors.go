package httputil

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Common domain sentinel errors that services define in their domain/errors.go.
// These are re-declared here so MapDomainError can match against them without
// importing every service's domain package. Services MUST use errors.New with
// these exact strings, or wrap with %w so errors.Is matches.
var (
	ErrNotFound     = errors.New("not found")
	ErrAccessDenied = errors.New("access denied")
	ErrAlreadyExists = errors.New("already exists")
	ErrInvalidInput = errors.New("invalid input")
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
