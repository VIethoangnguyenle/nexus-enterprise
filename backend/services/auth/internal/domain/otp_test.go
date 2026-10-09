package domain_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
)

// recordingSender captures delivered codes instead of sending them.
type recordingSender struct {
	mu    sync.Mutex
	codes []string
	err   error
}

func (r *recordingSender) SendCode(_ context.Context, _, _, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.codes = append(r.codes, code)
	return r.err
}

func (r *recordingSender) last() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.codes) == 0 {
		return ""
	}
	return r.codes[len(r.codes)-1]
}

func testRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

// otpService builds a service with real Redis; opts==nil keeps the defaults.
func otpService(t *testing.T, opts *domain.OTPOptions) (*domain.Service, *fakeWorld, *redis.Client) {
	t.Helper()
	rdb := testRedisClient(t)
	w := newFakeWorld()
	auth.SetJWTSecret("test-secret-key-for-testing-only")
	svc := domain.NewService(w, rdb, &fakePolicyRead{}, &fakePolicyWrite{w: w}, &fakeWorkspace{w: w}, &fakeMessaging{w: w})
	if opts != nil {
		svc.ConfigureOTP(*opts)
	}
	return svc, w, rdb
}

// uniqueEmail keeps tests independent of each other's rate-limit counters.
func uniqueEmail() string {
	return fmt.Sprintf("otp-%d@example.test", time.Now().UnixNano())
}

func TestOTP_DefaultConfigAcceptsFixedTestCode(t *testing.T) {
	svc, _, _ := otpService(t, nil)
	if !svc.OTPEnabled() || !svc.OTPFixedCodeActive() {
		t.Fatal("default config must enable OTP in fixed-code test mode")
	}

	sid, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if _, err := svc.VerifyOTP(context.Background(), sid, "999999"); err != nil {
		t.Fatalf("999999 must be accepted in the default test mode: %v", err)
	}
}

func TestOTP_FixedModeRejectsOtherCodesAndCountsAttempts(t *testing.T) {
	svc, _, _ := otpService(t, &domain.OTPOptions{FixedCode: "999999"})
	sid, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := svc.VerifyOTP(context.Background(), sid, "123456"); !errors.Is(err, domain.ErrOTPInvalid) {
			t.Fatalf("attempt %d: err = %v, want ErrOTPInvalid", i, err)
		}
	}
	if _, err := svc.VerifyOTP(context.Background(), sid, "999999"); !errors.Is(err, domain.ErrTooManyAttempts) {
		t.Fatalf("after 5 wrong attempts even the right code must fail, got %v", err)
	}
}

func TestOTP_RandomModeRejectsFixedCodeAndAcceptsDeliveredCode(t *testing.T) {
	sender := &recordingSender{}
	svc, _, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: sender})
	if svc.OTPFixedCodeActive() {
		t.Fatal("empty fixed code must turn the fixed-code mode off")
	}

	sid, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	code := sender.last()
	if code == "" {
		t.Fatal("the code was not delivered")
	}
	if code != "999999" {
		if _, err := svc.VerifyOTP(context.Background(), sid, "999999"); !errors.Is(err, domain.ErrOTPInvalid) {
			t.Fatalf("999999 must be rejected when the fixed code is off, got %v", err)
		}
	}
	if _, err := svc.VerifyOTP(context.Background(), sid, code); err != nil {
		t.Fatalf("the delivered code must be accepted: %v", err)
	}
	if _, err := svc.VerifyOTP(context.Background(), sid, code); !errors.Is(err, domain.ErrOTPExpired) {
		t.Fatalf("a code works once, got %v", err)
	}
}

