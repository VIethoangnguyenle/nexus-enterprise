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
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
	// Buttons are actions shown on the notification (Chrome and Android: at most two;
	// other browsers show none, so the notification's own URL must still make sense).
	Buttons []Button
}

// Button is a notification action that opens an app path.
type Button struct {
	ID   string
	Text string
	URL  string
}

type webButton struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	URL  string `json:"url"`
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
	// appURL is the web app's origin; a notification's path opens under it.
	appURL  string
	http    *http.Client
	backoff time.Duration
}

func newOneSignal(appID, key, endpoint, appURL string) *OneSignal {
	return &OneSignal{appID: appID, key: key, endpoint: endpoint, appURL: strings.TrimRight(appURL, "/"),
		http: &http.Client{Timeout: requestTimeout}, backoff: 500 * time.Millisecond}
}

// FromEnv builds a sender from ONESIGNAL_APP_ID and ONESIGNAL_REST_API_KEY.
// Neither set: a sender that does nothing. Only one set: an error, so a
// half-configured deployment never silently sends nothing. The error never
// echoes the key. APP_BASE_URL is the web app the notifications open; web push
// needs an absolute address.
func FromEnv() (Sender, error) {
	appID := strings.TrimSpace(os.Getenv("ONESIGNAL_APP_ID"))
	key := strings.TrimSpace(os.Getenv("ONESIGNAL_REST_API_KEY"))
	switch {
	case appID == "" && key == "":
		return Nop{}, nil
	case appID == "" || key == "":
		return nil, errors.New("push: set both ONESIGNAL_APP_ID and ONESIGNAL_REST_API_KEY, or neither")
	}
	appURL := strings.TrimSpace(os.Getenv("APP_BASE_URL"))
	if appURL == "" {
		appURL = "http://localhost:5173"
	}
	if u, err := url.Parse(appURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("push: APP_BASE_URL must be an absolute http(s) URL, got %q", appURL)
	}
	return newOneSignal(appID, key, defaultEndpoint, appURL), nil
}

// newIdempotencyKey is a random UUID (version 4): OneSignal drops a second
// request with the same key, so a retry after a timeout does not deliver twice.
func newIdempotencyKey() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
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
	// WebPushTopic replaces an earlier web notification with the same topic;
	// CollapseID does the same on mobile.
	WebPushTopic   string      `json:"web_push_topic,omitempty"`
	CollapseID     string      `json:"collapse_id,omitempty"`
	WebButtons     []webButton `json:"web_buttons,omitempty"`
	IdempotencyKey string      `json:"idempotency_key"`
}

// absolute makes an app path an address under the web app; web push opens only absolute ones.
func (s *OneSignal) absolute(u string) string {
	if strings.HasPrefix(u, "/") {
		return s.appURL + u
	}
	return u
}

// Send delivers a notification, retrying a server error or a rate limit a
// bounded number of times; a refusal (4xx) is not retried.
func (s *OneSignal) Send(ctx context.Context, m Message) error {
	if len(m.ExternalIDs) == 0 {
		return nil
	}
	idem, err := newIdempotencyKey()
	if err != nil {
		return fmt.Errorf("push: idempotency key: %w", err)
	}
	var buttons []webButton
	for _, b := range m.Buttons {
		buttons = append(buttons, webButton{ID: b.ID, Text: b.Text, URL: s.absolute(b.URL)})
	}
	body, err := json.Marshal(request{
		AppID: s.appID, TargetChannel: "push",
		IncludeAliases: map[string][]string{"external_id": m.ExternalIDs},
		// "en" is the language OneSignal requires; the text is the same for all.
		Headings:       map[string]string{"en": m.Title, "vi": m.Title},
		Contents:       map[string]string{"en": m.Body, "vi": m.Body},
		WebURL:         s.absolute(m.URL),
		WebPushTopic:   m.CollapseID,
		CollapseID:     m.CollapseID,
		WebButtons:     buttons,
		IdempotencyKey: idem,
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
