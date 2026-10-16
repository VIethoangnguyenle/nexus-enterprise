package domain_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ngac-platform/services/auth/internal/domain"
)

// scriptedSender is a CodeSender whose behaviour each test sets.
type scriptedSender struct {
	delay   time.Duration
	failFor int32 // the first failFor calls fail
	calls   atomic.Int32
	mu      sync.Mutex
	codes   []string
	block   chan struct{} // when set, SendCode waits for it (or ctx)
}

func (s *scriptedSender) SendCode(ctx context.Context, _, _, code string) error {
	n := s.calls.Add(1)
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if n <= s.failFor {
		return errors.New("smtp: connect: refused")
	}
	s.mu.Lock()
	s.codes = append(s.codes, code)
	s.mu.Unlock()
	return nil
}

func (s *scriptedSender) delivered() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.codes...)
}

// lockedBuffer lets the worker log while the test reads.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func fastQueue(inner domain.CodeSender, size int) *domain.QueuedSender {
	return domain.NewQueuedSender(inner, domain.QueueOptions{Size: size, Retries: 3, Backoff: 5 * time.Millisecond})
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestQueuedSender_SlowMailerDoesNotHoldTheRequest(t *testing.T) {
	inner := &scriptedSender{delay: 300 * time.Millisecond}
	q := fastQueue(inner, 8)
	t.Cleanup(func() { q.Close(2 * time.Second) })

	start := time.Now()
	if err := q.SendCode(context.Background(), "a@b.test", "email", "123456"); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Fatalf("SendCode took %v, it must only enqueue", d)
	}
	waitFor(t, "delivery", func() bool { return len(inner.delivered()) == 1 })
	if q.Stats().Sent != 1 {
		t.Errorf("sent = %d, want 1", q.Stats().Sent)
	}
}

func TestQueuedSender_RetriesThenSucceeds(t *testing.T) {
	inner := &scriptedSender{failFor: 2}
	q := fastQueue(inner, 8)
	t.Cleanup(func() { q.Close(2 * time.Second) })
	if err := q.SendCode(context.Background(), "a@b.test", "email", "123456"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "delivery after retries", func() bool { return len(inner.delivered()) == 1 })
	if s := q.Stats(); s.Failed != 0 || s.Sent != 1 {
		t.Errorf("stats = %+v, want 1 sent and 0 failed", s)
	}
}

func TestQueuedSender_CountsFailureAfterRetriesWithoutLeakingTheCode(t *testing.T) {
	buf := &lockedBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	inner := &scriptedSender{failFor: 1000}
	q := fastQueue(inner, 8)
	t.Cleanup(func() { q.Close(2 * time.Second) })
	if err := q.SendCode(context.Background(), "someone@b.test", "email", "314159"); err != nil {
		t.Fatalf("the request side must not see delivery failures: %v", err)
	}
	waitFor(t, "failure counted", func() bool { return q.Stats().Failed == 1 })
	if got := inner.calls.Load(); got != 4 {
		t.Errorf("attempts = %d, want 1 + 3 retries", got)
	}
	logs := buf.String()
	if strings.Contains(logs, "314159") || strings.Contains(logs, "someone@b.test") {
		t.Errorf("failure log leaks the code or the address:\n%s", logs)
	}
	if !strings.Contains(logs, "OTP email delivery failed") {
		t.Errorf("failure was not logged:\n%s", logs)
	}
}

func TestQueuedSender_FullQueueRefusesSoftlyWithoutBlocking(t *testing.T) {
	inner := &scriptedSender{block: make(chan struct{})}
	q := fastQueue(inner, 2)
	t.Cleanup(func() { close(inner.block); q.Close(2 * time.Second) })

	// One job is in the worker's hands, two fill the queue.
	for i := 0; i < 3; i++ {
		if err := q.SendCode(context.Background(), "a@b.test", "email", "111111"); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
		if i == 0 {
			waitFor(t, "worker to take the first job", func() bool { return inner.calls.Load() == 1 })
		}
	}
	start := time.Now()
	err := q.SendCode(context.Background(), "a@b.test", "email", "222222")
	if !errors.Is(err, domain.ErrOTPBusy) {
		t.Fatalf("err = %v, want ErrOTPBusy", err)
	}
	if d := time.Since(start); d > 50*time.Millisecond {
		t.Errorf("a full queue blocked for %v", d)
	}
	if q.Stats().Rejected != 1 {
		t.Errorf("rejected = %d, want 1", q.Stats().Rejected)
	}
}

