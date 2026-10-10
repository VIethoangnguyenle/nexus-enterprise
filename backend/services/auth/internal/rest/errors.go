package rest

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
)

// statusCodes are the codes for errors Echo or a middleware raises with only a
// status (an unknown route, a method not allowed, a body that is too large).
var statusCodes = map[int]string{
	http.StatusBadRequest:            "invalid_input",
	http.StatusUnauthorized:          "session_required",
	http.StatusForbidden:             "access_denied",
	http.StatusNotFound:              "not_found",
	http.StatusMethodNotAllowed:      "method_not_allowed",
	http.StatusRequestEntityTooLarge: "too_large",
	http.StatusTooManyRequests:       "rate_limited",
	http.StatusServiceUnavailable:    "unavailable",
}

// envelopeErrors is the HTTP error handler of the auth service: whatever failed,
// the body is {"message", "code"} (plus extra fields where a handler added
// them), so a screen reads one shape and branches on the code. A handler that
// already built the envelope passes through; a bare status gets one; anything
// that is not an HTTP error is an internal error whose text is logged, never sent.
func envelopeErrors(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	he := new(echo.HTTPError)
	if !errors.As(err, &he) {
		slog.Error("auth request failed", "method", c.Request().Method, "path", c.Path(), "error", err)
		he = apiError(http.StatusInternalServerError, "internal", "internal error")
	}

	var body any = he.Message
	switch m := he.Message.(type) {
	case map[string]any:
		// Already the envelope.
	case string:
		code, ok := statusCodes[he.Code]
		if !ok {
			code = "internal"
			if he.Code < 500 {
				code = "invalid_input"
			}
		}
		if he.Code >= 500 {
			// A 5xx message is for the log; the caller gets the plain sentence.
			slog.Error("auth request failed", "status", he.Code, "message", m)
			m = "internal error"
		}
		body = map[string]any{"message": m, "code": code}
	default:
		body = map[string]any{"message": http.StatusText(he.Code), "code": statusCodes[he.Code]}
	}

	if c.Request().Method == http.MethodHead {
		_ = c.NoContent(he.Code)
		return
	}
	if err := c.JSON(he.Code, body); err != nil {
		slog.Error("could not write an error answer", "error", err)
	}
}
