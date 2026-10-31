package domain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/store"
)

// deskIP is the computer's address; phones scan from it unless a test says otherwise.
const deskIP = "203.0.113.10"

const chromeOnWindows = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

// qrWorld is a service with real Redis, one tenant and one active member of it.
type qrWorld struct {
	t      *testing.T
	rdb    *redis.Client
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
	return qrWorld{t: t, rdb: rdb, svc: svc, w: w, phone: u, tenant: tenant}
}

// phoneOf is u signed in on a phone in tenant: a live session with a refresh family.
func (q qrWorld) phoneOf(u *store.User, tenant string) domain.QRPhone {
	q.t.Helper()
	_, sessionID, err := auth.GenerateToken(u.ID, u.Username, u.NGACNodeID, tenant)
	if err != nil {
		q.t.Fatal(err)
	}
	if _, err := q.svc.AttachRefreshToken(context.Background(), domain.RefreshIdentity{
		UserID: u.ID, Username: u.Username, NGACNodeID: u.NGACNodeID, TenantID: tenant, SessionID: sessionID,
	}); err != nil {
		q.t.Fatal(err)
	}
	return domain.QRPhone{UserID: u.ID, Username: u.Username, NGACNodeID: u.NGACNodeID, TenantID: tenant, SessionID: sessionID, IP: "203.0.113.77"}
}

func (q qrWorld) create(t *testing.T) domain.QRTicket {
	t.Helper()
	ticket, err := q.svc.CreateQRLogin(context.Background(), chromeOnWindows, deskIP)
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
	if _, err := q.svc.ClaimQRLogin(ctx, ticket.Token, ticket.PollSecret); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Fatalf("a scan alone must not hand out a session: %v", err)
	}

	if err := q.svc.ApproveQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// Approved: the computer is told who and where, and asks its person to confirm.
	poll, err = q.svc.PollQRLogin(ctx, ticket.Token, ticket.PollSecret)
	if err != nil || poll.Status != domain.QRApproved || poll.Scanner == nil || poll.Scanner.TenantName != "NovaPay" {
		t.Fatalf("after approve: %+v, %v", poll, err)
	}
	// Polling again does not use the approval up.
	if again, err := q.svc.PollQRLogin(ctx, ticket.Token, ticket.PollSecret); err != nil || again.Status != domain.QRApproved {
		t.Fatalf("second poll after approve: %+v, %v", again, err)
	}
	s, err := q.svc.ClaimQRLogin(ctx, ticket.Token, ticket.PollSecret)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if s.UserID != q.phone.ID || s.TenantID != q.tenant || s.SessionID == "" {
		t.Errorf("session %+v", s)
	}
	if got := tenantOf(t, s.Token); got != q.tenant {
		t.Errorf("token tenant = %q, want the phone's %q", got, q.tenant)
	}

	// Single use: the session is handed out once, and the code is gone.
	if _, err := q.svc.ClaimQRLogin(ctx, ticket.Token, ticket.PollSecret); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Fatalf("second claim: err = %v, want ErrQRUnavailable", err)
	}
	if _, err := q.svc.PollQRLogin(ctx, ticket.Token, ticket.PollSecret); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Fatalf("poll after claim: err = %v, want ErrQRUnavailable", err)
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
		if s, err := q.svc.ClaimQRLogin(ctx, ticket.Token, secret); !errors.Is(err, domain.ErrQRUnavailable) {
			t.Errorf("%s claim secret: got %+v, %v; want ErrQRUnavailable", name, s, err)
		}
		if err := q.svc.CancelQRLogin(ctx, ticket.Token, secret); !errors.Is(err, domain.ErrQRUnavailable) {
			t.Errorf("%s cancel secret: %v; want ErrQRUnavailable", name, err)
		}
	}
	// The rightful browser can still claim it afterwards.
	if s, err := q.svc.ClaimQRLogin(ctx, ticket.Token, ticket.PollSecret); err != nil || s == nil {
		t.Fatalf("rightful claim: %+v, %v", s, err)
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
	if _, err := q.svc.ClaimQRLogin(ctx, ticket.Token, ticket.PollSecret); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Fatalf("a refused approval must not leave a session to claim: %v", err)
	}

	// A phone with no tenant at all.
	ticket = q.create(t)
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, "")); !errors.Is(err, domain.ErrAccessDenied) {
		t.Errorf("scan with no tenant: err = %v, want ErrAccessDenied", err)
	}
}

func TestQRLogin_AnEndedPhoneSessionCannotHandOutANewOne(t *testing.T) {
	q := newQRWorld(t)
	ctx := context.Background()

	// Signed out on the phone: the access token still verifies, the session does not.
	ticket := q.create(t)
	phone := q.phoneOf(q.phone, q.tenant)
	if err := q.svc.EndSession(ctx, phone.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, phone); !errors.Is(err, domain.ErrSessionEnded) {
		t.Errorf("scan from an ended session: err = %v, want ErrSessionEnded", err)
	}

	// Ended between scan and approve.
	phone = q.phoneOf(q.phone, q.tenant)
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, phone); err != nil {
		t.Fatal(err)
	}
	if err := q.svc.EndSession(ctx, phone.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := q.svc.ApproveQRLogin(ctx, ticket.Token, phone); !errors.Is(err, domain.ErrSessionEnded) {
		t.Fatalf("approve from an ended session: err = %v, want ErrSessionEnded", err)
	}
	if _, err := q.svc.ClaimQRLogin(ctx, ticket.Token, ticket.PollSecret); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Fatalf("an ended session must not leave a session to claim: %v", err)
	}

	// Every session of the person revoked (an account takeover being cleaned up).
	ticket = q.create(t)
	phone = q.phoneOf(q.phone, q.tenant)
	if err := q.svc.RevokeAllSessions(ctx, q.phone.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, phone); !errors.Is(err, domain.ErrSessionEnded) {
		t.Errorf("scan after a user-wide revocation: err = %v, want ErrSessionEnded", err)
	}

	// A token with no session at all.
	phone = q.phoneOf(q.phone, q.tenant)
	phone.SessionID = ""
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, phone); !errors.Is(err, domain.ErrSessionEnded) {
		t.Errorf("scan with no session: err = %v, want ErrSessionEnded", err)
	}
}

