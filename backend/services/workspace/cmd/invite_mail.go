package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"ngac-platform/pkg/bootstrap"
	"ngac-platform/pkg/mailer"
	"ngac-platform/services/workspace/internal/domain"
)

// configureInviteMail turns on the invitation email when SMTP is configured
// (SMTP_*, shared with the auth service) and APP_BASE_URL names the web app the
// link points to. Without SMTP, invitations are recorded and nothing is sent.
// A half-configured SMTP is an error: it must not silently send nothing.
func configureInviteMail(svc *domain.Service) error {
	m, err := mailer.FromEnv()
	if err != nil {
		return err
	}
	if m == nil {
		slog.Info("invitation emails are off: SMTP_HOST is not set")
		return nil
	}
	appURL, err := appBaseURL(bootstrap.Env("APP_BASE_URL", "http://localhost:5173"))
	if err != nil {
		return err
	}
	svc.WithInviteMail(m, appURL)
	slog.Info("invitation emails are sent over SMTP")
	return nil
}

// appBaseURL checks that the link put in an email is an absolute http(s) URL.
func appBaseURL(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("APP_BASE_URL must be an absolute http(s) URL, got %q", raw)
	}
	return raw, nil
}

// drainInviteMail gives the invitation emails still being sent a moment to finish.
func drainInviteMail(svc *domain.Service) { svc.DrainMail(10 * time.Second) }
