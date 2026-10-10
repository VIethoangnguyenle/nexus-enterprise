package main

import (
	"strings"
	"testing"

	"ngac-platform/services/workspace/internal/domain"
)

func TestConfigureInviteMail(t *testing.T) {
	svc := domain.NewService(nil, nil, nil, nil, nil, nil)
	setSMTP := func(host, password string) {
		t.Setenv("SMTP_HOST", host)
		t.Setenv("SMTP_PORT", "587")
		t.Setenv("SMTP_USERNAME", "me@gmail.com")
		t.Setenv("SMTP_PASSWORD", password)
		t.Setenv("SMTP_FROM", "Nexus <me@gmail.com>")
		t.Setenv("SMTP_TLS", "")
	}
	t.Setenv("APP_BASE_URL", "https://nexus.example.test/")

	setSMTP("", "")
	if err := configureInviteMail(svc); err != nil {
		t.Fatalf("no SMTP is a valid dev setup, got %v", err)
	}
	setSMTP("smtp.gmail.com", "abcd efgh ijkl mnop")
	if err := configureInviteMail(svc); err != nil {
		t.Fatalf("complete SMTP: %v", err)
	}
	setSMTP("smtp.gmail.com", "")
	if err := configureInviteMail(svc); err == nil {
		t.Fatal("a half-configured SMTP must refuse to start")
	}
	setSMTP("smtp.gmail.com", "abcd efgh ijkl mnop")
	t.Setenv("APP_BASE_URL", "nexus.example.test")
	err := configureInviteMail(svc)
	if err == nil || strings.Contains(err.Error(), "abcd") {
		t.Fatalf("a link that is not an absolute URL must be refused, got %v", err)
	}
}

func TestAppBaseURL(t *testing.T) {
	if got, err := appBaseURL(" https://nexus.example.test/ "); err != nil || got != "https://nexus.example.test" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, bad := range []string{"", "nexus.test", "javascript:alert(1)", "ftp://x.test", "https://"} {
		if _, err := appBaseURL(bad); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
}
