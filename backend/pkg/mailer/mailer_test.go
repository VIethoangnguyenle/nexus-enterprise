package mailer_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"

	"ngac-platform/pkg/mailer"
	"ngac-platform/testutil"
)

func senderFor(t *testing.T, f *testutil.FakeSMTP, tlsMode string, mutate func(*mailer.Config)) *mailer.Sender {
	t.Helper()
	cfg := mailer.Config{
		Host: f.Host, Port: f.Port, Username: "me@gmail.test", Password: "app-password-1234",
		From: "Nexus <me@gmail.test>", TLS: tlsMode,
		TLSConfig: &tls.Config{RootCAs: f.Pool},
	}
	if mutate != nil {
		mutate(&cfg)
	}
	s, err := mailer.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

var sample = mailer.Message{
	To:      "owner@example.test",
	Subject: "Xin chào Nexus",
	Text:    "Nội dung văn bản 314159\n",
	HTML:    "<!DOCTYPE html><html><body><p>Nội dung HTML 314159</p></body></html>\n",
}

func TestSend_WellFormedMessage_STARTTLS(t *testing.T) {
	f := testutil.NewFakeSMTP(t, false, nil)
	if err := senderFor(t, f, "starttls", nil).Send(context.Background(), sample); err != nil {
		t.Fatalf("send: %v", err)
	}
	assertMessage(t, f)
}

func TestSend_WellFormedMessage_ImplicitTLS(t *testing.T) {
	f := testutil.NewFakeSMTP(t, true, nil)
	if err := senderFor(t, f, "tls", nil).Send(context.Background(), sample); err != nil {
		t.Fatalf("send: %v", err)
	}
	assertMessage(t, f)
}

func assertMessage(t *testing.T, f *testutil.FakeSMTP) {
	t.Helper()
	msgs := f.Messages()
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1", len(msgs))
	}
	raw := msgs[0]

	if got := f.Auths(); len(got) != 1 || got[0] != "\x00me@gmail.test\x00app-password-1234" {
		t.Errorf("auth = %q", got)
	}
	if got := f.Froms(); len(got) != 1 || !strings.Contains(got[0], "<me@gmail.test>") {
		t.Errorf("envelope from = %v", got)
	}
	if got := f.Rcpts(); len(got) != 1 || !strings.Contains(got[0], "<owner@example.test>") {
		t.Errorf("envelope rcpt = %v", got)
	}
	if strings.Contains(strings.ReplaceAll(raw, "\r\n", ""), "\n") {
		t.Error("message contains a bare LF")
	}

	m, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if err != nil || subject != sample.Subject {
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
		kinds = append(kinds, p.Header.Get("Content-Type"))
		if !strings.Contains(string(body), "314159") {
			t.Errorf("%s part lacks the body:\n%s", p.Header.Get("Content-Type"), body)
		}
	}
	if len(kinds) != 2 || !strings.HasPrefix(kinds[0], "text/plain") || !strings.HasPrefix(kinds[1], "text/html") {
		t.Errorf("parts = %v, want plain then html", kinds)
	}
}

func TestSend_NeverLogsBodyOrPassword(t *testing.T) {
	logs := captureLogs(t)
	bad := testutil.NewFakeSMTP(t, false, func(f *testutil.FakeSMTP) { f.RejectData = true })
	err := senderFor(t, bad, "starttls", nil).Send(context.Background(), sample)
	if err == nil {
		t.Fatal("rejected message must fail")
	}
	for _, leak := range []string{"314159", "app-password-1234"} {
		if strings.Contains(logs.String(), leak) || strings.Contains(err.Error(), leak) {
			t.Errorf("%q leaked:\nlogs: %s\nerr: %v", leak, logs.String(), err)
		}
	}
}

func TestSend_RefusesHeaderInjectionInTheRecipient(t *testing.T) {
	f := testutil.NewFakeSMTP(t, false, nil)
	s := senderFor(t, f, "starttls", nil)
	for _, rcpt := range []string{
		"a@b.test\r\nBcc: evil@x.test",
		"a@b.test\nBcc: evil@x.test",
		"a@b.test>\r\nRCPT TO:<evil@x.test",
		"a@b.test, evil@x.test",
		"Evil <a@b.test>",
		"a b@c.test",
		"",
	} {
		m := sample
		m.To = rcpt
		if err := s.Send(context.Background(), m); err == nil {
			t.Errorf("%q: must be refused", rcpt)
		}
	}
	if conns := f.Conns(); conns != 0 {
		t.Errorf("a refused recipient must not even open a connection, got %d", conns)
	}
}