func TestQRLogin_TheTenantIsTheOneScannedFrom(t *testing.T) {
	q := newQRWorld(t)
	ctx := context.Background()
	other := q.w.addTenant("Other", "")
	if err := q.w.InsertTenantUser(ctx, other, q.phone.ID, "member", "active", q.phone.NGACNodeID); err != nil {
		t.Fatal(err)
	}
	stranger := q.w.addTenant("Stranger", "")

	ticket := q.create(t)
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); err != nil {
		t.Fatal(err)
	}
	// Switched workspace on the phone after scanning: approval is refused.
	if err := q.svc.ApproveQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, other)); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Errorf("approve from another tenant: err = %v, want ErrQRUnavailable", err)
	}
	// Re-scanning from a tenant the person is not a member of is refused too.
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, stranger)); !errors.Is(err, domain.ErrAccessDenied) {
		t.Errorf("re-scan into a non-member tenant: err = %v, want ErrAccessDenied", err)
	}
	if p, _ := q.svc.PollQRLogin(ctx, ticket.Token, ticket.PollSecret); p == nil || p.Status != domain.QRScanned {
		t.Errorf("the code must still wait for its first scan's tenant, got %+v", p)
	}
}

func TestQRLogin_ScanningDoesNotExtendTheCode(t *testing.T) {
	q := newQRWorld(t)
	ctx := context.Background()
	ticket := q.create(t)
	time.Sleep(1100 * time.Millisecond)
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); err != nil {
		t.Fatal(err)
	}
	var left time.Duration
	keys, err := q.rdb.Keys(ctx, "qr_login:*").Result()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		if ttl := q.rdb.TTL(ctx, k).Val(); ttl > left {
			left = ttl
		}
	}
	if left <= 0 || left >= domain.QRLoginTTL {
		t.Errorf("a scanned code has %v left; it must keep its original expiry (< %v)", left, domain.QRLoginTTL)
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
	if err != nil || p.Status != domain.QRDeclined {
		t.Fatalf("after decline: %+v, %v", p, err)
	}
	if err := q.svc.ApproveQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Errorf("approve after decline: err = %v, want ErrQRUnavailable", err)
	}
}

func TestQRLogin_NotMeOnTheComputerEndsTheCode(t *testing.T) {
	q := newQRWorld(t)
	ctx := context.Background()
	ticket := q.create(t)
	if _, err := q.svc.ScanQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); err != nil {
		t.Fatal(err)
	}
	if err := q.svc.ApproveQRLogin(ctx, ticket.Token, q.phoneOf(q.phone, q.tenant)); err != nil {
		t.Fatal(err)
	}
	if err := q.svc.CancelQRLogin(ctx, ticket.Token, ticket.PollSecret); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := q.svc.ClaimQRLogin(ctx, ticket.Token, ticket.PollSecret); !errors.Is(err, domain.ErrQRUnavailable) {
		t.Fatalf("claim after Không phải tôi: %v, want ErrQRUnavailable", err)
	}
}

func TestQRLogin_ThePhoneIsToldWhenTheComputerIsOnAnotherNetwork(t *testing.T) {
	q := newQRWorld(t)
	ctx := context.Background()
	cases := []struct {
		name, desk, phone string
		same              bool
	}{
		{"same address", "203.0.113.10", "203.0.113.10", true},
		{"same IPv4 /24", "203.0.113.10", "203.0.113.200", true},
		{"other IPv4 network", "203.0.113.10", "198.51.100.7", false},
		{"same IPv6 /64", "2001:db8:1:2::10", "2001:db8:1:2:abcd::1", true},
		{"other IPv6 /64", "2001:db8:1:2::10", "2001:db8:1:3::1", false},
		{"IPv4 against IPv6", "203.0.113.10", "2001:db8::1", false},
		{"unknown computer address", "", "203.0.113.10", false},
	}
	for _, c := range cases {
		ticket, err := q.svc.CreateQRLogin(ctx, chromeOnWindows, c.desk)
		if err != nil {
			t.Fatal(err)
		}
		phone := q.phoneOf(q.phone, q.tenant)
		phone.IP = c.phone
		scan, err := q.svc.ScanQRLogin(ctx, ticket.Token, phone)
		if err != nil {
			t.Fatalf("%s: scan: %v", c.name, err)
		}
		if scan.SameNetwork != c.same {
			t.Errorf("%s: same network = %v, want %v", c.name, scan.SameNetwork, c.same)
		}
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
		if _, err := q.svc.ClaimQRLogin(ctx, tok, "x"); !errors.Is(err, domain.ErrQRUnavailable) {
			t.Errorf("claim %q: %v", tok, err)
		}
	}
}

func TestQRLogin_WithoutRedisIsUnavailableNotAPanic(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t) // no Redis
	if _, err := svc.CreateQRLogin(context.Background(), chromeOnWindows, deskIP); !errors.Is(err, domain.ErrQRUnavailable) {
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
