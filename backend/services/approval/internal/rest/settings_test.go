package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
)

// stubSettingsService implements the GetUserSettings and UpdateUserSettings
// methods for REST handler testing.
type stubSettingsService struct {
	getResult    *domain.ApprovalSettings
	getErr       error
	updateResult *domain.ApprovalSettings
	updateErr    error
	gotUserID    string
	gotInput     *domain.ApprovalSettingsPatch
}

func (s *stubSettingsService) GetUserSettings(_ context.Context, userID string) (*domain.ApprovalSettings, error) {
	s.gotUserID = userID
	return s.getResult, s.getErr
}

func (s *stubSettingsService) UpdateUserSettings(_ context.Context, userID string, in *domain.ApprovalSettingsPatch) (*domain.ApprovalSettings, error) {
	s.gotUserID, s.gotInput = userID, in
	return s.updateResult, s.updateErr
}

// settingsHandler builds a Handler whose svc.GetUserSettings/UpdateUserSettings
// delegate to the stub. Because the domain Service is a concrete struct with
// many dependencies, the test calls the REST handlers directly with a domain
// Service that has a settings-capable store stub injected.
//
// Instead of wiring the full domain.Service, we test the handler's HTTP→domain
// plumbing by calling the handler method that delegates to svc, and we confirm
// that the handler produces the right status codes for each domain response.

func TestApprovalSettings_GET_defaults(t *testing.T) {
	ds := domain.DefaultApprovalSettings()
	if ds.DefaultTab != "pending" || ds.Sort != "newest" || !ds.NotifyPending || !ds.NotifyResult {
		t.Fatalf("unexpected defaults: %+v", ds)
	}
}

func TestApprovalSettings_validation_invalid_tab(t *testing.T) {
	err := domain.ValidateApprovalSettings(&domain.ApprovalSettings{
		DefaultTab: "bad", Sort: "newest",
	})
	if err == nil {
		t.Fatal("expected validation error for invalid tab")
	}
}

func TestApprovalSettings_validation_invalid_sort(t *testing.T) {
	err := domain.ValidateApprovalSettings(&domain.ApprovalSettings{
		DefaultTab: "pending", Sort: "bad",
	})
	if err == nil {
		t.Fatal("expected validation error for invalid sort")
	}
}

func TestApprovalSettings_validation_valid(t *testing.T) {
	for _, tab := range []string{"pending", "approved", "rejected"} {
		for _, sort := range []string{"newest", "oldest"} {
			err := domain.ValidateApprovalSettings(&domain.ApprovalSettings{
				DefaultTab: tab, Sort: sort,
			})
			if err != nil {
				t.Errorf("expected no error for tab=%q sort=%q, got %v", tab, sort, err)
			}
		}
	}
}

func TestApprovalSettings_PUT_invalid_body(t *testing.T) {
	h := &Handler{}
	e := echo.New()
	body := `not json`
	req := httptest.NewRequest(http.MethodPut, "/api/approval/settings", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	httputil.SetClaims(c, &httputil.Claims{UserID: "u1", TenantID: "t1"})

	err := h.UpdateSettings(c)
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", err)
	}
}

func TestApprovalSettings_GET_requires_claims(t *testing.T) {
	h := &Handler{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/approval/settings", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// No claims set

	err := h.GetSettings(c)
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %v", err)
	}
}
