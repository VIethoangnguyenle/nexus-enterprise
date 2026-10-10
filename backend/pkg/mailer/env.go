package mailer

import (
	"fmt"
	"os"
	"strings"
)

// FromEnv builds a sender from SMTP_HOST, SMTP_PORT (default 587), SMTP_USERNAME,
// SMTP_PASSWORD, SMTP_FROM and the optional SMTP_TLS ("starttls" or "tls";
// default by port). It returns nil, nil when SMTP_HOST is unset, and an error
// when it is set but the rest is unusable, so a half-configured production
// never silently falls back to sending nothing. The error never echoes the
// password.
func FromEnv() (*Sender, error) {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	if host == "" {
		return nil, nil
	}
	s, err := New(Config{
		Host:     host,
		Port:     os.Getenv("SMTP_PORT"),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     os.Getenv("SMTP_FROM"),
		TLS:      os.Getenv("SMTP_TLS"),
	})
	if err != nil {
		return nil, fmt.Errorf("SMTP_HOST is set but the SMTP configuration is unusable: %w", err)
	}
	return s, nil
}
