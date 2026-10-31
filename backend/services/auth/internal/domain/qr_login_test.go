package domain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/store"
)

const chromeOnWindows = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

// qrWorld is a service with real Redis, one tenant and one active member of it.
type qrWorld struct {
	svc    *domain.Service
	w      *fakeWorld
	phone  *store.User
	tenant string
}

func newQRWorld(t *testing.T) qrWorld {
	t.Helper()
	rdb := testRedisClient(t)
	w := newFakeWorld()
	auth.SetJWTSecret("test-secret-key-for-testing-only")
	svc := domain.NewService(w, rdb, &fakePolicyRead{}, &fakePolicyWrite{w: w}, &fakeWorkspace{w: w}, &fakeMessaging{w: w})
	u := w.addUser("qr-" + time.Now().Format("150405.000000000") + "@example.test")
	tenant := w.addTenant("NovaPay", "")
	if err := w.InsertTenantUser(context.Background(), tenant, u.ID, "member", "active", u.NGACNodeID); err != nil {
		t.Fatal(err)
	}
	return qrWorld{svc: svc, w: w, phone: u, tenant: tenant}
}

func (q qrWorld) phoneOf(u *store.User, tenant string) domain.QRPhone {
	return domain.QRPhone{UserID: u.ID, Username: u.Username, NGACNodeID: u.NGACNodeID, TenantID: tenant}
}

func (q qrWorld) create(t *testing.T) domain.QRTicket {
	t.Helper()
	ticket, err := q.svc.CreateQRLogin(context.Background(), chromeOnWindows)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if ticket.Token == "" || ticket.PollSecret == "" || ticket.Token == ticket.PollSecret {
		t.Fatalf("ticket must carry two distinct secrets: %+v", ticket)
	}
	return ticket
}

func TestQRLogin_FullFlowSignsTheComputerInToThePhonesTenant(t *testing.T) {
	q := newQRWorld(t)
	ctx := context.Background()
	ticket := q.create(t)

	poll, err := q.svc.PollQRLogin(ctx, ticket.Token, ticket.PollSecret)
	if err != nil || poll.Status != domain.QRPending {
		t.Fatalf("fresh ticket: %+v, %v", poll, err)
	}

	scan, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant))
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if scan.Browser != "Chrome" || scan.OS != "Windows" || scan.TenantName != "NovaPay" || scan.CreatedAt.IsZero() {
		t.Errorf("scan shows %+v", scan)
	}

	poll, err = q.svc.PollQRLogin(ctx, ticket.Token, ticket.PollSecret)
	if err != nil || poll.Status != domain.QRScanned || poll.Scanner == nil {
		t.Fatalf("after scan: %+v, %v", poll, err)
	}
	if poll.Session != nil {
		t.Fatal("a scan alone must not hand out a session")
	}

	if err := q.svc.ApproveQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); err != nil {
		t.Fatalf("approve: %v", err)
	}
	poll, err = q.svc.PollQRLogin(ctx, ticket.Token, ticket.PollSecret)
	if err != nil || poll.Status != domain.QRApproved || poll.Session == nil {
		t.Fatalf("after approve: %+v, %v", poll, err)
	}
	s := poll.Session
	if s.UserID != q.phone.ID || s.TenantID != q.tenant || s.SessionID == "" {
		t.Errorf("session %+v", s)
	}
	if got := tenantOf(t, s.Token); got != q.tenant {
		t.Errorf("token tenant = %q, want the phone's %q", got, q.tenant)
	}

	// Single use: the session is handed out once.
	if _, err := q.svc.PollQRLogin(ctx, ticket.Token, ticket.PollSecret); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Fatalf("second claim: err = %v, want ErrQRUnavailable", err)
	}
}

func TestQRLogin_OnlyTheBrowserThatCreatedTheCodeGetsTheSession(t *testing.T) {
	q := newQRWorld(t)
	ctx := context.Background()
	ticket := q.create(t)
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); err != nil {
		t.Fatal(err)
	}
	if err := q.svc.ApproveQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); err != nil {
		t.Fatal(err)
	}
	for name, secret := range map[string]string{"missing": "", "wrong": "not-the-secret", "the token itself": ticket.Token} {
		if p, err := q.svc.PollQRLogin(ctx, ticket.Token, secret); !errors.Is(err, domain.ErrQRUnavailable) {
			t.Errorf("%s poll secret: got %+v, %v; want ErrQRUnavailable", name, p, err)
		}
	}
	// The rightful browser can still claim it afterwards.
	if p, err := q.svc.PollQRLogin(ctx, ticket.Token, ticket.PollSecret); err != nil || p.Session == nil {
		t.Fatalf("rightful claim: %+v, %v", p, err)
	}
}

