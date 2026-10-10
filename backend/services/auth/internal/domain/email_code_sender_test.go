package domain_test

import (
	"context"
	"crypto/tls"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"

	"ngac-platform/pkg/mailer"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/testutil"
)

func codeSender(t *testing.T, f *testutil.FakeSMTP) *domain.EmailCodeSender {
	t.Helper()
	m, err := mailer.New(mailer.Config{
		Host: f.Host, Port: f.Port, Username: "me@gmail.test", Password: "app-password-1234",
		From: "Nexus <me@gmail.test>", TLS: "starttls",
		TLSConfig: &tls.Config{RootCAs: f.Pool},
	})
	if err != nil {
		t.Fatal(err)
	}
	return domain.NewEmailCodeSender(m)
}

func TestEmailCodeSender_SendsTheCodeInVietnamese(t *testing.T) {
	f := testutil.NewFakeSMTP(t, false, nil)
	if err := codeSender(t, f).SendCode(context.Background(), "owner@example.test", "email", "314159"); err != nil {
		t.Fatalf("send: %v", err)
	}
	msgs := f.Messages()
	if len(msgs) != 1 {
		t.Fatalf("messages = %d, want 1", len(msgs))
	}
	m, err := mail.ReadMessage(strings.NewReader(msgs[0]))
	if err != nil {
		t.Fatal(err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(m.Header.Get("Subject"))
	if err != nil || subject != "Mã đăng nhập Nexus: 314159" {
		t.Errorf("subject = %q (%v)", subject, err)
	}
	_, params, _ := mime.ParseMediaType(m.Header.Get("Content-Type"))
	mr := multipart.NewReader(m.Body, params["boundary"])
	parts := 0
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(quotedprintable.NewReader(p))
		parts++
		for _, want := range []string{"314159", "5 phút", "bỏ qua email này"} {
			if !strings.Contains(string(body), want) {
				t.Errorf("%s part lacks %q:\n%s", p.Header.Get("Content-Type"), want, body)
			}
		}
	}
	if parts != 2 {
		t.Errorf("parts = %d, want 2", parts)
	}
}

func TestEmailCodeSender_DeliversEmailOnly(t *testing.T) {
	f := testutil.NewFakeSMTP(t, false, nil)
	if err := codeSender(t, f).SendCode(context.Background(), "0912345678", "phone", "123456"); err == nil {
		t.Error("this sender delivers email only")
	}
	if f.Conns() != 0 {
		t.Error("a refused identifier must not open a connection")
	}
}

func TestEmailCodeSender_NeverLogsTheCode(t *testing.T) {
	f := testutil.NewFakeSMTP(t, false, nil)
	logs := captureLogs(t)
	if err := codeSender(t, f).SendCode(context.Background(), "owner@example.test", "email", "601733"); err != nil {
		t.Fatal(err)
	}
	bad := testutil.NewFakeSMTP(t, false, func(f *testutil.FakeSMTP) { f.RejectData = true })
	err := codeSender(t, bad).SendCode(context.Background(), "owner@example.test", "email", "601733")
	if err == nil {
		t.Fatal("rejected message must fail")
	}
	for _, leak := range []string{"601733", "app-password-1234"} {
		if strings.Contains(logs.String(), leak) || strings.Contains(err.Error(), leak) {
			t.Errorf("%q leaked:\nlogs: %s\nerr: %v", leak, logs.String(), err)
		}
	}
}

func TestEmailCodeSender_FailedSendLeavesNoUsableSession(t *testing.T) {
	f := testutil.NewFakeSMTP(t, false, func(f *testutil.FakeSMTP) { f.RejectData = true })
	svc, _, rdb := otpService(t, &domain.OTPOptions{Sender: codeSender(t, f)})
	addr := uniqueEmail()
	if _, err := svc.RequestOTP(context.Background(), addr, "email"); err == nil {
		t.Fatal("a failed delivery must fail the request")
	}
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

func TestEmailCodeSender_DeliveredCodeProvesTheAddress_FixedCodeStillDoesNot(t *testing.T) {
	f := testutil.NewFakeSMTP(t, false, nil)
	sender := codeSender(t, f)

	svc, _, _ := otpService(t, &domain.OTPOptions{Sender: sender})
	if !svc.OTPProvesOwnership() {
		t.Fatal("an email-delivered code proves the address")
	}

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
	if f.Conns() != 0 {
		t.Error("fixed-code mode sends no email")
	}
}