func TestQueuedSender_CloseDrainsThenDropsAndCounts(t *testing.T) {
	t.Run("drains what it can in the grace period", func(t *testing.T) {
		inner := &scriptedSender{delay: 20 * time.Millisecond}
		q := fastQueue(inner, 8)
		for i := 0; i < 3; i++ {
			if err := q.SendCode(context.Background(), "a@b.test", "email", "123456"); err != nil {
				t.Fatal(err)
			}
		}
		if dropped := q.Close(2 * time.Second); dropped != 0 {
			t.Errorf("dropped = %d, want 0", dropped)
		}
		if got := len(inner.delivered()); got != 3 {
			t.Errorf("delivered = %d, want 3", got)
		}
	})
	t.Run("drops the rest when the grace period ends", func(t *testing.T) {
		inner := &scriptedSender{block: make(chan struct{})}
		q := fastQueue(inner, 8)
		for i := 0; i < 4; i++ {
			if err := q.SendCode(context.Background(), "a@b.test", "email", "123456"); err != nil {
				t.Fatal(err)
			}
		}
		waitFor(t, "worker to take the first job", func() bool { return inner.calls.Load() == 1 })
		start := time.Now()
		dropped := q.Close(100 * time.Millisecond)
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("Close took %v, it must give up after the grace period", d)
		}
		if dropped != 4 {
			t.Errorf("dropped = %d, want 4 (one in flight, three queued)", dropped)
		}
		if err := q.SendCode(context.Background(), "a@b.test", "email", "1"); !errors.Is(err, domain.ErrOTPBusy) {
			t.Errorf("a closed queue must refuse, got %v", err)
		}
	})
}

func TestRequestOTP_DoesNotWaitForSlowMail(t *testing.T) {
	inner := &scriptedSender{delay: 5 * time.Second}
	q := fastQueue(inner, 8)
	t.Cleanup(func() { q.Close(50 * time.Millisecond) })
	svc, _, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: q})

	start := time.Now()
	sid, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email")
	if err != nil || sid == "" {
		t.Fatalf("RequestOTP: %q, %v", sid, err)
	}
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Fatalf("RequestOTP took %v, want < 100ms with a 5s mailer", d)
	}
}

func TestRequestOTP_FailingMailStillAnswersOK(t *testing.T) {
	inner := &scriptedSender{failFor: 1000}
	q := fastQueue(inner, 8)
	t.Cleanup(func() { q.Close(time.Second) })
	svc, _, _ := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: q})

	if _, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email"); err != nil {
		t.Fatalf("a mail failure must not change the answer: %v", err)
	}
	waitFor(t, "failure counted", func() bool { return q.Stats().Failed == 1 })
}

func TestRequestOTP_FullQueueIsASoftRefusalAndLeavesNoSession(t *testing.T) {
	inner := &scriptedSender{block: make(chan struct{})}
	q := fastQueue(inner, 1)
	t.Cleanup(func() { close(inner.block); q.Close(time.Second) })
	svc, _, rdb := otpService(t, &domain.OTPOptions{FixedCode: "", Sender: q})

	for i := 0; i < 2; i++ {
		if _, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email"); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if i == 0 {
			waitFor(t, "worker to take the first job", func() bool { return inner.calls.Load() == 1 })
		}
	}
	keysBefore := rdb.Keys(context.Background(), "otp:*").Val()
	_, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email")
	if !errors.Is(err, domain.ErrOTPBusy) {
		t.Fatalf("err = %v, want ErrOTPBusy", err)
	}
	if after := rdb.Keys(context.Background(), "otp:*").Val(); len(after) != len(keysBefore) {
		t.Errorf("a refused request left a session behind: %d -> %d", len(keysBefore), len(after))
	}
}
