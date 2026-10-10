package bootstrap_test

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"ngac-platform/pkg/bootstrap"
)

func capture(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestEnv(t *testing.T) {
	t.Setenv("BOOT_SET", "value")
	t.Setenv("BOOT_EMPTY", "")
	if got := bootstrap.Env("BOOT_SET", "fb"); got != "value" {
		t.Errorf("set: %q", got)
	}
	if got := bootstrap.Env("BOOT_EMPTY", "fb"); got != "fb" {
		t.Errorf("empty falls back: %q", got)
	}
	if got := bootstrap.Env("BOOT_UNSET_XYZ", "fb"); got != "fb" {
		t.Errorf("unset falls back: %q", got)
	}
}

func TestPolicyAddrPrefersTheCurrentName(t *testing.T) {
	logs := capture(t)
	t.Setenv("POLICY_SERVICE_ADDR", "new:1")
	t.Setenv("POLICY_ADDR", "old:2")
	if got := bootstrap.PolicyAddr(); got != "new:1" {
		t.Fatalf("got %q, want new:1", got)
	}
	if strings.Contains(logs.String(), "deprecated") {
		t.Errorf("no warning expected when the current name is set: %s", logs)
	}
}

func TestPolicyAddrStillHonoursTheOldNameAndSaysSo(t *testing.T) {
	logs := capture(t)
	t.Setenv("POLICY_SERVICE_ADDR", "")
	t.Setenv("POLICY_ADDR", "old:2")
	if got := bootstrap.PolicyAddr(); got != "old:2" {
		t.Fatalf("got %q, want old:2", got)
	}
	out := logs.String()
	if !strings.Contains(out, "deprecated") || !strings.Contains(out, "POLICY_ADDR") || !strings.Contains(out, "POLICY_SERVICE_ADDR") {
		t.Errorf("deprecation warning must name both variables: %s", out)
	}
}

func TestPolicyAddrDefault(t *testing.T) {
	t.Setenv("POLICY_SERVICE_ADDR", "")
	t.Setenv("POLICY_ADDR", "")
	if got := bootstrap.PolicyAddr(); got != "localhost:50051" {
		t.Fatalf("got %q", got)
	}
}

func TestConnectDBRejectsABadURLAndAnUnreachableServer(t *testing.T) {
	if _, err := bootstrap.ConnectDB(context.Background(), "::not a url::"); err == nil {
		t.Error("a malformed URL must fail")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := bootstrap.ConnectDB(ctx, "postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1"); err == nil {
		t.Error("an unreachable server must fail")
	}
}

type recorder struct {
	mu    sync.Mutex
	steps []string
}

func (r *recorder) add(s string) { r.mu.Lock(); r.steps = append(r.steps, s); r.mu.Unlock() }

type fakeGRPC struct {
	rec    *recorder
	hang   bool
	unhang chan struct{}
}

func (f *fakeGRPC) GracefulStop() {
	f.rec.add("grpc-graceful")
	if f.hang {
		<-f.unhang
	}
}
func (f *fakeGRPC) Stop() {
	f.rec.add("grpc-stop")
	close(f.unhang)
}

type fakeHTTP struct {
	rec  *recorder
	name string
}

func (f fakeHTTP) Shutdown(context.Context) error { f.rec.add(f.name); return nil }

func TestShutdownStopsEverythingInOrder(t *testing.T) {
	rec := &recorder{}
	sig := make(chan os.Signal, 1)
	sig <- syscall.SIGTERM

	bootstrap.Shutdown{
		GRPC:    &fakeGRPC{rec: rec},
		HTTP:    []bootstrap.HTTPServer{fakeHTTP{rec, "rest"}, fakeHTTP{rec, "ws"}},
		Cancel:  func() { rec.add("cancel") },
		Cleanup: func() { rec.add("cleanup") },
		Signals: sig,
	}.Wait()

	got := strings.Join(rec.steps, ",")
	if want := "cancel,cleanup,rest,ws,grpc-graceful"; got != want {
		t.Fatalf("order = %s, want %s", got, want)
	}
}

func TestShutdownForcesAHungGRPCServerStopAfterTheTimeout(t *testing.T) {
	rec := &recorder{}
	sig := make(chan os.Signal, 1)
	sig <- syscall.SIGINT

	done := make(chan struct{})
	go func() {
		bootstrap.Shutdown{
			GRPC:    &fakeGRPC{rec: rec, hang: true, unhang: make(chan struct{})},
			Timeout: 50 * time.Millisecond,
			Signals: sig,
		}.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Wait did not return for a hung gRPC server")
	}
	if got := strings.Join(rec.steps, ","); got != "grpc-graceful,grpc-stop" {
		t.Fatalf("steps = %s", got)
	}
}

func TestConfigureInternalIdentity(t *testing.T) {
	const strong = "a-production-grade-identity-secret-0123456789"
	cases := map[string]struct {
		env     map[string]string
		wantErr string
	}{
		"dev default":               {map[string]string{"APP_ENV": "dev"}, ""},
		"dev explicit":              {map[string]string{"APP_ENV": "local", "INTERNAL_IDENTITY_SECRET": strong}, ""},
		"prod unset":                {map[string]string{"APP_ENV": "production"}, "is not set"},
		"unset env unset secret":    {map[string]string{}, "is not set"},
		"prod placeholder":          {map[string]string{"APP_ENV": "production", "INTERNAL_IDENTITY_SECRET": bootstrap.DevInternalIdentitySecret}, "placeholder"},
		"prod strong":               {map[string]string{"APP_ENV": "production", "INTERNAL_IDENTITY_SECRET": strong}, ""},
		"same as jwt":               {map[string]string{"APP_ENV": "production", "INTERNAL_IDENTITY_SECRET": strong, "JWT_SECRET": strong}, "must differ from JWT_SECRET"},
		"previous same as jwt":      {map[string]string{"APP_ENV": "production", "INTERNAL_IDENTITY_SECRET": strong, "INTERNAL_IDENTITY_SECRET_PREVIOUS": strong + "-jwt", "JWT_SECRET": strong + "-jwt"}, "must differ from JWT_SECRET"},
		"31 bytes":                  {map[string]string{"APP_ENV": "production", "INTERNAL_IDENTITY_SECRET": strings.Repeat("k", 31)}, "at least 32"},
		"too short":                 {map[string]string{"APP_ENV": "production", "INTERNAL_IDENTITY_SECRET": "short"}, "at least"},
		"prod previous placeholder": {map[string]string{"APP_ENV": "production", "INTERNAL_IDENTITY_SECRET": strong, "INTERNAL_IDENTITY_SECRET_PREVIOUS": bootstrap.DevInternalIdentitySecret}, "placeholder"},
		"prod with previous":        {map[string]string{"APP_ENV": "production", "INTERNAL_IDENTITY_SECRET": strong, "INTERNAL_IDENTITY_SECRET_PREVIOUS": strong + "-old"}, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for _, k := range []string{"APP_ENV", "INTERNAL_IDENTITY_SECRET", "INTERNAL_IDENTITY_SECRET_PREVIOUS", "JWT_SECRET"} {
				t.Setenv(k, tc.env[k])
			}
			err := bootstrap.ConfigureInternalIdentity()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}
