package rest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
)

// brokenStore fails every request lookup the way a dead database would.
type brokenStore struct{ *fakeStore }

func (brokenStore) GetRequest(context.Context, string) (*domain.Request, error) {
	return nil, errors.New(`ERROR: relation "t_acme.approval_requests" does not exist (SQLSTATE 42P01)`)
}

// A failure of ours never shows its text: the client gets a generic message and
// a request ID, the log gets the cause.
func TestInternalFailuresAreSanitised(t *testing.T) {
	h := &Handler{svc: domain.NewService(brokenStore{fixture()}, noScopes{})}
	e := httputil.NewEcho("approval")
	e.GET("/api/approval/requests/:id", func(c echo.Context) error {
		httputil.SetClaims(c, &httputil.Claims{UserID: "u", TenantID: "t", NGACNodeID: hoaN})
		return h.GetRequest(c)
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/approval/requests/"+reqID, nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	body := rec.Body.String()
	for _, leak := range []string{"SQLSTATE", "approval_requests", "t_acme"} {
		if strings.Contains(body, leak) {
			t.Errorf("body leaks %q: %s", leak, body)
		}
	}
	if !strings.Contains(body, `"request_id"`) || !strings.Contains(body, `"internal error"`) {
		t.Errorf("body = %s", body)
	}
}

// The service's own refusals keep their message and status.
func TestRefusalsKeepTheirStatus(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want int
	}{
		"step not active": {domain.ErrStepNotActive, http.StatusConflict},
		"completed":       {domain.ErrRequestCompleted, http.StatusConflict},
		"no template":     {domain.ErrNoMatchingTemplate, http.StatusNotFound},
		"stale":           {domain.ErrStale, http.StatusConflict},
		"denied":          {domain.ErrAccessDenied, http.StatusForbidden},
		"unclassified":    {errors.New("boom"), http.StatusInternalServerError},
	} {
		if got := mapDomainError(tc.err).Code; got != tc.want {
			t.Errorf("%s: %d, want %d", name, got, tc.want)
		}
	}
	if he := mapDomainError(errors.New("pq: secret")); strings.Contains(he.Message.(string), "secret") {
		t.Error("an unclassified error must not reach the message")
	}
}
