package mailer_test

import (
	"context"
	"testing"
	"time"

	"ngac-platform/pkg/mailer"
	"ngac-platform/testutil"
)

func TestSend_ReusesTheConnectionWhileItLives(t *testing.T) {
	f := testutil.NewFakeSMTP(t, false, nil)
	s := senderFor(t, f, "starttls", nil)
	t.Cleanup(func() { _ = s.Close() })
	for i := 0; i < 3; i++ {
		if err := s.Send(context.Background(), sample); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if got := len(f.Messages()); got != 3 {
		t.Fatalf("messages = %d, want 3", got)
	}
	if conns := f.Conns(); conns != 1 {
		t.Errorf("connections = %d, want 1: the session must be reused", conns)
	}
	if auths := len(f.Auths()); auths != 1 {
		t.Errorf("auths = %d, want 1", auths)
	}
}

func TestSend_ReconnectsAfterIdleAndAfterClose(t *testing.T) {
	f := testutil.NewFakeSMTP(t, false, nil)
	s := senderFor(t, f, "starttls", func(c *mailer.Config) { c.IdleTimeout = 50 * time.Millisecond })
	if err := s.Send(context.Background(), sample); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if err := s.Send(context.Background(), sample); err != nil {
		t.Fatalf("send after idle close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), sample); err != nil {
		t.Fatalf("send after Close: %v", err)
	}
	if got := len(f.Messages()); got != 3 {
		t.Errorf("messages = %d, want 3", got)
	}
	if conns := f.Conns(); conns != 3 {
		t.Errorf("connections = %d, want 3", conns)
	}
	_ = s.Close()
}

func TestSend_RejectedMessageDoesNotPoisonTheNextSend(t *testing.T) {
	f := testutil.NewFakeSMTP(t, false, func(f *testutil.FakeSMTP) { f.RejectData = true })
	s := senderFor(t, f, "starttls", nil)
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Send(context.Background(), sample); err == nil {
		t.Fatal("a rejected message must fail")
	}
	f.RejectData = false
	if err := s.Send(context.Background(), sample); err != nil {
		t.Fatalf("send after a rejected one: %v", err)
	}
	if got := len(f.Messages()); got != 1 {
		t.Errorf("messages = %d, want 1", got)
	}
}
