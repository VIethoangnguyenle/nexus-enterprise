package httputil

import (
	"fmt"
	"net"
	"strings"

	"github.com/labstack/echo/v4"
)

// NewIPExtractor says whose address a request comes from. Public per-address
// limits in every service use it, so they agree on who a client is.
//
// With no trusted proxies (the default) it is the connection's own address and
// X-Forwarded-For is ignored: a header any client can write must never choose
// which counter a request lands in. Behind a reverse proxy, list the proxy's
// networks (CIDR) and the client is read from X-Forwarded-For right to left,
// skipping only entries those networks added, so a forged leading entry does
// not count either.
func NewIPExtractor(trustedProxies []string) (echo.IPExtractor, error) {
	var ranges []echo.TrustOption
	for _, raw := range trustedProxies {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, n, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q is not a CIDR range: %w", raw, err)
		}
		ranges = append(ranges, echo.TrustIPRange(n))
	}
	if len(ranges) == 0 {
		return echo.ExtractIPDirect(), nil
	}
	// Nothing is trusted but what is listed: not loopback, not link-local, not
	// "any private address".
	opts := append([]echo.TrustOption{
		echo.TrustLoopback(false), echo.TrustLinkLocal(false), echo.TrustPrivateNet(false),
	}, ranges...)
	return echo.ExtractIPFromXFFHeader(opts...), nil
}
