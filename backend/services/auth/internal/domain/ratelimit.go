package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// PublicLimit is how many requests one address may make to one public route
// per window.
type PublicLimit struct {
	Max    int
	Window time.Duration
}

// DefaultPublicLimit is generous on purpose: an office behind one NAT address
// signs in at nine, and the page load of each person refreshes a session. What
// it stops is a script, not a crowd; the per-identifier and per-session limits
// do the fine work.
var DefaultPublicLimit = PublicLimit{Max: 60, Window: time.Minute}

// TakePublicRequest counts one request from ip to the named public route and
// returns ErrRateLimited (with the time left in the window) when the address has
// used up its allowance. Without Redis there is nothing to count against and
// nothing is limited. The address is stored hashed: the counter needs to tell
// addresses apart, not to remember them.
func (s *Service) TakePublicRequest(ctx context.Context, route, ip string, lim PublicLimit) error {
	if s == nil || s.rdb == nil || lim.Max <= 0 {
		return nil
	}
	if lim.Window <= 0 {
		lim.Window = DefaultPublicLimit.Window
	}
	sum := sha256.Sum256([]byte(ip))
	key := fmt.Sprintf("pub_rl:%s:%s", route, hex.EncodeToString(sum[:8]))
	n, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("public rate limit: %w", err)
	}
	if n == 1 {
		s.rdb.Expire(ctx, key, lim.Window)
	}
	if n <= int64(lim.Max) {
		return nil
	}
	return &rateLimitedError{base: ErrRateLimited, after: s.windowLeft(ctx, key, lim.Window)}
}
