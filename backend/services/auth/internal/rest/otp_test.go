package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"ngac-platform/services/auth/internal/domain"
)

type nopSender struct{}

func (nopSender) SendCode(context.Context, string, string, string) error { return nil }

// otpTestService builds a domain service whose OTP status is fully decided by
// opts. The Redis client is never dialled — providers only checks it exists.
func otpTestService(withRedis bool, opts *domain.OTPOptions) *domain.Service {
	var rdb *redis.Client
	if withRedis {
		rdb = redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	}
	svc := domain.NewService(nil, rdb, nil, nil, nil, nil)
	if opts != nil {
		svc.ConfigureOTP(*opts)
	}
	return svc
}

func providersBody(t *testing.T, h *Handler) map[string]bool {
	t.Helper()
	c, rec := request(t, http.MethodGet, "/api/auth/providers")
	if err := h.Providers(c); err != nil {
		t.Fatal(err)
	}
	var body map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestProviders_ReportsOTPMode(t *testing.T) {
	for _, tc := range []struct {
		name          string
		svc           *domain.Service
		wantOTP       bool
		wantFixedCode bool
	}{
		{"default fixed test code", otpTestService(true, nil), true, true},
		{"random codes with a sender", otpTestService(true, &domain.OTPOptions{Sender: nopSender{}}), true, false},
		{"no sender, no fixed code", otpTestService(true, &domain.OTPOptions{}), false, false},
		{"no redis", otpTestService(false, nil), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := providersBody(t, &Handler{svc: tc.svc})
			if body["otp"] != tc.wantOTP || body["otp_fixed_code"] != tc.wantFixedCode {
				t.Errorf("providers = %v, want otp=%v otp_fixed_code=%v", body, tc.wantOTP, tc.wantFixedCode)
			}
			if body["google"] {
				t.Error("google must be off without EnableGoogle")
			}
		})
	}
}

func TestProviders_ReportsGoogle(t *testing.T) {
	h := &Handler{svc: otpTestService(true, nil)}
	h.google = newGoogleFixture().h
	if !providersBody(t, h)["google"] {
		t.Error("google must be reported once enabled")
	}
}

func TestRequestOTP_DisabledIs503(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	h := &Handler{svc: otpTestService(true, &domain.OTPOptions{})}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"identifier":"a@example.test","type":"email"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := echo.New().NewContext(req, httptest.NewRecorder())

	err := h.RequestOTP(c)
	var he *echo.HTTPError
	if !errors.As(err, &he) || he.Code != http.StatusServiceUnavailable {
		t.Fatalf("err = %v, want 503", err)
	}
}

func TestMapError_OTPStatuses(t *testing.T) {
	if got := mapError(domain.ErrOTPRateLimited).Code; got != http.StatusTooManyRequests {
		t.Errorf("rate limited = %d, want 429", got)
	}
	if got := mapError(domain.ErrOTPUnavailable).Code; got != http.StatusServiceUnavailable {
		t.Errorf("unavailable = %d, want 503", got)
	}
}
