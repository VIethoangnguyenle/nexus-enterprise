// Package push sends web push notifications through OneSignal.
//
// Devices are linked to users by OneSignal's external_id, which the frontend
// sets to users.id at sign-in; a message is addressed to users, never to
// devices. Without configuration the sender does nothing, so development and
// tests run without an account. Logs and errors never carry a message's text.
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// Message is one notification to some users.
type Message struct {
	// ExternalIDs are users.id values; OneSignal resolves their devices.
	ExternalIDs []string
	Title       string
	Body        string
	// URL is the app path the notification opens.
	URL string
	// CollapseID replaces an earlier notification with the same id on the device.
	CollapseID string
}

// Sender sends push notifications.
type Sender interface {
	Send(ctx context.Context, m Message) error
	// Enabled reports whether anything is actually sent.
	Enabled() bool
}

// Nop sends nothing.
type Nop struct{}

// Send does nothing.
func (Nop) Send(context.Context, Message) error { return nil }

// Enabled is false.
func (Nop) Enabled() bool { return false }

const (
	defaultEndpoint = "https://api.onesignal.com/notifications"
	maxAttempts     = 3
	requestTimeout  = 10 * time.Second
)

// OneSignal sends through the OneSignal REST API.
type OneSignal struct {
	appID    string
	key      string
	endpoint string
	http     *http.Client
	backoff  time.Duration
}

func newOneSignal(appID, key, endpoint string) *OneSignal {
	return &OneSignal{appID: appID, key: key, endpoint: endpoint,
		http: &http.Client{Timeout: requestTimeout}, backoff: 500 * time.Millisecond}
}

// FromEnv builds a sender from ONESIGNAL_APP_ID and ONESIGNAL_REST_API_KEY.
// Neither set: a sender that does nothing. Only one set: an error, so a
// half-configured deployment never silently sends nothing. The error never
// echoes the key.
func FromEnv() (Sender, error) {
	appID := strings.TrimSpace(os.Getenv("ONESIGNAL_APP_ID"))
	key := strings.TrimSpace(os.Getenv("ONESIGNAL_REST_API_KEY"))
	switch {
	case appID == "" && key == "":
		return Nop{}, nil
	case appID == "" || key == "":
		return nil, errors.New("push: set both ONESIGNAL_APP_ID and ONESIGNAL_REST_API_KEY, or neither")
	}
	return newOneSignal(appID, key, defaultEndpoint), nil
}

// Enabled is true.
func (s *OneSignal) Enabled() bool { return true }

type request struct {
	AppID          string              `json:"app_id"`
	TargetChannel  string              `json:"target_channel"`
	IncludeAliases map[string][]string `json:"include_aliases"`
	Headings       map[string]string   `json:"headings"`
	Contents       map[string]string   `json:"contents"`
	WebURL         string              `json:"web_url,omitempty"`
	CollapseID     string              `json:"collapse_id,omitempty"`
}

// Send delivers a notification, retrying a server error or a rate limit a
// bounded number of times; a refusal (4xx) is not retried.
func (s *OneSignal) Send(ctx context.Context, m Message) error {
	if len(m.ExternalIDs) == 0 {
		return nil
	}
	body, err := json.Marshal(request{
		AppID: s.appID, TargetChannel: "push",
		IncludeAliases: map[string][]string{"external_id": m.ExternalIDs},
		// "en" is the language OneSignal requires; the text is the same for all.
		Headings:   map[string]string{"en": m.Title, "vi": m.Title},
		Contents:   map[string]string{"en": m.Body, "vi": m.Body},
		WebURL:     m.URL,
		CollapseID: m.CollapseID,
	})
	if err != nil {
		return fmt.Errorf("push: encode: %w", err)
	}
	var last error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		status, err := s.post(ctx, body)
		switch {
		case err == nil && status < 300:
			return nil
		case err == nil && status < 500 && status != http.StatusTooManyRequests:
			return fmt.Errorf("push: refused with status %d", status)
		case err != nil:
			last = fmt.Errorf("push: request failed: %w", err)
		default:
			last = fmt.Errorf("push: status %d", status)
		}
		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.backoff * time.Duration(attempt)):
			}
		}
	}
	return last
}

func (s *OneSignal) post(ctx context.Context, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Key "+s.key)
	resp, err := s.http.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}
