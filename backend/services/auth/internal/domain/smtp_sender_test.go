package domain_test

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"io"
	"math/big"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"sync"
	"testing"
	"time"

	"ngac-platform/services/auth/internal/domain"
)

// fakeSMTP is a minimal in-process SMTP server: enough of the dialogue for
// net/smtp (EHLO, STARTTLS, AUTH PLAIN, MAIL, RCPT, DATA, QUIT).
type fakeSMTP struct {
	addr     string
	host     string
	port     string
	pool     *x509.CertPool
	implicit bool

	// Behaviour switches, set before the first connection.
	hang       bool // accept, send nothing
	rejectData bool // 554 after DATA
	noSTARTTLS bool

	mu       sync.Mutex
	conns    int
	auths    []string // decoded AUTH PLAIN payloads
	froms    []string
	rcpts    []string
	messages []string
}

func newFakeSMTP(t *testing.T, implicit bool, mutate func(*fakeSMTP)) *fakeSMTP {
	t.Helper()
	cert, pool := selfSignedCert(t)
	f := &fakeSMTP{pool: pool, implicit: implicit}
	if mutate != nil {
		mutate(f)
	}
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if implicit {
		ln = tls.NewListener(ln, tlsCfg)
	}
	f.addr = ln.Addr().String()
	f.host, f.port, _ = net.SplitHostPort(f.addr)
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c, tlsCfg)
		}
	}()
	return f
}

func selfSignedCert(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "fake smtp"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}

func (f *fakeSMTP) serve(c net.Conn, tlsCfg *tls.Config) {
	f.mu.Lock()
	f.conns++
	f.mu.Unlock()
	defer func() { c.Close() }()
	if f.hang {
		time.Sleep(5 * time.Second)
		return
	}
	_ = c.SetDeadline(time.Now().Add(10 * time.Second))
	r := bufio.NewReader(c)
	say := func(s string) { _, _ = io.WriteString(c, s+"\r\n") }
	secure := f.implicit
	say("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		up := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(up, "EHLO"), strings.HasPrefix(up, "HELO"):
			say("250-fake")
			if !secure && !f.noSTARTTLS {
				say("250-STARTTLS")
			}
			say("250-AUTH PLAIN")
			say("250 8BITMIME")
		case up == "STARTTLS":
			say("220 go ahead")
			tc := tls.Server(c, tlsCfg)
			if err := tc.Handshake(); err != nil {
				return
			}
			c = tc
			r = bufio.NewReader(tc)
			secure = true
		case strings.HasPrefix(up, "AUTH PLAIN"):
			raw, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(line[len("AUTH PLAIN"):]))
			f.mu.Lock()
			f.auths = append(f.auths, string(raw))
			f.mu.Unlock()
			say("235 ok")
		case strings.HasPrefix(up, "MAIL FROM:"):
			f.mu.Lock()
			f.froms = append(f.froms, line[len("MAIL FROM:"):])
			f.mu.Unlock()
			say("250 ok")
		case strings.HasPrefix(up, "RCPT TO:"):
			f.mu.Lock()
			f.rcpts = append(f.rcpts, line[len("RCPT TO:"):])
			f.mu.Unlock()
			say("250 ok")
		case up == "DATA":
			say("354 go")
			var sb strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				sb.WriteString(strings.TrimPrefix(l, "."))
			}
			if f.rejectData {
				say("554 rejected")
				continue
			}
			f.mu.Lock()
			f.messages = append(f.messages, sb.String())
			f.mu.Unlock()
			say("250 queued")
		case up == "QUIT":
			say("221 bye")
			return
		default:
			say("502 unsupported")
		}
	}
}

