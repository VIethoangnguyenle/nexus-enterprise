package httputil_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/httputil"
)

const leak = `pq: relation "ngac_nodes" does not exist (SQLSTATE 42P01)`

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func serve(t *testing.T, e *echo.Echo, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

type body struct {
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Reason    string `json:"reason"`
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) body {
	t.Helper()
	var b body
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("body is not JSON: %q (%v)", rec.Body.String(), err)
	}
	return b
}

// Every way a handler can fail with a 500 gets the same answer: a generic
// message and the request ID, with the cause in the log under that ID.
func TestNewEchoAnswersEveryInternalFailureGenerically(t *testing.T) {
	handlers := map[string]echo.HandlerFunc{
		"plain error":           func(echo.Context) error { return errors.New(leak) },
		"wrapped error":         func(echo.Context) error { return fmt.Errorf("list: %w", errors.New(leak)) },
		"MapDomainError":        func(echo.Context) error { return httputil.MapDomainError(errors.New(leak)) },
		"MapGRPCError internal": func(echo.Context) error { return httputil.MapGRPCError(status.Error(codes.Internal, leak)) },
		"MapGRPCError plain":    func(echo.Context) error { return httputil.MapGRPCError(errors.New(leak)) },
		"MapGRPCError unavail":  func(echo.Context) error { return httputil.MapGRPCError(status.Error(codes.Unavailable, leak)) },
		"explicit 500 message": func(echo.Context) error {
			return echo.NewHTTPError(http.StatusInternalServerError, leak)
		},
		"panic": func(echo.Context) error { panic(leak) },
	}
	for name, h := range handlers {
		t.Run(name, func(t *testing.T) {
			logs := captureLogs(t)
			e := httputil.NewEcho("svc")
			e.GET("/x", h)

			rec := serve(t, e, http.MethodGet, "/x")

			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d", rec.Code)
			}
			if strings.Contains(rec.Body.String(), "SQLSTATE") || strings.Contains(rec.Body.String(), "ngac_nodes") {
				t.Fatalf("raw error text reached the client: %s", rec.Body)
			}
			b := decode(t, rec)
			if b.Message != httputil.InternalMessage {
				t.Errorf("message = %q", b.Message)
			}
			if b.RequestID == "" || b.RequestID != rec.Header().Get(echo.HeaderXRequestID) {
				t.Errorf("request_id %q must match the X-Request-ID header %q", b.RequestID, rec.Header().Get(echo.HeaderXRequestID))
			}
			if !strings.Contains(logs.String(), "SQLSTATE 42P01") || !strings.Contains(logs.String(), b.RequestID) {
				t.Errorf("the cause must be logged under the request ID: %s", logs)
			}
		})
	}
}

// A caller's own request ID is kept, so a client can correlate its logs.
func TestRequestIDIsKeptWhenTheClientSendsOne(t *testing.T) {
	captureLogs(t)
	e := httputil.NewEcho("svc")
	e.GET("/x", func(echo.Context) error { return errors.New(leak) })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(echo.HeaderXRequestID, "client-chosen-1")
	e.ServeHTTP(rec, req)
	if got := decode(t, rec).RequestID; got != "client-chosen-1" {
		t.Errorf("request_id = %q", got)
	}
}

// Without the middleware (a handler test) an ID is still made up.
func TestErrorHandlerMakesUpAnIDWhenNoMiddlewareRan(t *testing.T) {
	captureLogs(t)
	e := echo.New()
	e.HTTPErrorHandler = httputil.ErrorHandler("svc")
	e.GET("/x", func(echo.Context) error { return errors.New(leak) })
	rec := serve(t, e, http.MethodGet, "/x")
	if decode(t, rec).RequestID == "" {
		t.Error("no request_id")
	}
}

// The caller's own mistakes are theirs to read.
func TestErrorHandlerLeavesClientErrorsAlone(t *testing.T) {
	e := httputil.NewEcho("svc")
	e.GET("/denied", func(echo.Context) error {
		return httputil.MapDomainError(fmt.Errorf("%w: write on oa-1", httputil.ErrAccessDenied))
	})
	e.GET("/coded", func(echo.Context) error {
		return httputil.CodedError(http.StatusUnauthorized, httputil.CodeSessionInvalid, "session expired")
	})

	rec := serve(t, e, http.MethodGet, "/denied")
	if rec.Code != http.StatusForbidden || decode(t, rec).Message != "access denied: write on oa-1" {
		t.Errorf("denied: %d %s", rec.Code, rec.Body)
	}
	rec = serve(t, e, http.MethodGet, "/coded")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "session_invalid") {
		t.Errorf("coded: %d %s", rec.Code, rec.Body)
	}
	if rec = serve(t, e, http.MethodGet, "/nope"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown route: %d", rec.Code)
	}
}

func TestErrorHandlerHEADHasNoBody(t *testing.T) {
	captureLogs(t)
	e := httputil.NewEcho("svc")
	e.HEAD("/x", func(echo.Context) error { return errors.New(leak) })
	rec := serve(t, e, http.MethodHead, "/x")
	if rec.Code != http.StatusInternalServerError || rec.Body.Len() != 0 {
		t.Errorf("HEAD: %d %q", rec.Code, rec.Body)
	}
}

func TestMapGRPCErrorStatuses(t *testing.T) {
	cases := map[codes.Code]int{
		codes.NotFound:           http.StatusNotFound,
		codes.PermissionDenied:   http.StatusForbidden,
		codes.Unauthenticated:    http.StatusUnauthorized,
		codes.InvalidArgument:    http.StatusBadRequest,
		codes.AlreadyExists:      http.StatusConflict,
		codes.FailedPrecondition: http.StatusConflict,
		codes.Internal:           http.StatusInternalServerError,
		codes.Unavailable:        http.StatusInternalServerError,
		codes.Unknown:            http.StatusInternalServerError,
	}
	for code, want := range cases {
		he := httputil.MapGRPCError(status.Error(code, "msg"))
		if he.Code != want {
			t.Errorf("%v -> %d, want %d", code, he.Code, want)
		}
		if want == http.StatusInternalServerError && he.Message != httputil.InternalMessage {
			t.Errorf("%v: 500 message %q", code, he.Message)
		}
	}
}

func TestMapGRPCErrorCarriesTheReason(t *testing.T) {
	st, err := status.New(codes.FailedPrecondition, "asset changed").
		WithDetails(&errdetails.ErrorInfo{Reason: "state_changed", Domain: "asset"})
	if err != nil {
		t.Fatal(err)
	}
	captureLogs(t)
	e := httputil.NewEcho("svc")
	e.GET("/x", func(echo.Context) error { return httputil.MapGRPCError(st.Err()) })
	rec := serve(t, e, http.MethodGet, "/x")

	b := decode(t, rec)
	if rec.Code != http.StatusConflict || b.Message != "asset changed" || b.Reason != "state_changed" {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
}
