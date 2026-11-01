package grpc

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ConnLimits bounds what one WebSocket connection, one address and one user may
// cost the hub. The hub shares its process with messaging's REST and gRPC
// servers, so a socket that is never limited is a way to starve all of them.
type ConnLimits struct {
	// MaxFrameBytes is the largest client frame read; a larger one closes the
	// connection with 1009. Client frames are small: the largest is the auth
	// frame, a JWT of about a kilobyte.
	MaxFrameBytes int64
	// PongWait is how long the hub waits for any frame, a pong included, before
	// it gives the connection up. Pings go out at nine tenths of it.
	PongWait time.Duration
	// PerIP caps the connections open from one client address; PreAuthPerIP caps
	// those of them that have not authenticated yet. Over a cap the upgrade is
	// answered 429.
	PerIP        int
	PreAuthPerIP int
	// FramesPerSecond and FrameBurst throttle the frames one user sends, across
	// all of their sessions on this instance (per connection before it has
	// authenticated). Frames over the budget wait; they are not dropped, so a
	// burst of subscribes after a reconnect is slowed, never lost.
	FramesPerSecond float64
	FrameBurst      int
}

// DefaultConnLimits are the limits the hub runs with unless SetConnLimits says
// otherwise. The per-address caps leave room for an office behind one NAT
// address, where every tab is a socket.
func DefaultConnLimits() ConnLimits {
	return ConnLimits{
		MaxFrameBytes:   64 << 10,
		PongWait:        60 * time.Second,
		PerIP:           256,
		PreAuthPerIP:    32,
		FramesPerSecond: 50,
		FrameBurst:      500,
	}
}

// writeWait bounds one write to the peer, a ping included.
const writeWait = 10 * time.Second

// SetConnLimits replaces the connection limits for connections opened from now
// on. Zero or negative fields keep their default.
func (h *Hub) SetConnLimits(l ConnLimits) {
	d := DefaultConnLimits()
	if l.MaxFrameBytes <= 0 {
		l.MaxFrameBytes = d.MaxFrameBytes
	}
	if l.PongWait <= 0 {
		l.PongWait = d.PongWait
	}
	if l.PerIP <= 0 {
		l.PerIP = d.PerIP
	}
	if l.PreAuthPerIP <= 0 {
		l.PreAuthPerIP = d.PreAuthPerIP
	}
	if l.FramesPerSecond <= 0 {
		l.FramesPerSecond = d.FramesPerSecond
	}
	if l.FrameBurst <= 0 {
		l.FrameBurst = d.FrameBurst
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.limits = l
}

// SetClientIP installs how the client address of an upgrade request is read.
// Behind a reverse proxy it must read the forwarded address from the trusted
// proxy only (httputil.NewIPExtractor); the default is the TCP peer.
func (h *Hub) SetClientIP(fn func(*http.Request) string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clientIP = fn
}

// directIP is the TCP peer's address without its port.
func directIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ipSlots counts the open and the not-yet-authenticated connections per client
// address.
type ipSlots struct {
	mu      sync.Mutex
	open    map[string]int
	preAuth map[string]int
}

func newIPSlots() *ipSlots {
	return &ipSlots{open: make(map[string]int), preAuth: make(map[string]int)}
}

// take reserves an open, unauthenticated slot for ip, or reports false when
// either cap is reached.
func (s *ipSlots) take(ip string, l ConnLimits) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.open[ip] >= l.PerIP || s.preAuth[ip] >= l.PreAuthPerIP {
		return false
	}
	s.open[ip]++
	s.preAuth[ip]++
	return true
}

// authenticated releases ip's unauthenticated slot.
func (s *ipSlots) authenticated(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	decrement(s.preAuth, ip)
}

// closed releases ip's open slot, and its unauthenticated one when the
// connection never authenticated.
func (s *ipSlots) closed(ip string, preAuth bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	decrement(s.open, ip)
	if preAuth {
		decrement(s.preAuth, ip)
	}
}

func decrement(m map[string]int, key string) {
	if m[key] <= 1 {
		delete(m, key)
		return
	}
	m[key]--
}

// userLimiter returns the frame budget shared by the user's sessions on this
// instance, creating it on first use. It is dropped with the user's last
// session (UnsubscribeAll). Callers hold h.mu.
func (h *Hub) userLimiterLocked(userID string) *rate.Limiter {
	if l := h.frameLimiters[userID]; l != nil {
		return l
	}
	l := rate.NewLimiter(rate.Limit(h.limits.FramesPerSecond), h.limits.FrameBurst)
	h.frameLimiters[userID] = l
	return l
}

// OriginPolicy decides which web pages may open a socket. A browser sends the
// page's origin with the upgrade; a page of another site must not get a socket
// in the user's name, even though today the socket proves who it is by a token
// rather than a cookie.
//
// Admitted: a request without Origin (not a browser), a page of the same host
// the socket is served on (the app behind nginx, or the Vite dev proxy, which
// keeps the Host header), the configured app origin, and — in development only —
// a page on a loopback host at any port (a dev server whose proxy rewrites Host).
type OriginPolicy struct {
	allowed  map[string]bool
	loopback bool
}

// NewOriginPolicy builds the policy from the app's public base URL (APP_BASE_URL,
// may be empty) and whether this is a development environment.
func NewOriginPolicy(appBaseURL string, dev bool) (OriginPolicy, error) {
	p := OriginPolicy{allowed: map[string]bool{}, loopback: dev}
	appBaseURL = strings.TrimSpace(appBaseURL)
	if appBaseURL == "" {
		return p, nil
	}
	origin, ok := normalizeOrigin(appBaseURL)
	if !ok {
		return OriginPolicy{}, fmt.Errorf("app base URL %q is not an absolute http(s) URL", appBaseURL)
	}
	p.allowed[origin] = true
	return p, nil
}

// SetOriginPolicy replaces the origin policy. Until it is called, only a request
// without Origin or from the socket's own host is admitted.
func (h *Hub) SetOriginPolicy(p OriginPolicy) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.origins = p
}

// checkOrigin is the upgrader's CheckOrigin.
func (h *Hub) checkOrigin(r *http.Request) bool {
	raw := r.Header.Get("Origin")
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	h.mu.RLock()
	p := h.origins
	h.mu.RUnlock()
	if origin, ok := normalizeOrigin(raw); ok && p.allowed[origin] {
		return true
	}
	if p.loopback {
		switch strings.ToLower(u.Hostname()) {
		case "localhost", "127.0.0.1", "::1":
			return true
		}
	}
	return false
}

// normalizeOrigin returns scheme://host[:port] of an absolute http(s) URL in
// lower case, the form a browser sends as Origin.
func normalizeOrigin(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	return strings.ToLower(u.Scheme + "://" + u.Host), true
}
