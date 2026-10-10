package rest

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/auth/internal/domain"
)

// However a handler fails with a 500 — a bare error, a recovered panic, the
// helper, an unclassified domain error through mapError — the answer is the
// auth envelope with a generic message and the request ID, and the cause is
// not in it.
func TestInternalFailuresAreSanitisedInTheEnvelope(t *testing.T) {
	const leak = `ERROR: relation "users" does not exist (SQLSTATE 42P01)`
	handlers := map[string]echo.HandlerFunc{
		"bare error":   func(echo.Context) error { return errors.New(leak) },
		"panic":        func(echo.Context) error { panic(leak) },
		"helper":       func(echo.Context) error { return internalFailure(fmt.Errorf("end previous session: %s", leak)) },
		"mapped error": func(c echo.Context) error { return fail(c, errors.New(leak)) },
		"plain 500":    func(echo.Context) error { return echo.NewHTTPError(http.StatusInternalServerError, leak) },
	}
	for name, h := range handlers {
		e := httputil.NewEcho("auth")
		e.HTTPErrorHandler = envelopeErrors
		e.GET("/x", h)

		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

		body := errBody(t, rec)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("%s: status = %d", name, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "SQLSTATE") || strings.Contains(rec.Body.String(), "users") {
			t.Errorf("%s: body leaks: %s", name, rec.Body)
		}
		if body["code"] != "internal" || body["message"] != httputil.InternalMessage {
			t.Errorf("%s: body = %v", name, body)
		}
		if id, _ := body["request_id"].(string); id == "" || id != rec.Header().Get(echo.HeaderXRequestID) {
			t.Errorf("%s: request_id %q must match the header %q", name, id, rec.Header().Get(echo.HeaderXRequestID))
		}
	}
}

// A refusal that is not a failure keeps its code and message.
func TestRefusalsAreNotMadeGeneric(t *testing.T) {
	e := httputil.NewEcho("auth")
	e.HTTPErrorHandler = envelopeErrors
	e.GET("/x", func(c echo.Context) error { return fail(c, domain.ErrAccessDenied) })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	body := errBody(t, rec)
	if rec.Code != http.StatusForbidden || body["code"] != "access_denied" || body["request_id"] != nil {
		t.Errorf("%d %v", rec.Code, body)
	}
}
