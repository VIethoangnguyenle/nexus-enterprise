package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/drive/internal/domain"
)

// settingsSpy records calls to the settings methods.
type settingsSpy struct {
	DriveService
	getResult    *domain.DriveSettings
	getErr       error
	updateResult *domain.DriveSettings
	updateErr    error
	gotUserID    string
	gotWSID      string
	gotInput     *domain.DriveSettingsPatch
}

func (s *settingsSpy) GetDriveSettings(_ context.Context, userID, wsID string) (*domain.DriveSettings, error) {
	s.gotUserID, s.gotWSID = userID, wsID
	return s.getResult, s.getErr
}

func (s *settingsSpy) UpdateDriveSettings(_ context.Context, userID, wsID string, in *domain.DriveSettingsPatch) (*domain.DriveSettings, error) {
	s.gotUserID, s.gotWSID, s.gotInput = userID, wsID, in
	return s.updateResult, s.updateErr
}

func TestDriveSettings_GET_defaults(t *testing.T) {
	spy := &settingsSpy{getResult: domain.DefaultDriveSettings()}
	h := NewHandler(spy, nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/drive/settings?workspace_id=ws-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	httputil.SetClaims(c, &httputil.Claims{UserID: "u1", NGACNodeID: "n1"})

	if err := h.GetDriveSettings(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if spy.gotUserID != "u1" || spy.gotWSID != "ws-1" {
		t.Fatalf("forwarded user=%q ws=%q", spy.gotUserID, spy.gotWSID)
	}
}

func TestDriveSettings_PUT_valid(t *testing.T) {
	result := &domain.DriveSettings{View: "grid", Sort: "name", FoldersFirst: false, CellularPreviews: false, NotifyShared: false}
	spy := &settingsSpy{updateResult: result}
	h := NewHandler(spy, nil)
	e := echo.New()
	body := `{"view":"grid","sort":"name","folders_first":false,"cellular_previews":false,"notify_shared":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/drive/settings?workspace_id=ws-1", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	httputil.SetClaims(c, &httputil.Claims{UserID: "u1", NGACNodeID: "n1"})

	if err := h.UpdateDriveSettings(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// A field the body leaves out reaches the domain as absent, not as its zero
// value, so the domain keeps the stored value.
func TestDriveSettings_PUT_partial_body_leaves_absent_fields_nil(t *testing.T) {
	spy := &settingsSpy{updateResult: domain.DefaultDriveSettings()}
	h := NewHandler(spy, nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/drive/settings?workspace_id=ws-1", strings.NewReader(`{"sort":"name","notify_shared":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	httputil.SetClaims(c, &httputil.Claims{UserID: "u1", NGACNodeID: "n1"})

	if err := h.UpdateDriveSettings(c); err != nil {
		t.Fatal(err)
	}
	in := spy.gotInput
	if in == nil || in.Sort == nil || *in.Sort != "name" || in.NotifyShared == nil || *in.NotifyShared {
		t.Fatalf("sent fields not forwarded: %+v", in)
	}
	if in.View != nil || in.FoldersFirst != nil || in.CellularPreviews != nil {
		t.Fatalf("absent fields must stay nil: %+v", in)
	}
}

func TestDriveSettings_PUT_invalid_view(t *testing.T) {
	spy := &settingsSpy{updateErr: domain.ErrInvalidInput}
	h := NewHandler(spy, nil)
	e := echo.New()
	body := `{"view":"tiles","sort":"name"}`
	req := httptest.NewRequest(http.MethodPut, "/api/drive/settings?workspace_id=ws-1", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	httputil.SetClaims(c, &httputil.Claims{UserID: "u1", NGACNodeID: "n1"})

	err := h.UpdateDriveSettings(c)
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", err)
	}
}

func TestDriveSettings_GET_nonmember(t *testing.T) {
	spy := &settingsSpy{getErr: domain.ErrAccessDenied}
	h := NewHandler(spy, nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/drive/settings?workspace_id=ws-1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	httputil.SetClaims(c, &httputil.Claims{UserID: "u1", NGACNodeID: "n1"})

	err := h.GetDriveSettings(c)
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %v", err)
	}
}

func TestDriveSettings_GET_missing_workspace(t *testing.T) {
	spy := &settingsSpy{}
	h := NewHandler(spy, nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/drive/settings", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	httputil.SetClaims(c, &httputil.Claims{UserID: "u1", NGACNodeID: "n1"})

	err := h.GetDriveSettings(c)
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v", err)
	}
}