func (f *fakeSMTP) sender(t *testing.T, tlsMode string, mutate func(*domain.SMTPConfig)) *domain.SMTPSender {
	t.Helper()
	cfg := domain.SMTPConfig{
		Host: f.host, Port: f.port, Username: "me@gmail.test", Password: "app-password-1234",
		From: "Nexus <me@gmail.test>", TLS: tlsMode,
		TLSConfig: &tls.Config{RootCAs: f.pool},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	s, err := domain.NewSMTPSender(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (f *fakeSMTP) snapshot() (conns int, messages []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.conns, append([]string(nil), f.messages...)
}

func TestSMTPSender_SendsAWellFormedMessage_STARTTLS(t *testing.T) {
	f := newFakeSMTP(t, false, nil)
	s := f.sender(t, "starttls", nil)
	if err := s.SendCode(context.Background(), "owner@example.test", "email", "314159"); err != nil {
		t.Fatalf("send: %v", err)
	}
	assertMessage(t, f, "314159")
}

func TestSMTPSender_SendsAWellFormedMessage_ImplicitTLS(t *testing.T) {
	f := newFakeSMTP(t, true, nil)
	s := f.sender(t, "tls", nil)
	if err := s.SendCode(context.Background(), "owner@example.test", "email", "271828"); err != nil {
		t.Fatalf("send: %v", err)
	}
	assertMessage(t, f, "271828")
}

func assertMessage(t *testing.T, f *fakeSMTP, code string) {
	t.Helper()
	_, msgs := f.snapshot()
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1", len(msgs))
	}
	raw := msgs[0]

	if got := f.auths; len(got) != 1 || got[0] != "\x00me@gmail.test\x00app-password-1234" {
		t.Errorf("auth = %q", got)
	}
	if len(f.froms) != 1 || !strings.Contains(f.froms[0], "<me@gmail.test>") {
		t.Errorf("envelope from = %v", f.froms)
	}
	if len(f.rcpts) != 1 || !strings.Contains(f.rcpts[0], "<owner@example.test>") {
		t.Errorf("envelope rcpt = %v", f.rcpts)
	}

	// CRLF everywhere: no bare LF.
	if strings.Contains(strings.ReplaceAll(raw, "\r\n", ""), "\n") {
		t.Error("message contains a bare LF")
	}

	m, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	dec := new(mime.WordDecoder)
	subject, err := dec.DecodeHeader(m.Header.Get("Subject"))
	if err != nil || subject != "Mã đăng nhập Nexus: "+code {
		t.Errorf("subject = %q (%v)", subject, err)
	}
	from, err := m.Header.AddressList("From")
	if err != nil || len(from) != 1 || from[0].Address != "me@gmail.test" || from[0].Name != "Nexus" {
		t.Errorf("from = %v (%v)", from, err)
	}
	if to := m.Header.Get("To"); to != "owner@example.test" {
		t.Errorf("to = %q", to)
	}
	if _, err := m.Header.Date(); err != nil {
		t.Errorf("date: %v", err)
	}
	if id := m.Header.Get("Message-ID"); !strings.HasPrefix(id, "<") || !strings.HasSuffix(id, "@gmail.test>") {
		t.Errorf("message-id = %q", id)
	}
	if m.Header.Get("MIME-Version") != "1.0" {
		t.Error("missing MIME-Version")
	}
	mt, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/alternative" {
		t.Fatalf("content-type = %q (%v)", mt, err)
	}

	mr := multipart.NewReader(m.Body, params["boundary"])
	var kinds []string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(quotedprintable.NewReader(p))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		kinds = append(kinds, p.Header.Get("Content-Type"))
		for _, want := range []string{code, "5 phút", "bỏ qua email này"} {
			if !strings.Contains(text, want) {
				t.Errorf("%s part lacks %q:\n%s", p.Header.Get("Content-Type"), want, text)
			}
		}
		if strings.HasPrefix(p.Header.Get("Content-Type"), "text/html") && !strings.Contains(text, "<html") {
			t.Error("html part is not html")
		}
	}
	if len(kinds) != 2 || !strings.HasPrefix(kinds[0], "text/plain") || !strings.HasPrefix(kinds[1], "text/html") {
		t.Errorf("parts = %v, want plain then html", kinds)
	}
}