func TestOTP_RandomCodesDifferPerRequest(t *testing.T) {
	sender := &recordingSender{}
	svc, _, _ := otpService(t, &domain.OTPOptions{Sender: sender})
	for i := 0; i < 3; i++ {
		if _, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email"); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	c := sender.codes
	if c[0] == c[1] && c[1] == c[2] {
		t.Fatalf("three requests produced the same code %q", c[0])
	}
	for _, code := range c {
		if len(code) != 6 || strings.Trim(code, "0123456789") != "" {
			t.Errorf("code %q is not six digits", code)
		}
	}
}

func TestOTP_RandomModeStoresOnlyAHash(t *testing.T) {
	sender := &recordingSender{}
	svc, _, rdb := otpService(t, &domain.OTPOptions{Sender: sender})
	sid, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := rdb.Get(context.Background(), "otp:"+sid).Result()
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	if strings.Contains(raw, sender.last()) {
		t.Fatal("the plaintext code is stored in Redis")
	}
}

func TestOTP_RandomModeWrongCodeCountsAttempts(t *testing.T) {
	sender := &recordingSender{}
	svc, _, _ := otpService(t, &domain.OTPOptions{Sender: sender})
	sid, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email")
	if err != nil {
		t.Fatal(err)
	}
	wrong := "000000"
	if sender.last() == wrong {
		wrong = "111111"
	}
	for i := 0; i < 5; i++ {
		if _, err := svc.VerifyOTP(context.Background(), sid, wrong); !errors.Is(err, domain.ErrOTPInvalid) {
			t.Fatalf("attempt %d: err = %v", i, err)
		}
	}
	if _, err := svc.VerifyOTP(context.Background(), sid, sender.last()); !errors.Is(err, domain.ErrTooManyAttempts) {
		t.Fatalf("err = %v, want ErrTooManyAttempts", err)
	}
}

func TestOTP_DisabledWithoutSenderOrFixedCode(t *testing.T) {
	svc, _, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: nil})
	if svc.OTPEnabled() {
		t.Fatal("no fixed code and no sender must disable OTP")
	}
	if _, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email"); !errors.Is(err, domain.ErrOTPUnavailable) {
		t.Fatalf("err = %v, want ErrOTPUnavailable", err)
	}
	if _, err := svc.VerifyOTP(context.Background(), "any", "999999"); !errors.Is(err, domain.ErrOTPUnavailable) {
		t.Fatalf("verify err = %v, want ErrOTPUnavailable", err)
	}
}

func TestOTP_RequestIsRateLimitedPerIdentifier(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts domain.OTPOptions
	}{
		{"fixed", domain.OTPOptions{FixedCode: "999999"}},
		{"random", domain.OTPOptions{Sender: &recordingSender{}}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			svc, _, _ := otpService(t, &mode.opts)
			email := uniqueEmail()
			for i := 0; i < 5; i++ {
				if _, err := svc.RequestOTP(context.Background(), email, "email"); err != nil {
					t.Fatalf("request %d: %v", i, err)
				}
			}
			if _, err := svc.RequestOTP(context.Background(), email, "email"); !errors.Is(err, domain.ErrOTPRateLimited) {
				t.Fatalf("6th request err = %v, want ErrOTPRateLimited", err)
			}
			// Another identifier is unaffected.
			if _, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email"); err != nil {
				t.Fatalf("other identifier: %v", err)
			}
		})
	}
}

// captureLogs routes slog to a buffer for the duration of the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestOTP_CodeNeverLogged(t *testing.T) {
	t.Run("random", func(t *testing.T) {
		sender := &recordingSender{}
		svc, _, _ := otpService(t, &domain.OTPOptions{Sender: sender})
		logs := captureLogs(t)
		sid, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.VerifyOTP(context.Background(), sid, sender.last()); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(logs.String(), sender.last()) {
			t.Fatalf("the code appears in the logs:\n%s", logs.String())
		}
	})
	t.Run("fixed", func(t *testing.T) {
		svc, _, _ := otpService(t, &domain.OTPOptions{FixedCode: "424242"})
		logs := captureLogs(t)
		sid, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := svc.VerifyOTP(context.Background(), sid, "424242"); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(logs.String(), "424242") {
			t.Fatalf("the fixed code appears in the logs:\n%s", logs.String())
		}
	})
}

func TestOTP_SendFailureDoesNotLeaveAUsableSession(t *testing.T) {
	sender := &recordingSender{err: errors.New("smtp down")}
	svc, _, _ := otpService(t, &domain.OTPOptions{Sender: sender})
	if _, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email"); err == nil {
		t.Fatal("a failed delivery must fail the request")
	}
}

func TestLogSender_RefusesOutsideDevMode(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("AUTH_DEV_OTP", "")
	logs := captureLogs(t)
	if err := (domain.LogSender{}).SendCode(context.Background(), "a@b.test", "email", "314159"); err == nil {
		t.Fatal("LogSender must refuse to run outside dev mode")
	}
	if strings.Contains(logs.String(), "314159") {
		t.Fatal("LogSender logged a code outside dev mode")
	}
}

func TestLogSender_LogsInDevMode(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	logs := captureLogs(t)
	if err := (domain.LogSender{}).SendCode(context.Background(), "a@b.test", "email", "314159"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.Contains(logs.String(), "314159") {
		t.Fatal("in dev mode the log is the delivery channel")
	}
}
