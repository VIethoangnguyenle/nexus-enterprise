package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/asset/internal/domain"
)

// settingsSpy records calls to the settings methods.
type settingsSpy struct {
	getResult    *domain.AssetSettings
	getErr       error
	updateResult *domain.AssetSettings
	updateErr    error
	gotUserID    string
	gotWSID      string
	gotInput     *domain.AssetSettingsPatch
}

func (s *settingsSpy) GetAssetSettings(_ context.Context, userID, wsID string) (*domain.AssetSettings, error) {
	s.gotUserID, s.gotWSID = userID, wsID
	return s.getResult, s.getErr
}

func (s *settingsSpy) UpdateAssetSettings(_ context.Context, userID, wsID string, in *domain.AssetSettingsPatch) (*domain.AssetSettings, error) {
	s.gotUserID, s.gotWSID, s.gotInput = userID, wsID, in
	return s.updateResult, s.updateErr
}

func assetSettingsHandler(spy *settingsSpy) *Handler {
	return NewHandler(nil, nil, nil).WithSettings(spy)
}

func TestAssetSettings_GET_defaults(t *testing.T) {
	spy := &settingsSpy{getResult: domain.DefaultAssetSettings()}
	h := assetSettingsHandler(spy)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/ws-1/asset-settings", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("ws-1")
	httputil.SetClaims(c, &httputil.Claims{UserID: "u1", NGACNodeID: "n1"})

	if err := h.GetAssetSettings(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if spy.gotUserID != "u1" || spy.gotWSID != "ws-1" {
		t.Fatalf("forwarded user=%q ws=%q", spy.gotUserID, spy.gotWSID)
	}
}

func TestAssetSettings_PUT_valid(t *testing.T) {
	result := &domain.AssetSettings{DefaultTab: "inventory", Sort: "created", NotifyRequests: false, NotifyMaintenance: false}
	spy := &settingsSpy{updateResult: result}
	h := assetSettingsHandler(spy)
	e := echo.New()
	body := `{"default_tab":"inventory","sort":"created","notify_requests":false,"notify_maintenance":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/workspaces/ws-1/asset-settings", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("ws-1")
	httputil.SetClaims(c, &httputil.Claims{UserID: "u1", NGACNodeID: "n1"})

	if err := h.UpdateAssetSettings(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// A field the body leaves out reaches the domain as absent, not as its zero
// value, so the domain keeps the stored value.
func TestAssetSettings_PUT_partial_body_leaves_absent_fields_nil(t *testing.T) {
	spy := &settingsSpy{updateResult: domain.DefaultAssetSettings()}
	h := assetSettingsHandler(spy)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/workspaces/ws-1/asset-settings", strings.NewReader(`{"sort":"created"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("ws-1")
	httputil.SetClaims(c, &httputil.Claims{UserID: "u1", NGACNodeID: "n1"})

	if err := h.UpdateAssetSettings(c); err != nil {
		t.Fatal(err)
	}
	in := spy.gotInput
	if in == nil || in.Sort == nil || *in.Sort != "created" {
		t.Fatalf("sent field not forwarded: %+v", in)
	}
	if in.DefaultTab != nil || in.NotifyRequests != nil || in.NotifyMaintenance != nil {
		t.Fatalf("absent fields must stay nil: %+v", in)
	}
}

func TestAssetSettings_PUT_invalid_tab(t *testing.T) {
	spy := &settingsSpy{updateErr: domain.ErrInvalidInput}
	h := assetSettingsHandler(spy)
	e := echo.New()
	body := `{"default_tab":"badtab","sort":"name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/workspaces/ws-1/asset-settings", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("ws-1")
	httputil.SetClaims(c, &httputil.Claims{UserID: "u1", NGACNodeID: "n1"})

	err := h.UpdateAssetSettings(c)
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", err)
	}
}

func TestAssetSettings_GET_nonmember(t *testing.T) {
	spy := &settingsSpy{getErr: domain.ErrAccessDenied}
	h := assetSettingsHandler(spy)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces/ws-1/asset-settings", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("ws-1")
	httputil.SetClaims(c, &httputil.Claims{UserID: "u1", NGACNodeID: "n1"})

	err := h.GetAssetSettings(c)
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestAssetSettings_GET_missing_workspace(t *testing.T) {
	spy := &settingsSpy{}
	h := assetSettingsHandler(spy)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/workspaces//asset-settings", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("") // empty workspace
	httputil.SetClaims(c, &httputil.Claims{UserID: "u1", NGACNodeID: "n1"})

	err := h.GetAssetSettings(c)
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", err)
	}
}
