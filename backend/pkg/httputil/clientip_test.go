package httputil_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

func ipOf(t *testing.T, extract echo.IPExtractor, remote string, xff ...string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remote
	for _, v := range xff {
		req.Header.Add(echo.HeaderXForwardedFor, v)
	}
	return extract(req)
}

func TestIPExtractor_TrustsNothingByDefault(t *testing.T) {
	extract, err := httputil.NewIPExtractor(nil)
	if err != nil {
		t.Fatal(err)
	}
	// A client that writes its own X-Forwarded-For is still who it is.
	if got := ipOf(t, extract, "203.0.113.9:4000", "198.51.100.1"); got != "203.0.113.9" {
		t.Errorf("ip = %q, want the connection's own address", got)
	}
	// Even from a private address: with nothing configured, no proxy is known.
	if got := ipOf(t, extract, "10.0.0.5:4000", "198.51.100.1"); got != "10.0.0.5" {
		t.Errorf("ip = %q, want the connection's own address", got)
	}
}

func TestIPExtractor_HonoursForwardedForOnlyFromAConfiguredProxy(t *testing.T) {
	extract, err := httputil.NewIPExtractor([]string{"10.0.0.0/8", "172.16.0.0/12"})
	if err != nil {
		t.Fatal(err)
	}
	// Behind Traefik: the address it saw is the client.
	if got := ipOf(t, extract, "172.18.0.4:5000", "198.51.100.7"); got != "198.51.100.7" {
		t.Errorf("ip = %q, want the client Traefik saw", got)
	}
	// The client wrote a header of its own before Traefik appended the real one:
	// only the right-most entries a trusted proxy added count.
	if got := ipOf(t, extract, "172.18.0.4:5000", "1.2.3.4, 198.51.100.7"); got != "198.51.100.7" {
		t.Errorf("ip = %q, a forged leading entry must not win", got)
	}
	// Not from a proxy: the header is just text a client typed.
	if got := ipOf(t, extract, "203.0.113.9:4000", "1.2.3.4"); got != "203.0.113.9" {
		t.Errorf("ip = %q, want the connection's address when it is not a proxy", got)
	}
	if got := ipOf(t, extract, "172.18.0.4:5000", "not-an-ip"); got != "172.18.0.4" {
		t.Errorf("ip = %q, an unparseable header is not trusted", got)
	}
}

func TestIPExtractor_RefusesABadRange(t *testing.T) {
	if _, err := httputil.NewIPExtractor([]string{"10.0.0.0/8", "not-a-cidr"}); err == nil {
		t.Fatal("a typo in the trusted ranges must stop start-up, not silently trust nothing")
	}
}

