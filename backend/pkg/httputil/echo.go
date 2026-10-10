package httputil

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// NewEcho returns the Echo instance every service's REST edge runs on:
// a request ID on every response (X-Request-ID), the given middleware (the
// access log), panic recovery, and ErrorHandler, so a 500 is answered the same
// way whichever handler produced it.
func NewEcho(service string, middleware ...echo.MiddlewareFunc) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = ErrorHandler(service)
	e.Use(echomw.RequestID())
	e.Use(middleware...)
	e.Use(echomw.Recover())
	return e
}

// RequestID returns the ID of the request being served. The RequestID
// middleware sets it; when it did not run (a handler test, a route outside the
// middleware) one is made up and attached to the response so the client can
// still quote it.
func RequestID(c echo.Context) string {
	if id := c.Response().Header().Get(echo.HeaderXRequestID); id != "" {
		return id
	}
	if id := c.Request().Header.Get(echo.HeaderXRequestID); id != "" {
		c.Response().Header().Set(echo.HeaderXRequestID, id)
		return id
	}
	var b [8]byte
	_, _ = rand.Read(b[:])
	id := hex.EncodeToString(b[:])
	c.Response().Header().Set(echo.HeaderXRequestID, id)
	return id
}

// LogInternal records the detail of a failure the client is not told about,
// under the request's ID. ErrorHandler uses it; a service with its own error
// envelope (auth) calls it too.
func LogInternal(c echo.Context, service string, cause error) string {
	id := RequestID(c)
	slog.Error(service+" request failed",
		"request_id", id,
		"method", c.Request().Method,
		"path", c.Path(),
		"error", cause)
	return id
}

// ErrorHandler is Echo's HTTPErrorHandler for the platform. A 4xx is the
// caller's to read and goes through Echo's default handling unchanged. A 500 —
// an HTTPError with that status, an error that is not an HTTPError at all, a
// recovered panic — is answered {"message": "internal error", "request_id": …}
// and its cause is logged with the same request ID. Nothing the error says
// reaches the client.
func ErrorHandler(service string) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		var he *echo.HTTPError
		isHTTP := errors.As(err, &he)
		if isHTTP && he.Code != http.StatusInternalServerError {
			c.Echo().DefaultHTTPErrorHandler(err, c)
			return
		}

		cause := err
		if isHTTP {
			switch {
			case he.Internal != nil:
				cause = he.Internal
			case he.Message != nil && he.Message != InternalMessage:
				cause = fmt.Errorf("%v", he.Message)
			}
		}
		id := LogInternal(c, service, cause)

		if c.Request().Method == http.MethodHead {
			_ = c.NoContent(http.StatusInternalServerError)
			return
		}
		if werr := c.JSON(http.StatusInternalServerError, map[string]any{
			"message":    InternalMessage,
			"request_id": id,
		}); werr != nil {
			slog.Error("could not write an error answer", "service", service, "error", werr)
		}
	}
}