func TestQRLogin_ASecondPhoneCannotTakeOverAScannedCode(t *testing.T) {
	q := newQRWorld(t)
	ctx := context.Background()
	other := q.w.addUser("qr-other-" + time.Now().Format("150405.000000000") + "@example.test")
	if err := q.w.InsertTenantUser(ctx, q.tenant, other.ID, "member", "active", other.NGACNodeID); err != nil {
		t.Fatal(err)
	}
	ticket := q.create(t)
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); err != nil {
		t.Fatal(err)
	}
	// Scanning again from the same phone is fine (the page reloaded).
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); err != nil {
		t.Fatalf("re-scan by the same person: %v", err)
	}
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(other, q.tenant)); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Errorf("scan by another person: err = %v, want ErrQRUnavailable", err)
	}
	if err := q.svc.ApproveQRLogin(ctx, ticket.Token, q.phoneOf(other, q.tenant)); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Errorf("approve by another person: err = %v, want ErrQRUnavailable", err)
	}
	if err := q.svc.DeclineQRLogin(ctx, ticket.Token, q.phoneOf(other, q.tenant)); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Errorf("decline by another person: err = %v, want ErrQRUnavailable", err)
	}
	if p, _ := q.svc.PollQRLogin(ctx, ticket.Token, ticket.PollSecret); p == nil || p.Status != domain.QRScanned {
		t.Errorf("the code must still wait for its scanner, got %+v", p)
	}
}

func TestQRLogin_ApproveNeedsAScanFirst(t *testing.T) {
	q := newQRWorld(t)
	ticket := q.create(t)
	if err := q.svc.ApproveQRLogin(context.Background(), ticket.Token, q.phoneOf(q.phone, q.tenant)); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Errorf("approve before scan: err = %v, want ErrQRUnavailable", err)
	}
}

func TestQRLogin_RefusesAPhoneThatIsNoLongerAMemberOfItsTenant(t *testing.T) {
	q := newQRWorld(t)
	ctx := context.Background()

	// Not a member at scan time.
	stranger := q.w.addUser("qr-stranger-" + time.Now().Format("150405.000000000") + "@example.test")
	ticket := q.create(t)
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(stranger, q.tenant)); !errors.Is(err, domain.ErrAccessDenied) {
		t.Errorf("scan by a non-member: err = %v, want ErrAccessDenied", err)
	}

	// Removed between scan and approve.
	ticket = q.create(t)
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); err != nil {
		t.Fatal(err)
	}
	q.w.membership(q.tenant, q.phone.ID).Status = "disabled"
	if err := q.svc.ApproveQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); !errors.Is(err, domain.ErrAccessDenied) {
		t.Fatalf("approve after removal: err = %v, want ErrAccessDenied", err)
	}
	if p, _ := q.svc.PollQRLogin(ctx, ticket.Token, ticket.PollSecret); p != nil && p.Session != nil {
		t.Fatal("a refused approval must not leave a session to claim")
	}

	// A phone with no tenant at all.
	ticket = q.create(t)
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, "")); !errors.Is(err, domain.ErrAccessDenied) {
		t.Errorf("scan with no tenant: err = %v, want ErrAccessDenied", err)
	}
}

func TestQRLogin_DeclineEndsTheCode(t *testing.T) {
	q := newQRWorld(t)
	ctx := context.Background()
	ticket := q.create(t)
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); err != nil {
		t.Fatal(err)
	}
	if err := q.svc.DeclineQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); err != nil {
		t.Fatalf("decline: %v", err)
	}
	p, err := q.svc.PollQRLogin(ctx, ticket.Token, ticket.PollSecret)
	if err != nil || p.Status != domain.QRDeclined || p.Session != nil {
		t.Fatalf("after decline: %+v, %v", p, err)
	}
	if err := q.svc.ApproveQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Errorf("approve after decline: err = %v, want ErrQRUnavailable", err)
	}
}

func TestQRLogin_UnknownTokensAreIndistinguishable(t *testing.T) {
	q := newQRWorld(t)
	ctx := context.Background()
	for _, tok := range []string{"", "nope", "a-token-that-was-never-issued-0123456789"} {
		if _, err := q.svc.PollQRLogin(ctx, tok, "x"); !errors.Is(err, domain.ErrQRUnavailable) {
			t.Errorf("poll %q: %v", tok, err)
		}
		if _, err := q.svc.ScanQRLogin(ctx, tok, q.phoneOf(q.phone, q.tenant)); !errors.Is(err, domain.ErrQRUnavailable) {
			t.Errorf("scan %q: %v", tok, err)
		}
		if err := q.svc.ApproveQRLogin(ctx, tok, q.phoneOf(q.phone, q.tenant)); !errors.Is(err, domain.ErrQRUnavailable) {
			t.Errorf("approve %q: %v", tok, err)
		}
	}
}

func TestQRLogin_WithoutRedisIsUnavailableNotAPanic(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t) // no Redis
	if _, err := svc.CreateQRLogin(context.Background(), chromeOnWindows); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Errorf("create without Redis: %v", err)
	}
	if _, err := svc.PollQRLogin(context.Background(), "t", "s"); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Errorf("poll without Redis: %v", err)
	}
}

func TestParseUserAgent(t *testing.T) {
	cases := []struct{ ua, browser, os string }{
		{chromeOnWindows, "Chrome", "Windows"},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0", "Edge", "Windows"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15", "Safari", "macOS"},
		{"Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0", "Firefox", "Linux"},
		{"Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36", "Chrome", "ChromeOS"},
		{"Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36", "Chrome", "Android"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1", "Safari", "iOS"},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 OPR/114.0.0.0", "Opera", "Windows"},
		{"curl/8.5.0", "", ""},
		{"", "", ""},
	}
	for _, c := range cases {
		b, o := domain.ParseUserAgent(c.ua)
		if b != c.browser || o != c.os {
			t.Errorf("%q → (%q, %q), want (%q, %q)", c.ua, b, o, c.browser, c.os)
		}
	}
}
