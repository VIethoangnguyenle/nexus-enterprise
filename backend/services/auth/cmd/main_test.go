package main

import (
	"os"
	"strings"
	"testing"
)

func TestOtpOptions_SMTPSender(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("AUTH_DEV_OTP", "")
	t.Setenv("AUTH_FIXED_OTP_CODE", "")
	t.Setenv("SMTP_HOST", "smtp.gmail.com")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USERNAME", "me@gmail.com")
	t.Setenv("SMTP_PASSWORD", "abcd efgh ijkl mnop")
	t.Setenv("SMTP_FROM", "Nexus <me@gmail.com>")
	t.Setenv("SMTP_TLS", "")

	opts, err := otpOptions("secret")
	if err != nil {
		t.Fatal(err)
	}
	if opts.Sender == nil || opts.FixedCode != "" {
		t.Fatalf("SMTP configured: sender=%v fixed=%q", opts.Sender, opts.FixedCode)
	}

	t.Setenv("SMTP_PASSWORD", "")
	if _, err := otpOptions("secret"); err == nil || strings.Contains(err.Error(), "abcd") {
		t.Fatalf("half-configured SMTP must refuse to start without echoing secrets, got %v", err)
	}

	t.Setenv("SMTP_HOST", "")
	opts, err = otpOptions("secret")
	if err != nil || opts.Sender != nil {
		t.Fatalf("no SMTP and no dev mode: sender=%v err=%v", opts.Sender, err)
	}
}

func TestOtpOptions_FixedCodeWinsOverSMTP(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("AUTH_FIXED_OTP_CODE", "123456")
	t.Setenv("SMTP_HOST", "smtp.gmail.com")
	opts, err := otpOptions("secret")
	if err != nil || opts.FixedCode != "123456" || opts.Sender != nil {
		t.Fatalf("fixed=%q sender=%v err=%v", opts.FixedCode, opts.Sender, err)
	}
}

// A fixed code is a master key to every account: outside development it is
// never the default. An operator may still set one on purpose (tester mode).
func TestOtpOptions_NoDefaultFixedCodeOutsideDevelopment(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	t.Setenv("AUTH_DEV_OTP", "")
	for _, env := range []string{"production", "staging", ""} {
		t.Setenv("APP_ENV", env)
		t.Setenv("AUTH_FIXED_OTP_CODE", "123456")
		if opts, err := otpOptions("secret"); err != nil || opts.FixedCode != "123456" {
			t.Errorf("APP_ENV=%q with a deliberately configured code: fixed=%q err=%v", env, opts.FixedCode, err)
		}
		os.Unsetenv("AUTH_FIXED_OTP_CODE")
		if opts, err := otpOptions("secret"); err == nil && opts.FixedCode != "" {
			t.Errorf("APP_ENV=%q with AUTH_FIXED_OTP_CODE unset fell back to the default fixed code", env)
		}
	}

	t.Setenv("APP_ENV", "dev")
	os.Unsetenv("AUTH_FIXED_OTP_CODE")
	if opts, err := otpOptions("secret"); err != nil || opts.FixedCode == "" {
		t.Fatalf("development keeps the default fixed code: fixed=%q err=%v", opts.FixedCode, err)
	}
}