func TestSMTPSender_NeverLogsTheCodeOrPassword(t *testing.T) {
	f := newFakeSMTP(t, false, nil)
	s := f.sender(t, "starttls", nil)
	logs := captureLogs(t)
	if err := s.SendCode(context.Background(), "owner@example.test", "email", "601733"); err != nil {
		t.Fatal(err)
	}
	// A failing send logs through the caller; its error text must be clean too.
	bad := newFakeSMTP(t, false, func(f *fakeSMTP) { f.rejectData = true })
	err := bad.sender(t, "starttls", nil).SendCode(context.Background(), "owner@example.test", "email", "601733")
	if err == nil {
		t.Fatal("rejected message must fail")
	}
	for _, leak := range []string{"601733", "app-password-1234"} {
		if strings.Contains(logs.String(), leak) || strings.Contains(err.Error(), leak) {
			t.Errorf("%q leaked:\nlogs: %s\nerr: %v", leak, logs.String(), err)
		}
	}
}

func TestSMTPSender_RefusesHeaderInjectionInTheRecipient(t *testing.T) {
	f := newFakeSMTP(t, false, nil)
	s := f.sender(t, "starttls", nil)
	for _, rcpt := range []string{
		"a@b.test\r\nBcc: evil@x.test",
		"a@b.test\nBcc: evil@x.test",
		"a@b.test>\r\nRCPT TO:<evil@x.test",
		"a@b.test, evil@x.test",
		"Evil <a@b.test>",
		"a b@c.test",
		"",
	} {
		if err := s.SendCode(context.Background(), rcpt, "email", "123456"); err == nil {
			t.Errorf("%q: must be refused", rcpt)
		}
	}
	if conns, _ := f.snapshot(); conns != 0 {
		t.Errorf("a refused recipient must not even open a connection, got %d", conns)
	}
	if err := s.SendCode(context.Background(), "0912345678", "phone", "123456"); err == nil {
		t.Error("this sender delivers email only")
	}
}

func TestSMTPSender_FromWithLineBreakIsRefusedAtConfig(t *testing.T) {
	_, err := domain.NewSMTPSender(domain.SMTPConfig{
		Host: "smtp.example.test", Username: "u", Password: "p",
		From: "Nexus <me@gmail.test>\r\nBcc: evil@x.test",
	})
	if err == nil {
		t.Fatal("a line break in From must be refused")
	}
}

func TestSMTPSender_TimeoutAndFailureSurfaceAsErrors(t *testing.T) {
	t.Run("server that never answers", func(t *testing.T) {
		f := newFakeSMTP(t, false, func(f *fakeSMTP) { f.hang = true })
		s := f.sender(t, "starttls", func(c *domain.SMTPConfig) { c.SendTimeout = 300 * time.Millisecond })
		start := time.Now()
		if err := s.SendCode(context.Background(), "owner@example.test", "email", "123456"); err == nil {
			t.Fatal("must time out")
		}
		if d := time.Since(start); d > 3*time.Second {
			t.Errorf("took %v, the send timeout was not enforced", d)
		}
	})
	t.Run("nothing listening", func(t *testing.T) {
		ln, _ := net.Listen("tcp", "127.0.0.1:0")
		host, port, _ := net.SplitHostPort(ln.Addr().String())
		ln.Close()
		s, err := domain.NewSMTPSender(domain.SMTPConfig{Host: host, Port: port, Username: "u", Password: "p", From: "me@gmail.test"})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SendCode(context.Background(), "owner@example.test", "email", "123456"); err == nil {
			t.Fatal("connection refused must be an error")
		}
	})
	t.Run("server without STARTTLS never sees credentials", func(t *testing.T) {
		f := newFakeSMTP(t, false, func(f *fakeSMTP) { f.noSTARTTLS = true })
		if err := f.sender(t, "starttls", nil).SendCode(context.Background(), "owner@example.test", "email", "123456"); err == nil {
			t.Fatal("must refuse to downgrade")
		}
		if len(f.auths) != 0 {
			t.Error("credentials were sent without TLS")
		}
	})
	t.Run("untrusted certificate", func(t *testing.T) {
		f := newFakeSMTP(t, false, nil)
		s := f.sender(t, "starttls", func(c *domain.SMTPConfig) { c.TLSConfig = nil })
		if err := s.SendCode(context.Background(), "owner@example.test", "email", "123456"); err == nil {
			t.Fatal("an unverifiable certificate must fail")
		}
	})
}