func TestSend_SubjectCannotAddHeaders(t *testing.T) {
	f := testutil.NewFakeSMTP(t, false, nil)
	m := sample
	m.Subject = "Lan mời bạn\r\nBcc: evil@x.test\nX-Evil: 1 vào Công ty"
	if err := senderFor(t, f, "starttls", nil).Send(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	raw := f.Messages()[0]
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"Bcc", "X-Evil"} {
		if msg.Header.Get(h) != "" {
			t.Errorf("header %s was injected through the subject", h)
		}
	}
	head, _, _ := strings.Cut(raw, "\r\n\r\n")
	if strings.Contains(head, "\nBcc:") || strings.Contains(head, "\nX-Evil:") {
		t.Errorf("injected header line in:\n%s", head)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || !strings.Contains(subject, "Công ty") || strings.ContainsAny(subject, "\r\n") {
		t.Errorf("subject = %q (%v)", subject, err)
	}
}

func TestNew_FromWithLineBreakIsRefused(t *testing.T) {
	_, err := mailer.New(mailer.Config{
		Host: "smtp.example.test", Username: "u", Password: "p",
		From: "Nexus <me@gmail.test>\r\nBcc: evil@x.test",
	})
	if err == nil {
		t.Fatal("a line break in From must be refused")
	}
}

func TestSend_TimeoutAndFailureSurfaceAsErrors(t *testing.T) {
	t.Run("server that never answers", func(t *testing.T) {
		f := testutil.NewFakeSMTP(t, false, func(f *testutil.FakeSMTP) { f.Hang = true })
		s := senderFor(t, f, "starttls", func(c *mailer.Config) { c.SendTimeout = 300 * time.Millisecond })
		start := time.Now()
		if err := s.Send(context.Background(), sample); err == nil {
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
		s, err := mailer.New(mailer.Config{Host: host, Port: port, Username: "u", Password: "p", From: "me@gmail.test"})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Send(context.Background(), sample); err == nil {
			t.Fatal("connection refused must be an error")
		}
	})
	t.Run("server without STARTTLS never sees credentials", func(t *testing.T) {
		f := testutil.NewFakeSMTP(t, false, func(f *testutil.FakeSMTP) { f.NoSTARTTLS = true })
		if err := senderFor(t, f, "starttls", nil).Send(context.Background(), sample); err == nil {
			t.Fatal("must refuse to downgrade")
		}
		if len(f.Auths()) != 0 {
			t.Error("credentials were sent without TLS")
		}
	})
	t.Run("untrusted certificate", func(t *testing.T) {
		f := testutil.NewFakeSMTP(t, false, nil)
		s := senderFor(t, f, "starttls", func(c *mailer.Config) { c.TLSConfig = nil })
		if err := s.Send(context.Background(), sample); err == nil {
			t.Fatal("an unverifiable certificate must fail")
		}
	})
}

func TestNew_Validation(t *testing.T) {
	ok := mailer.Config{Host: "smtp.gmail.com", Port: "587", Username: "u@gmail.com", Password: "p", From: "Nexus <u@gmail.com>"}
	if _, err := mailer.New(ok); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	for name, mutate := range map[string]func(*mailer.Config){
		"no host":      func(c *mailer.Config) { c.Host = "" },
		"bad port":     func(c *mailer.Config) { c.Port = "mail" },
		"no password":  func(c *mailer.Config) { c.Password = "" },
		"no username":  func(c *mailer.Config) { c.Username = "" },
		"bad from":     func(c *mailer.Config) { c.From = "not an address" },
		"empty from":   func(c *mailer.Config) { c.From = "" },
		"unknown mode": func(c *mailer.Config) { c.TLS = "plain" },
	} {
		c := ok
		mutate(&c)
		if _, err := mailer.New(c); err == nil {
			t.Errorf("%s: must be refused", name)
		}
	}
}

func setSMTPEnv(t *testing.T, host, pass string) {
	t.Helper()
	t.Setenv("SMTP_HOST", host)
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USERNAME", "me@gmail.com")
	t.Setenv("SMTP_PASSWORD", pass)
	t.Setenv("SMTP_FROM", "Nexus <me@gmail.com>")
	t.Setenv("SMTP_TLS", "")
}

func TestFromEnv(t *testing.T) {
	setSMTPEnv(t, "smtp.gmail.com", "abcd efgh ijkl mnop")
	s, err := mailer.FromEnv()
	if err != nil || s == nil {
		t.Fatalf("complete config: sender=%v err=%v", s, err)
	}

	setSMTPEnv(t, "smtp.gmail.com", "")
	if s, err := mailer.FromEnv(); err == nil || s != nil {
		t.Fatalf("a partial config must be refused, got sender=%v err=%v", s, err)
	}
	setSMTPEnv(t, "smtp.gmail.com", "abcd efgh ijkl mnop")
	t.Setenv("SMTP_FROM", "")
	if _, err := mailer.FromEnv(); err == nil || strings.Contains(err.Error(), "abcd") {
		t.Fatalf("a missing from must be refused without echoing secrets, got %v", err)
	}

	setSMTPEnv(t, "", "x")
	if s, err := mailer.FromEnv(); s != nil || err != nil {
		t.Fatalf("no SMTP_HOST means not configured, got sender=%v err=%v", s, err)
	}
}

func TestMaskEmail(t *testing.T) {
	for in, want := range map[string]string{
		"moi@novapay.vn": "mo***@novapay.vn",
		"ab@novapay.vn":  "***",
		"no-at-sign":     "***",
		"":               "***",
	} {
		if got := mailer.MaskEmail(in); got != want {
			t.Errorf("MaskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}
