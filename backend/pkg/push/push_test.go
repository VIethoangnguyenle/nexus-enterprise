package push

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOneSignal_SendsOneRequestAddressedByExternalID(t *testing.T) {
	var got map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("body is not JSON: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := newOneSignal("app-1", "key-1", srv.URL, "https://nexus.example").Send(context.Background(), Message{
		ExternalIDs: []string{"user-1"}, Title: "Nhóm Kế toán", Body: "Lan: đã gửi báo cáo",
		URL: "/channels/c1?thread=m1", CollapseID: "channel-c1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if auth != "Key key-1" {
		t.Errorf("authorization header %q", auth)
	}
	want := map[string]any{
		"app_id":          "app-1",
		"target_channel":  "push",
		"include_aliases": map[string]any{"external_id": []any{"user-1"}},
		// Web push opens an absolute address and replaces by topic.
		"web_url":        "https://nexus.example/channels/c1?thread=m1",
		"web_push_topic": "channel-c1",
		"collapse_id":    "channel-c1",
	}
	for k, v := range want {
		if !reflect.DeepEqual(got[k], v) {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
	if got["headings"].(map[string]any)["en"] != "Nhóm Kế toán" || got["contents"].(map[string]any)["en"] != "Lan: đã gửi báo cáo" {
		t.Errorf("headings/contents = %v / %v", got["headings"], got["contents"])
	}
}

func TestOneSignal_RetriesAServerErrorThenGivesUp(t *testing.T) {
	var calls atomic.Int32
	var mu sync.Mutex
	keys := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var got map[string]any
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		mu.Lock()
		keys[fmt.Sprint(got["idempotency_key"])] = true
		mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	s := newOneSignal("app", "key", srv.URL, "https://nexus.example")
	s.backoff = time.Millisecond

	err := s.Send(context.Background(), Message{ExternalIDs: []string{"u"}, Title: "t", Body: "secret body"})
	if err == nil {
		t.Fatal("want an error")
	}
	if calls.Load() != maxAttempts {
		t.Errorf("%d attempts, want %d", calls.Load(), maxAttempts)
	}
	if strings.Contains(err.Error(), "secret body") {
		t.Error("an error never carries the message body")
	}
	// A retry after a timeout must not deliver twice: every attempt carries the
	// same idempotency key, a UUID as OneSignal requires.
	if len(keys) != 1 {
		t.Fatalf("attempts carried %d idempotency keys, want 1: %v", len(keys), keys)
	}
	for k := range keys {
		if !uuidPattern.MatchString(k) {
			t.Errorf("idempotency key %q is not a UUID", k)
		}
	}
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestFromEnv_NeedsAnAbsoluteAppAddress(t *testing.T) {
	t.Setenv("ONESIGNAL_APP_ID", "app")
	t.Setenv("ONESIGNAL_REST_API_KEY", "key")
	t.Setenv("APP_BASE_URL", "nexus.example")
	if _, err := FromEnv(); err == nil {
		t.Fatal("a relative APP_BASE_URL would make every notification link unusable")
	}
}

func TestOneSignal_DoesNotRetryARefusal(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	if err := newOneSignal("app", "key", srv.URL, "https://nexus.example").Send(context.Background(), Message{ExternalIDs: []string{"u"}}); err == nil {
		t.Fatal("want an error")
	}
	if calls.Load() != 1 {
		t.Errorf("%d attempts, want 1", calls.Load())
	}
}

func TestFromEnv_WithoutConfigurationSendsNothing(t *testing.T) {
	t.Setenv("ONESIGNAL_APP_ID", "")
	t.Setenv("ONESIGNAL_REST_API_KEY", "")
	s, err := FromEnv()
	if err != nil || s.Enabled() {
		t.Fatalf("no configuration: want a disabled sender, got %v %v", s, err)
	}
	if err := s.Send(context.Background(), Message{ExternalIDs: []string{"u"}}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ONESIGNAL_APP_ID", "app")
	if _, err := FromEnv(); err == nil {
		t.Fatal("half a configuration is an error, not a silent no-op")
	}

	t.Setenv("ONESIGNAL_REST_API_KEY", "key")
	s, err = FromEnv()
	if err != nil || !s.Enabled() {
		t.Fatalf("full configuration: want an enabled sender, got %v", err)
	}
}

func TestSend_NobodyToTellIsANoOp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request for no recipient")
	}))
	defer srv.Close()
	if err := newOneSignal("app", "key", srv.URL, "https://nexus.example").Send(context.Background(), Message{}); err != nil {
		t.Fatal(err)
	}
}