func TestSMTPSender_FailedSendLeavesNoUsableSession(t *testing.T) {
	f := newFakeSMTP(t, false, func(f *fakeSMTP) { f.rejectData = true })
	svc, _, rdb := otpService(t, &domain.OTPOptions{Sender: f.sender(t, "starttls", nil)})
	addr := uniqueEmail()
	if _, err := svc.RequestOTP(context.Background(), addr, "email"); err == nil {
		t.Fatal("a failed delivery must fail the request")
	}
	// The failure is the same whether or not the address has an account, and no
	// session key was left behind.
	keys, err := rdb.Keys(context.Background(), "otp:*").Result()
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range keys {
		if strings.HasPrefix(k, "otp:attempts") || strings.HasPrefix(k, "otp:rate") {
			continue
		}
		if v, _ := rdb.Get(context.Background(), k).Result(); strings.Contains(v, addr) {
			t.Errorf("session %s survived a failed delivery", k)
		}
	}
}

func TestSMTPSender_DeliveredCodeProvesTheAddress_FixedCodeStillDoesNot(t *testing.T) {
	f := newFakeSMTP(t, false, nil)
	sender := f.sender(t, "starttls", nil)

	svc, _, _ := otpService(t, &domain.OTPOptions{Sender: sender})
	if !svc.OTPProvesOwnership() {
		t.Fatal("an SMTP-delivered code proves the address")
	}

	// With the fixed code on, even a configured SMTP sender proves nothing.
	svc, w, _ := otpService(t, &domain.OTPOptions{FixedCode: "999999", Sender: sender})
	if svc.OTPProvesOwnership() {
		t.Fatal("fixed-code mode must never prove the address")
	}
	addr := uniqueEmail()
	existing := w.addUser(addr)
	sid, err := svc.RequestOTP(context.Background(), addr, "email")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.VerifyOTP(context.Background(), sid, "999999"); err != nil {
		t.Fatal(err)
	}
	if w.emailVerified(existing.ID) {
		t.Error("the fixed code must not verify an address")
	}
	if conns, _ := f.snapshot(); conns != 0 {
		t.Error("fixed-code mode sends no email")
	}
}

func TestNewSMTPSender_Validation(t *testing.T) {
	ok := domain.SMTPConfig{Host: "smtp.gmail.com", Port: "587", Username: "u@gmail.com", Password: "p", From: "Nexus <u@gmail.com>"}
	if _, err := domain.NewSMTPSender(ok); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	for name, mutate := range map[string]func(*domain.SMTPConfig){
		"no host":      func(c *domain.SMTPConfig) { c.Host = "" },
		"bad port":     func(c *domain.SMTPConfig) { c.Port = "mail" },
		"no password":  func(c *domain.SMTPConfig) { c.Password = "" },
		"no username":  func(c *domain.SMTPConfig) { c.Username = "" },
		"bad from":     func(c *domain.SMTPConfig) { c.From = "not an address" },
		"empty from":   func(c *domain.SMTPConfig) { c.From = "" },
		"unknown mode": func(c *domain.SMTPConfig) { c.TLS = "plain" },
	} {
		c := ok
		mutate(&c)
		if _, err := domain.NewSMTPSender(c); err == nil {
			t.Errorf("%s: must be refused", name)
		}
	}
}
